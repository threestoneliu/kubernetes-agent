package agent

import (
	"context"
	"errors"
	"sync"
)

// Session holds the per-conversation state for one chat session.
//
// Two unbuffered channels coordinate the blocking points in the agent
// loop: when a plan is ready, the loop emits PlanAwaitingConfirm and
// then blocks on ResumePlan; the HTTP layer calls ConfirmPlan() or
// CancelPlan() to unblock. AskUser follows the same pattern with
// ResumeAsk + AnswerAsk.
//
// Channels are unbuffered because we model the wakeup as a
// "release" — the agent loop blocks on `<-ResumePlan`, and the
// method that closes the channel (ConfirmPlan / CancelPlan /
// AnswerAsk) is the producer. Closing a channel broadcasts to all
// waiters, which matches the "one plan, one decision" semantic
// the agent loop wants.
type Session struct {
	ID        string
	ClusterID string

	// ResumePlan is closed by ConfirmPlan (user accepts) or
	// CancelPlan (user rejects). Closing it ends the wait.
	ResumePlan chan struct{}

	// ResumeAsk is closed by AnswerAsk (user provided an answer).
	// The answer is read from AskAnswer after the channel closes.
	ResumeAsk chan struct{}

	// AskAnswer is set by AnswerAsk under mu before ResumeAsk is
	// closed. The agent loop reads it after WaitAsk returns.
	AskAnswer string

	// PlanResult records the user's decision on the most recent
	// plan: "confirmed", "cancelled", or "" if still pending.
	// The agent loop reads this after WaitPlan returns.
	PlanResult string

	// LastEventID is the most recent SSE event id emitted. The
	// HTTP layer uses it to support EventSource reconnect with
	// `Last-Event-ID`. Not used by the agent loop itself.
	LastEventID string

	mu sync.Mutex
}

// NewSession constructs an empty Session with the two resume channels
// initialised. Channel allocation happens up front so WaitPlan /
// WaitAsk can be called from the agent loop without a race.
func NewSession(id string) *Session {
	return &Session{
		ID:         id,
		ResumePlan: make(chan struct{}),
		ResumeAsk:  make(chan struct{}),
	}
}

// ConfirmPlan signals that the user has approved the pending plan.
// Calling ConfirmPlan twice panics (channel close of closed channel)
// — the agent loop is expected to start a new turn before the next
// plan is awaited.
func (s *Session) ConfirmPlan() {
	s.mu.Lock()
	s.PlanResult = "confirmed"
	s.mu.Unlock()
	close(s.ResumePlan)
}

// CancelPlan signals that the user has rejected the pending plan.
// Idempotency: same as ConfirmPlan.
func (s *Session) CancelPlan() {
	s.mu.Lock()
	s.PlanResult = "cancelled"
	s.mu.Unlock()
	close(s.ResumePlan)
}

// ResetPlan resets the session so a new plan can be awaited.
// Creates fresh ResumePlan channel and clears PlanResult.
// Called by k8s_plan_write handler before WaitPlan to handle the
// case where a previous plan was cancelled.
func (s *Session) ResetPlan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// If channel is already closed, create a new one
	s.ResumePlan = make(chan struct{})
	s.PlanResult = ""
}

// AnswerAsk stores the user's answer and unblocks the agent loop.
// The answer is set under the mutex before ResumeAsk is closed, so
// the agent loop can safely read AskAnswer after WaitAsk returns
// without an extra synchronisation step.
func (s *Session) AnswerAsk(answer string) {
	s.mu.Lock()
	s.AskAnswer = answer
	s.mu.Unlock()
	close(s.ResumeAsk)
}

// WaitPlan blocks until the user confirms or cancels the plan, or
// the context is cancelled. Returns ctx.Err() on cancellation so
// the runner can distinguish "user cancelled plan" from
// "user cancelled context".
func (s *Session) WaitPlan(ctx context.Context) error {
	select {
	case <-s.ResumePlan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WaitAsk blocks until AnswerAsk is called or the context is
// cancelled. Same error semantics as WaitPlan.
func (s *Session) WaitAsk(ctx context.Context) error {
	select {
	case <-s.ResumeAsk:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SessionManager is a thread-safe registry of sessions keyed by id.
// The HTTP layer looks up (or creates) a session per request; the
// agent loop holds a *Session reference directly. We keep the
// manager simple: a map + a mutex. For the MVP, every session lives
// in memory; a future change can swap the map for a persistence
// layer.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// NewSessionManager returns an empty manager. The zero value is
// usable too, but NewSessionManager is friendlier.
func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: map[string]*Session{}}
}

// Get returns the session with the given id, creating an empty one
// if none exists. The returned pointer is stable for the life of
// the manager.
func (m *SessionManager) Get(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		return s
	}
	s := NewSession(id)
	m.sessions[id] = s
	return s
}

// ErrSessionNotFound is returned by Lookup when the session is not
// in the manager. Get never returns this — it lazily creates.
var ErrSessionNotFound = errors.New("session not found")

// Lookup returns an existing session without creating. Used by the
// HTTP layer to validate that a client-supplied session id is
// already known.
func (m *SessionManager) Lookup(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		return s, nil
	}
	return nil, ErrSessionNotFound
}

// Drop removes a session from the manager. Use for tests; production
// keeps sessions for the process lifetime.
func (m *SessionManager) Drop(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

// Set replaces (or installs) the session for id with the supplied
// pointer. Used by the chatHandler at the start of each turn so the
// manager points at the same Session the runner is using — the
// runner's channels (ResumePlan / ResumeAsk) are the ones the resume
// endpoint must close to unblock the agent loop. A previous turn's
// session is silently overwritten; the HTTP layer is expected to
// have finished draining its SSE stream before the next turn starts.
func (m *SessionManager) Set(id string, s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = s
}

package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession_ConfirmPlanUnblocks(t *testing.T) {
	s := NewSession("s1")
	go func() {
		time.Sleep(5 * time.Millisecond)
		s.ConfirmPlan()
	}()
	require.NoError(t, s.WaitPlan(context.Background()))
	assert.Equal(t, "confirmed", s.PlanResult)
}

func TestSession_CancelPlanUnblocks(t *testing.T) {
	s := NewSession("s1")
	go func() {
		time.Sleep(5 * time.Millisecond)
		s.CancelPlan()
	}()
	require.NoError(t, s.WaitPlan(context.Background()))
	assert.Equal(t, "cancelled", s.PlanResult)
}

func TestSession_WaitPlanCtxCancel(t *testing.T) {
	s := NewSession("s1")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()
	assert.ErrorIs(t, s.WaitPlan(ctx), context.Canceled)
}

func TestSession_AnswerAskUnblocks(t *testing.T) {
	s := NewSession("s1")
	go func() {
		time.Sleep(5 * time.Millisecond)
		s.AnswerAsk("ns-prod")
	}()
	require.NoError(t, s.WaitAsk(context.Background()))
	assert.Equal(t, "ns-prod", s.AskAnswer)
}

func TestSessionManager_GetCreatesOnce(t *testing.T) {
	m := NewSessionManager()
	a := m.Get("s1")
	b := m.Get("s1")
	c := m.Get("s2")
	assert.Same(t, a, b)
	assert.NotSame(t, a, c)
}

func TestSessionManager_LookupMissing(t *testing.T) {
	m := NewSessionManager()
	_, err := m.Lookup("nope")
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionManager_Drop(t *testing.T) {
	m := NewSessionManager()
	m.Get("s1")
	m.Drop("s1")
	_, err := m.Lookup("s1")
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

// TestSessionManager_SetReplaces verifies that Set installs the
// caller-supplied Session, overwriting any prior entry under the
// same id. The chatHandler relies on this so the manager points at
// the same Session the runner is using.
func TestSessionManager_SetReplaces(t *testing.T) {
	m := NewSessionManager()
	original := m.Get("s1")
	replacement := NewSession("s1")
	m.Set("s1", replacement)
	got, err := m.Lookup("s1")
	require.NoError(t, err)
	assert.Same(t, replacement, got)
	assert.NotSame(t, original, got)
}

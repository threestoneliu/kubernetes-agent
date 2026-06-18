package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/threestoneliu/kubernetes-agent/internal/agent"
	"github.com/threestoneliu/kubernetes-agent/internal/llm"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

type chatReq struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	ClusterID string `json:"cluster_id"`
}

// chatHandler is the SSE entry point. The handler:
//
//  1. Decodes + validates the request body (400 on empty message).
//  2. Resolves the session: empty session_id => new session; known
//     id => load it (404 if missing).
//  3. Opens an SSE stream, hands it to the agent runner, and
//     forwards every event as one SSE frame (`id:` / `event:` /
//     `data:`), flushing after each.
//  4. Respects ctx.Done() — when the client disconnects, the
//     runner's context is cancelled and the goroutine tears down.
//
// Last-Event-ID: the spec asks us to support EventSource reconnect
// by skipping events the client already has. For the MVP we just
// acknowledge the header in a comment and stream the entire new
// turn — the agent is designed to be invoked once per turn, and
// historical events are always available via
// /api/sessions/{id}/messages.
func chatHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req chatReq
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Message == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "message is required", false)
			return
		}

		// Resolve session: empty => create; otherwise => fetch.
		// This must happen before we set SSE headers so a 4xx
		// can still be rendered as JSON rather than an SSE frame.
		var sess store.Session
		var resolvedID string
		if req.SessionID == "" {
			// Default cluster_id may be omitted; the user can
			// switch later. We capture it on the row for the
			// sidebar's "active cluster" hint.
			var clusterPtr *string
			if req.ClusterID != "" {
				c := req.ClusterID
				clusterPtr = &c
			}
			sess = store.Session{
				ID:        uuid.NewString(),
				Title:     defaultSessionTitle(req.Message),
				ClusterID: clusterPtr,
			}
			if err := d.DB.CreateSession(r.Context(), sess); err != nil {
				writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
				return
			}
			resolvedID = sess.ID
		} else {
			row, err := d.DB.GetSession(r.Context(), req.SessionID)
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "session not found", false)
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
				return
			}
			sess = row
			resolvedID = row.ID
		}

		// SSE headers must be set before the first Write.
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "internal", "streaming unsupported", true)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// Disable nginx response buffering when running behind it.
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		// Emit a session_meta event up-front so the frontend has
		// the resolved id (in case it was empty in the request).
		emitSSE(w, flusher, 1, agent.EventSessionMeta, agent.SessionMeta{
			SessionID: resolvedID,
			ClusterID: req.ClusterID,
		})

		// Build the runner. If the factory is missing (shouldn't
		// happen in production) we surface a streaming error and
		// close the connection.
		if d.RunnerFactory == nil {
			emitSSE(w, flusher, 2, agent.EventError, agent.ErrorPayload{
				Code:      "internal",
				Message:   "agent runner not configured",
				Retryable: false,
			})
			return
		}
		runner := d.RunnerFactory.NewRunner(resolvedID, req.ClusterID)
		events := make(chan agent.Event, 64)
		var counter uint64
		runner.Events = events
		// Session must be set on the runner before Run is called.
		runner.Session = agent.NewSession(resolvedID)
		if req.ClusterID != "" {
			runner.Session.ClusterID = req.ClusterID
			// Surface the cluster UUID to the LLM via the system
			// prompt so it can pass cluster_id back to the k8s
			// tools. Without this the LLM guesses "default" or
			// asks the user. Append-only — does not replace the
			// default prompt.
			runner.SystemPrompt = llm.SystemPrompt +
				fmt.Sprintf("\n\n当前 session 绑定的 cluster_id: %s。所有 k8s_* 工具调用 MUST 使用此 UUID 作为 cluster_id 参数。", req.ClusterID)
		}
		// Register the live session so the /resume endpoint can
		// unblock a pending plan confirm or ask_user response.
		// Without this, the UI's PlanModal cannot release the agent
		// loop. Use Set (not Get) so the manager points at the
		// runner's own Session — otherwise the resume endpoint
		// would close channels that nobody is waiting on.
		if d.Sessions != nil {
			d.Sessions.Set(resolvedID, runner.Session)
			defer d.Sessions.Drop(resolvedID)
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = runner.Run(r.Context(), req.Message)
		}()

		// Forward events until the runner returns OR the client
		// disconnects. ctx.Done() unblocks the runner via the
		// cancellation we passed in.
		for {
			select {
			case ev, ok := <-events:
				if !ok {
					return
				}
				id := atomic.AddUint64(&counter, 1)
				if !emitSSE(w, flusher, id+1, ev.Type, json.RawMessage(ev.Payload)) {
					return
				}
			case <-r.Context().Done():
				// Client disconnected; wait briefly for the
				// runner goroutine to unwind then return.
				<-done
				return
			case <-done:
				return
			}
		}
	}
}

// emitSSE writes one SSE frame and flushes. The `data` payload is
// embedded verbatim — the agent already marshals its payloads, so
// we do not re-marshal here. Returns false if the write failed
// (caller should stop streaming).
func emitSSE(w http.ResponseWriter, flusher http.Flusher, id uint64, event string, data any) bool {
	var raw []byte
	switch v := data.(type) {
	case json.RawMessage:
		raw = v
	case []byte:
		raw = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return false
		}
		raw = b
	}
	if _, err := fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n",
		strconv.FormatUint(id, 10), event, raw); err != nil {
		return false
	}
	flusher.Flush()
	return true
}

// defaultSessionTitle turns the first ~30 chars of the user
// message into a sidebar label. Fall back to "new chat" if the
// message is whitespace-only.
func defaultSessionTitle(msg string) string {
	const max = 30
	s := msg
	if len(s) > max {
		s = s[:max] + "…"
	}
	if s == "" {
		return "new chat"
	}
	return s
}

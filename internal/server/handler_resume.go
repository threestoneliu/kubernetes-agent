package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/threestoneliu/kubernetes-agent/internal/agent"
)

// resumeReq is the wire shape for POST /api/sessions/{id}/resume.
// The frontend uses this single endpoint for both:
//   - plan confirm / cancel (kind="plan", approved=true|false)
//   - ask_user answer (kind="ask_user", answer="...")
// A discriminated union on the `kind` field lets one route handle
// the two resume paths the agent loop can block on.
type resumeReq struct {
	Kind     string `json:"kind"`
	PlanID   string `json:"plan_id,omitempty"`
	Approved *bool  `json:"approved,omitempty"`
	Answer   string `json:"answer,omitempty"`
}

// resumeHandler unblocks a blocked agent session. The agent loop
// calls WaitPlan / WaitAsk on the per-session channels; the UI
// surfaces the corresponding modal and posts back here with the
// user's decision.
//
// The handler looks up the active agent.Session via the
// SessionManager that the chatHandler populates on each new turn.
// If the session id is unknown (no in-flight turn) the handler
// returns 404 — the UI should treat this as a "your plan expired"
// case and start a new turn.
func resumeHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Sessions == nil {
			writeError(w, http.StatusInternalServerError, "internal", "session manager not configured", true)
			return
		}
		id := chi.URLParam(r, "id")
		var req resumeReq
		if !decodeJSON(w, r, &req) {
			return
		}
		sess, err := d.Sessions.Lookup(id)
		if err != nil {
			if errors.Is(err, agent.ErrSessionNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "no active session", false)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		switch req.Kind {
		case "plan":
			if req.PlanID == "" {
				writeError(w, http.StatusBadRequest, "validation_error", "plan_id is required", false)
				return
			}
			if req.Approved == nil {
				writeError(w, http.StatusBadRequest, "validation_error", "approved is required", false)
				return
			}
			// Idempotency guard: if the channel was already
			// closed (user double-clicked confirm), ReportStateOK
			// rather than panicking on the second close.
			defer func() {
				if rec := recover(); rec != nil {
					writeError(w, http.StatusConflict, "already_resolved", "plan already resolved", false)
				}
			}()
			if *req.Approved {
				sess.ConfirmPlan()
			} else {
				sess.CancelPlan()
			}
		case "ask_user":
			if req.Answer == "" {
				writeError(w, http.StatusBadRequest, "validation_error", "answer is required", false)
				return
			}
			defer func() {
				if rec := recover(); rec != nil {
					writeError(w, http.StatusConflict, "already_resolved", "ask_user already resolved", false)
				}
			}()
			sess.AnswerAsk(req.Answer)
		default:
			writeError(w, http.StatusBadRequest, "validation_error", "kind must be 'plan' or 'ask_user'", false)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
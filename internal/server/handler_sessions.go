package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

// sessionListLimit caps the number of sessions returned in the
// default list view. The UI only needs the most recent dozen or
// so for its sidebar; an explicit "show more" can be wired in
// later via a query param.
const sessionListLimit = 100

// sessionView is the public projection of a stored session.
type sessionView struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	ClusterID *string `json:"cluster_id,omitempty"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
}

func toSessionView(s store.Session) sessionView {
	return sessionView{
		ID:        s.ID,
		Title:     s.Title,
		ClusterID: s.ClusterID,
		CreatedAt: s.CreatedAt.Unix(),
		UpdatedAt: s.UpdatedAt.Unix(),
	}
}

func listSessionsHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := d.DB.ListSessions(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		// Defensive cap: ListSessions already orders by created_at
		// DESC, so the first N are the most recent.
		if len(rows) > sessionListLimit {
			rows = rows[:sessionListLimit]
		}
		out := make([]sessionView, 0, len(rows))
		for _, s := range rows {
			out = append(out, toSessionView(s))
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
	}
}

type createSessionReq struct {
	Title     string  `json:"title"`
	ClusterID *string `json:"cluster_id,omitempty"`
}

func createSessionHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createSessionReq
		if !decodeJSON(w, r, &req) {
			return
		}
		row := store.Session{
			ID:        uuid.NewString(),
			Title:     req.Title,
			ClusterID: req.ClusterID,
		}
		if err := d.DB.CreateSession(r.Context(), row); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		writeJSON(w, http.StatusCreated, toSessionView(row))
	}
}

func getSessionHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		row, err := d.DB.GetSession(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "session not found", false)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		writeJSON(w, http.StatusOK, toSessionView(row))
	}
}

// messageView is the wire shape for a single chat history message.
// Optional fields are omitempty so the list view stays compact for
// simple text messages.
type messageView struct {
	ID         string  `json:"id"`
	Role       string  `json:"role"`
	Content    *string `json:"content,omitempty"`
	ToolCalls  *string `json:"tool_calls,omitempty"`
	ToolCallID *string `json:"tool_call_id,omitempty"`
	Reasoning  *string `json:"reasoning,omitempty"`
	CreatedAt  int64   `json:"created_at"`
}

func toMessageView(m store.Message) messageView {
	return messageView{
		ID:         m.ID,
		Role:       m.Role,
		Content:    m.Content,
		ToolCalls:  m.ToolCalls,
		ToolCallID: m.ToolCallID,
		Reasoning:  m.Reasoning,
		CreatedAt:  m.CreatedAt,
	}
}

func listMessagesHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if _, err := d.DB.GetSession(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "session not found", false)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		rows, err := d.DB.ListMessagesBySession(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		out := make([]messageView, 0, len(rows))
		for _, m := range rows {
			out = append(out, toMessageView(m))
		}
		writeJSON(w, http.StatusOK, map[string]any{"messages": out})
	}
}

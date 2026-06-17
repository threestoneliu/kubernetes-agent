package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
		q := r.URL.Query().Get("q")
		sort := r.URL.Query().Get("sort")
		if sort == "" {
			sort = "updated_at"
		}
		order := r.URL.Query().Get("order")
		if order == "" {
			order = "desc"
		}
		limit := sessionListLimit
		if s := r.URL.Query().Get("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n <= 0 || n > sessionListLimit {
				writeError(w, http.StatusBadRequest, "invalid_limit",
					fmt.Sprintf("limit must be 1..%d", sessionListLimit), false)
				return
			}
			limit = n
		}
		offset := 0
		if s := r.URL.Query().Get("offset"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 {
				writeError(w, http.StatusBadRequest, "invalid_offset",
					"offset must be a non-negative integer", false)
				return
			}
			offset = n
		}
		rows, err := d.DB.ListSessionsFiltered(r.Context(), q, sort, order, limit, offset)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_sort", err.Error(), false)
			return
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

// putSessionHandler renames a session. The active-session manager
// does not gate rename: only stop/keep semantics matter for a
// streaming turn, and renaming is a metadata-only operation that
// does not affect the in-flight agent.
func putSessionHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Title string `json:"title"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.Title) == "" {
			writeError(w, http.StatusUnprocessableEntity, "invalid_title",
				"title must not be empty", false)
			return
		}
		if err := d.DB.UpdateSessionTitle(r.Context(), id, req.Title); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found",
					"session not found", false)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal",
				err.Error(), true)
			return
		}
		row, err := d.DB.GetSession(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				err.Error(), true)
			return
		}
		writeJSON(w, http.StatusOK, toSessionView(row))
	}
}

// deleteSessionHandler removes one session. Refuses if the session
// is currently in-flight in the agent (status 409 + session_active)
// so the user cannot interrupt a live turn by accident.
func deleteSessionHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if d.Sessions != nil {
			if _, err := d.Sessions.Lookup(id); err == nil {
				writeError(w, http.StatusConflict, "session_active",
					"session is in progress; stop the turn first", false)
				return
			}
		}
		n, err := d.DB.DeleteSession(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found",
					"session not found", false)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal",
				err.Error(), true)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": n})
	}
}

// bulkDeleteSessionsHandler empties the sessions table. Caller is
// expected to confirm in the UI before issuing this request.
func bulkDeleteSessionsHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := d.DB.DeleteAllSessions(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				err.Error(), true)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": n})
	}
}

// exportSessionHandler streams the requested session as Markdown or
// JSON for browser download. Markdown renders the chat history
// with reasoning collapsed and tool rows as fenced JSON blocks;
// JSON is a verbatim dump of the session + messages + plans +
// audit tables (intended as backup, not round-trip import).
func exportSessionHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		format := r.URL.Query().Get("format")
		if format != "md" && format != "json" {
			writeError(w, http.StatusBadRequest, "invalid_format",
				"format must be md or json", false)
			return
		}
		session, err := d.DB.GetSession(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found",
				"session not found", false)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				err.Error(), true)
			return
		}
		msgs, err := d.DB.ListMessagesBySession(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				err.Error(), true)
			return
		}
		audit, _ := d.DB.ListAudit(r.Context(), store.AuditFilter{SessionID: id})
		filename := fmt.Sprintf("session-%s.%s", id[:min(8, len(id))], format)
		w.Header().Set("Content-Disposition",
			`attachment; filename="`+filename+`"`)
		if format == "md" {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			w.Write([]byte(renderSessionMarkdown(session, msgs)))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		payload := map[string]any{
			"session":  session,
			"messages": msgs,
			"audit":    audit,
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(payload)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

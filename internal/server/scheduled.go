package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

// --- Request/Response types ---

type createScheduledTaskRequest struct {
	Name      string  `json:"name"`
	CronExpr  *string `json:"cron_expr,omitempty"`
	OnceAt    *int64  `json:"once_at,omitempty"`
	SessionID string  `json:"session_id"`
	ClusterID *string `json:"cluster_id,omitempty"`
	CreatedBy string  `json:"created_by"`
}

type updateScheduledTaskRequest struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Name    *string `json:"name,omitempty"`
	CronExpr *string `json:"cron_expr,omitempty"`
}

// --- Handlers --

func listScheduledTasksHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Query().Get("session_id")
		tasks, err := d.DB.GetScheduledTasks(r.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), false)
			return
		}
		out := make([]scheduledTaskResponse, 0, len(tasks))
		for _, t := range tasks {
			out = append(out, toResponse(t))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func createScheduledTaskHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createScheduledTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error(), false)
			return
		}
		if req.SessionID == "" || req.Name == "" {
			writeError(w, http.StatusBadRequest, "validation", "name and session_id are required", false)
			return
		}
		if req.CronExpr == nil && req.OnceAt == nil {
			writeError(w, http.StatusBadRequest, "validation", "either cron_expr or once_at is required", false)
			return
		}

		// Calculate next_run.
		var nextRun *int64
		now := time.Now().Unix()
		if req.OnceAt != nil {
			nextRun = req.OnceAt
		}

		task := &store.ScheduledTask{
			ID:        uuid.NewString(),
			Name:      req.Name,
			CronExpr:  req.CronExpr,
			OnceAt:    req.OnceAt,
			SessionID: req.SessionID,
			ClusterID: req.ClusterID,
			CreatedBy: req.CreatedBy,
			Enabled:   true,
			CreatedAt: now,
			NextRun:   nextRun,
		}

		if err := d.DB.CreateScheduledTask(r.Context(), task); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), false)
			return
		}

		// Add to scheduler.
		d.Scheduler.ScheduleTask(task)

		writeJSON(w, http.StatusOK, toResponse(task))
	}
}

func deleteScheduledTaskHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "validation", "id required", false)
			return
		}
		// Unschedule first (no-op if not found).
		d.Scheduler.UnscheduleTask(id)
		if err := d.DB.DeleteScheduledTask(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrScheduledTaskNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "task not found", false)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), false)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	}
}

func updateScheduledTaskHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "validation", "id required", false)
			return
		}
		var req updateScheduledTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error(), false)
			return
		}
		updates := map[string]any{}
		if req.Enabled != nil {
			updates["enabled"] = btof(*req.Enabled)
			if !*req.Enabled {
				d.Scheduler.UnscheduleTask(id)
			}
		}
		if req.Name != nil {
			updates["name"] = *req.Name
		}
		if req.CronExpr != nil {
			updates["cron_expr"] = *req.CronExpr
		}
		if err := d.DB.UpdateScheduledTask(r.Context(), id, updates); err != nil {
			if errors.Is(err, store.ErrScheduledTaskNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "task not found", false)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), false)
			return
		}
		t, err := d.DB.GetScheduledTask(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), false)
			return
		}
		// Re-schedule if enabled and has cron.
		if t.Enabled {
			d.Scheduler.ScheduleTask(t)
		}
		writeJSON(w, http.StatusOK, toResponse(t))
	}
}

func runScheduledTaskHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "validation", "id required", false)
			return
		}
		t, err := d.DB.GetScheduledTask(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrScheduledTaskNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "task not found", false)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), false)
			return
		}
		// Trigger synchronously for the "run now" button.
		if err := triggerTask(r.Context(), d, t); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), false)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "triggered"})
	}
}

// triggerTask fires a scheduled task immediately.
func triggerTask(ctx context.Context, d Deps, t *store.ScheduledTask) error {
	runID := uuid.NewString()
	runAt := time.Now().Unix()
	run := &store.ScheduledRun{
		ID:     runID,
		TaskID: t.ID,
		RunAt:  runAt,
		Status: "running",
	}
	if err := d.DB.CreateScheduledRun(ctx, run); err != nil {
		return err
	}
	userMsg := "Scheduled task: " + t.Name
	msg := store.Message{
		ID:        uuid.NewString(),
		SessionID: t.SessionID,
		Role:      "user",
		Content:   &userMsg,
		Source:    "scheduled",
	}
	if err := d.DB.BatchInsertMessages(ctx, []store.Message{msg}); err != nil {
		d.DB.UpdateScheduledRun(ctx, runID, "failed", err)
		return err
	}
	clusterID := ""
	if t.ClusterID != nil {
		clusterID = *t.ClusterID
	}
	runner := d.RunnerFactory.NewRunner(t.SessionID, clusterID)
	if err := runner.Run(ctx, userMsg); err != nil {
		d.DB.UpdateScheduledRun(ctx, runID, "failed", err)
		return err
	}
	d.DB.UpdateScheduledRun(ctx, runID, "success", nil)

	// Update task stats.
	now := time.Now().Unix()
	updates := map[string]any{
		"last_run":  now,
		"run_count": t.RunCount + 1,
	}
	d.DB.UpdateScheduledTask(ctx, t.ID, updates)
	return nil
}

// --- Helpers ---

type scheduledTaskResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	CronExpr  *string `json:"cron_expr,omitempty"`
	OnceAt    *int64  `json:"once_at,omitempty"`
	SessionID string  `json:"session_id"`
	Enabled   bool    `json:"enabled"`
	ClusterID *string `json:"cluster_id,omitempty"`
	CreatedBy string  `json:"created_by"`
	CreatedAt int64   `json:"created_at"`
	NextRun   *int64  `json:"next_run,omitempty"`
	LastRun   *int64  `json:"last_run,omitempty"`
	RunCount  int     `json:"run_count"`
}

func toResponse(t *store.ScheduledTask) scheduledTaskResponse {
	return scheduledTaskResponse{
		ID:        t.ID,
		Name:      t.Name,
		CronExpr:  t.CronExpr,
		OnceAt:    t.OnceAt,
		SessionID: t.SessionID,
		Enabled:   t.Enabled,
		ClusterID: t.ClusterID,
		CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt,
		NextRun:   t.NextRun,
		LastRun:   t.LastRun,
		RunCount:  t.RunCount,
	}
}

func btof(b bool) int {
	if b {
		return 1
	}
	return 0
}

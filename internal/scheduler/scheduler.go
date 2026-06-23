package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/threestoneliu/kubernetes-agent/internal/agent"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

// RunnerFactory creates a ready-to-run agent.Runner for a session.
// Built by cmd/server from server.Deps.RunnerFactory.
type RunnerFactory func(sessionID, clusterID string) *agent.Runner

// Scheduler runs in a background goroutine and dispatches scheduled tasks
// when their cron expression or one-shot time fires. On trigger it writes a
// "scheduled" message to the session and calls the agent runner so the
// result appears in the chat.
type Scheduler struct {
	store        *store.DB
	runnerFactory RunnerFactory
	sessionMgr   *agent.SessionManager
	cron         *cron.Cron
	entries     map[cron.EntryID]string // cronEntryID -> taskID
	mu          sync.Mutex
}

// NewScheduler creates a background scheduler.
func NewScheduler(db *store.DB, runnerFactory RunnerFactory, sessionMgr *agent.SessionManager) *Scheduler {
	return &Scheduler{
		store:        db,
		runnerFactory: runnerFactory,
		sessionMgr:   sessionMgr,
		cron:         cron.New(cron.WithSeconds()),
		entries:     map[cron.EntryID]string{},
	}
}

// Run starts the scheduler. It restores all enabled tasks from the store
// and begins polling. Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	slog.Info("scheduler: starting")
	if err := s.restore(ctx); err != nil {
		slog.Error("scheduler: restore error", "err", err)
	}
	go s.cron.Run()
	<-ctx.Done()
	slog.Info("scheduler: stopped")
	return ctx.Err()
}

// restore loads all enabled tasks from the store and schedules them.
func (s *Scheduler) restore(ctx context.Context) error {
	tasks, err := s.store.GetEnabledScheduledTasks(ctx)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		if err := s.ScheduleTask(t); err != nil {
			slog.Error("scheduler: restore schedule error", "task_id", t.ID, "err", err)
		}
	}
	slog.Info("scheduler: restored tasks", "count", len(tasks))
	return nil
}

// ScheduleTask adds a task to the cron scheduler.
func (s *Scheduler) ScheduleTask(t *store.ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scheduleTaskLocked(t)
}

// UnscheduleTask removes a task from the cron scheduler.
func (s *Scheduler) UnscheduleTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for entryID, id := range s.entries {
		if id == taskID {
			s.cron.Remove(entryID)
			delete(s.entries, entryID)
			return
		}
	}
}

func (s *Scheduler) scheduleTaskLocked(t *store.ScheduledTask) error {
	if !t.Enabled {
		return nil
	}
	var spec string
	if t.CronExpr != nil {
		spec = *t.CronExpr
	} else {
		// one-shot: use "@every 1s" and check once_at in trigger
		spec = "@every 1s"
	}
	taskID := t.ID
	entryID, err := s.cron.AddFunc(spec, func() {
		s.checkAndTrigger(context.Background(), taskID)
	})
	if err != nil {
		slog.Error("scheduler: add func error", "task_id", t.ID, "err", err)
		return err
	}
	s.entries[entryID] = t.ID
	return nil
}

// checkAndTrigger checks if the task is due and triggers or skips.
func (s *Scheduler) checkAndTrigger(ctx context.Context, taskID string) {
	t, err := s.store.GetScheduledTask(ctx, taskID)
	if err != nil {
		slog.Error("scheduler: get task error", "task_id", taskID, "err", err)
		return
	}
	if !t.Enabled {
		return
	}

	// For one-shot tasks, check if next_run has actually passed.
	if t.OnceAt != nil {
		if time.Now().Unix() < *t.OnceAt {
			return // not yet
		}
	}

	if err := s.trigger(ctx, t); err != nil {
		slog.Error("scheduler: trigger error", "task_id", taskID, "err", err)
		return
	}

	// For one-shot tasks, disable after firing.
	if t.OnceAt != nil {
		updates := map[string]any{"enabled": 0}
		if err := s.store.UpdateScheduledTask(ctx, taskID, updates); err != nil {
			slog.Error("scheduler: disable one-shot error", "task_id", taskID, "err", err)
		}
	}
}

// trigger fires the task: writes a scheduled message and runs the agent.
func (s *Scheduler) trigger(ctx context.Context, t *store.ScheduledTask) error {
	runID := uuid.NewString()
	runAt := time.Now().Unix()

	run := &store.ScheduledRun{
		ID:     runID,
		TaskID: t.ID,
		RunAt:  runAt,
		Status: "running",
	}
	if err := s.store.CreateScheduledRun(ctx, run); err != nil {
		slog.Error("scheduler: create run record error", "task_id", t.ID, "err", err)
	}

	// Build the user message.
	userMsg := "Scheduled task: " + t.Name

	// Write the scheduled message to the session.
	msg := store.Message{
		ID:        uuid.NewString(),
		SessionID: t.SessionID,
		Role:      "user",
		Content:   &userMsg,
		Source:    "scheduled",
	}
	if err := s.store.BatchInsertMessages(ctx, []store.Message{msg}); err != nil {
		s.updateRun(ctx, runID, "failed", err)
		return err
	}

	// Create runner for this session.
	clusterID := ""
	if t.ClusterID != nil {
		clusterID = *t.ClusterID
	}
	runner := s.runnerFactory(t.SessionID, clusterID)

	if err := runner.Run(ctx, userMsg); err != nil {
		s.updateRun(ctx, runID, "failed", err)
		return err
	}

	s.updateRun(ctx, runID, "success", nil)
	s.updateTaskStats(ctx, t)
	return nil
}

func (s *Scheduler) updateRun(ctx context.Context, runID, status string, runErr error) {
	if runErr != nil {
		s.store.UpdateScheduledRun(ctx, runID, status, runErr)
	} else {
		s.store.UpdateScheduledRun(ctx, runID, status, nil)
	}
}

func (s *Scheduler) updateTaskStats(ctx context.Context, t *store.ScheduledTask) {
	now := time.Now().Unix()
	var nextRun *int64
	if t.CronExpr != nil {
		parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if schedule, err := parser.Parse(*t.CronExpr); err == nil {
			next := schedule.Next(time.Now())
			v := next.Unix()
			nextRun = &v
		}
	}
	updates := map[string]any{
		"last_run":  now,
		"run_count": t.RunCount + 1,
	}
	if nextRun != nil {
		updates["next_run"] = *nextRun
	}
	s.store.UpdateScheduledTask(ctx, t.ID, updates)
}

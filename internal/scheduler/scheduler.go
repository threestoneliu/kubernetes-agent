package scheduler

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

// Scheduler runs in a background goroutine and dispatches scheduled tasks
// when their cron expression or one-shot time fires. On trigger it writes a
// "scheduled" message to the session and calls the agent runner so the
// result appears in the chat.
type Scheduler struct {
	store         *store.DB
	runnerFactory any // func(sessionID, clusterID) any
	cron          *cron.Cron
	entries       map[cron.EntryID]string
	mu            sync.Mutex
}

// NewScheduler creates a background scheduler.
func NewScheduler(db *store.DB, runnerFactory any) *Scheduler {
	return &Scheduler{
		store:         db,
		runnerFactory: runnerFactory,
		cron:          cron.New(cron.WithSeconds()),
		entries:       map[cron.EntryID]string{},
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

func (s *Scheduler) restore(ctx context.Context) error {
	tasks, err := s.store.GetEnabledScheduledTasks(ctx)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		s.ScheduleTask(t)
	}
	slog.Info("scheduler: restored tasks", "count", len(tasks))
	return nil
}

// ScheduleTask adds a task to the cron scheduler.
func (s *Scheduler) ScheduleTask(t *store.ScheduledTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduleTaskLocked(t)
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

func (s *Scheduler) scheduleTaskLocked(t *store.ScheduledTask) {
	if !t.Enabled {
		return
	}
	var spec string
	if t.CronExpr != nil {
		expr := *t.CronExpr
		// Prepend "0 " for standard 5-field expressions
		// (minute hour day month weekday) so they become 6-field
		// (second minute hour day month weekday) as required by WithSeconds().
		// Special expressions starting with @ are passed through.
		if strings.HasPrefix(expr, "@") {
			spec = expr
		} else {
			spec = "0 " + expr
		}
	} else {
		spec = "@every 1s"
	}
	taskID := t.ID
	entryID, err := s.cron.AddFunc(spec, func() {
		s.checkAndTrigger(context.Background(), taskID)
	})
	if err != nil {
		slog.Error("scheduler: add func error", "task_id", t.ID, "err", err)
		return
	}
	s.entries[entryID] = t.ID
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
	if t.OnceAt != nil && time.Now().Unix() < *t.OnceAt {
		return
	}
	if err := s.trigger(ctx, t); err != nil {
		slog.Error("scheduler: trigger error", "task_id", taskID, "err", err)
		return
	}
	if t.OnceAt != nil {
		updates := map[string]any{"enabled": 0}
		if err := s.store.UpdateScheduledTask(ctx, taskID, updates); err != nil {
			slog.Error("scheduler: disable one-shot error", "task_id", taskID, "err", err)
		}
	}
}

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
		return err
	}
	userMsg := t.Prompt
	msg := store.Message{
		ID:        uuid.NewString(),
		SessionID: t.SessionID,
		Role:      "user",
		Content:   &userMsg,
		Source:    "scheduled",
	}
	if err := s.store.BatchInsertMessages(ctx, []store.Message{msg}); err != nil {
		s.store.UpdateScheduledRun(ctx, runID, "failed", err)
		return err
	}
	// Use the session's cluster_id (task.ClusterID is rarely set;
	// session.ClusterID is the source of truth for the cluster the
	// user is operating against).
	session, err := s.store.GetSession(ctx, t.SessionID)
	if err != nil {
		s.store.UpdateScheduledRun(ctx, runID, "failed", err)
		return err
	}
	clusterID := ""
	if session.ClusterID != nil {
		clusterID = *session.ClusterID
	}
	runner := s.runnerFactory.(func(string, string) any)(t.SessionID, clusterID)
	runner.(interface{ SetSession(string, string) }).SetSession(t.SessionID, clusterID)
	if err := runner.(interface{ Run(context.Context, string) error }).Run(ctx, userMsg); err != nil {
		s.store.UpdateScheduledRun(ctx, runID, "failed", err)
		return err
	}
	s.store.UpdateScheduledRun(ctx, runID, "success", nil)
	s.updateTaskStats(ctx, t)
	return nil
}

func (s *Scheduler) updateTaskStats(ctx context.Context, t *store.ScheduledTask) {
	now := time.Now().Unix()
	updates := map[string]any{
		"last_run":  now,
		"run_count": t.RunCount + 1,
	}
	if t.CronExpr != nil {
		parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if schedule, err := parser.Parse(*t.CronExpr); err == nil {
			next := schedule.Next(time.Now())
			v := next.Unix()
			updates["next_run"] = v
		}
	}
	s.store.UpdateScheduledTask(ctx, t.ID, updates)
}

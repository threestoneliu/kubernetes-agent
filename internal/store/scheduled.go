package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ScheduledTask represents a scheduled task.
type ScheduledTask struct {
	ID        string
	Name      string
	CronExpr  *string // nil = one-shot task
	OnceAt    *int64 // UNIX timestamp for one-shot tasks
	SessionID string
	Enabled   bool
	CreatedBy string
	ClusterID *string
	CreatedAt int64
	NextRun   *int64 // UNIX timestamp
	LastRun   *int64
	RunCount  int
}

// ScheduledRun represents a single execution of a scheduled task.
type ScheduledRun struct {
	ID      string
	TaskID  string
	RunAt   int64
	Status  string // "running", "success", "failed", "skipped"
	Error   *string
}

var ErrScheduledTaskNotFound = errors.New("scheduled task not found")

// CreateScheduledTask inserts a new scheduled task.
func (d *DB) CreateScheduledTask(ctx context.Context, t *ScheduledTask) error {
	now := time.Now().Unix()
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	_, err := d.ExecContext(ctx,
		`INSERT INTO scheduled_tasks
		 (id, name, cron_expr, once_at, session_id, enabled, created_by, cluster_id, created_at, next_run, last_run, run_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.CronExpr, t.OnceAt, t.SessionID,
		btof(t.Enabled), t.CreatedBy, t.ClusterID, t.CreatedAt,
		t.NextRun, t.LastRun, t.RunCount)
	return err
}

// GetScheduledTasks returns all tasks, optionally filtered by sessionID.
// If sessionID is empty, returns all tasks.
func (d *DB) GetScheduledTasks(ctx context.Context, sessionID string) ([]*ScheduledTask, error) {
	var rows *sql.Rows
	var err error
	if sessionID == "" {
		rows, err = d.QueryContext(ctx,
			`SELECT id, name, cron_expr, once_at, session_id, enabled, created_by, cluster_id,
			        created_at, next_run, last_run, run_count
			 FROM scheduled_tasks ORDER BY created_at DESC`)
	} else {
		rows, err = d.QueryContext(ctx,
			`SELECT id, name, cron_expr, once_at, session_id, enabled, created_by, cluster_id,
			        created_at, next_run, last_run, run_count
			 FROM scheduled_tasks WHERE session_id = ? ORDER BY created_at DESC`, sessionID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScheduledTasks(rows)
}

// GetScheduledTask returns a single task by ID.
func (d *DB) GetScheduledTask(ctx context.Context, id string) (*ScheduledTask, error) {
	row := d.QueryRowContext(ctx,
		`SELECT id, name, cron_expr, once_at, session_id, enabled, created_by, cluster_id,
		        created_at, next_run, last_run, run_count
		 FROM scheduled_tasks WHERE id = ?`, id)
	t, err := scanScheduledTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrScheduledTaskNotFound
	}
	return t, err
}

// GetEnabledScheduledTasks returns all enabled tasks for scheduler restore.
func (d *DB) GetEnabledScheduledTasks(ctx context.Context) ([]*ScheduledTask, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, name, cron_expr, once_at, session_id, enabled, created_by, cluster_id,
		        created_at, next_run, last_run, run_count
		 FROM scheduled_tasks WHERE enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScheduledTasks(rows)
}

// UpdateScheduledTask updates fields of a scheduled task.
func (d *DB) UpdateScheduledTask(ctx context.Context, id string, updates map[string]any) error {
	// Build SET clause from updates map.
	set := ""
	args := []any{}
	for k, v := range updates {
		if set != "" {
			set += ", "
		}
		set += k + " = ?"
		args = append(args, v)
	}
	if set == "" {
		return nil
	}
	args = append(args, id)
	_, err := d.ExecContext(ctx, "UPDATE scheduled_tasks SET "+set+" WHERE id = ?", args...)
	return err
}

// DeleteScheduledTask removes a scheduled task.
func (d *DB) DeleteScheduledTask(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM scheduled_tasks WHERE id = ?`, id)
	return err
}

// CreateScheduledRun inserts a new run record.
func (d *DB) CreateScheduledRun(ctx context.Context, r *ScheduledRun) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO scheduled_runs (id, task_id, run_at, status, error) VALUES (?, ?, ?, ?, ?)`,
		r.ID, r.TaskID, r.RunAt, r.Status, r.Error)
	return err
}

// UpdateScheduledRun updates the status and error of a run.
func (d *DB) UpdateScheduledRun(ctx context.Context, id, status string, runErr error) error {
	var errStr *string
	if runErr != nil {
		s := runErr.Error()
		errStr = &s
	}
	_, err := d.ExecContext(ctx,
		`UPDATE scheduled_runs SET status = ?, error = ? WHERE id = ?`,
		status, errStr, id)
	return err
}

// --- helpers ---

func scanScheduledTasks(rows *sql.Rows) ([]*ScheduledTask, error) {
	var out []*ScheduledTask
	for rows.Next() {
		t, err := scanScheduledTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func scanScheduledTask(scanner interface{ Scan(...any) error }) (*ScheduledTask, error) {
	var (
		id, name, sessionID, createdBy           string
		cronExpr                            sql.NullString
		onceAt, nextRun, lastRun            sql.NullInt64
		clusterID                           sql.NullString
		enabled                             int
		createdAt                           int64
		runCount                            int
	)
	if err := scanner.Scan(
		&id, &name, &cronExpr, &onceAt, &sessionID, &enabled,
		&createdBy, &clusterID, &createdAt, &nextRun, &lastRun, &runCount,
	); err != nil {
		return nil, err
	}
	t := &ScheduledTask{
		ID:        id,
		Name:      name,
		SessionID: sessionID,
		Enabled:   enabled == 1,
		CreatedBy: createdBy,
		CreatedAt: createdAt,
		RunCount:  runCount,
	}
	if cronExpr.Valid {
		t.CronExpr = &cronExpr.String
	}
	if onceAt.Valid {
		t.OnceAt = &onceAt.Int64
	}
	if nextRun.Valid {
		t.NextRun = &nextRun.Int64
	}
	if lastRun.Valid {
		t.LastRun = &lastRun.Int64
	}
	if clusterID.Valid {
		t.ClusterID = &clusterID.String
	}
	return t, nil
}

func btof(b bool) int {
	if b {
		return 1
	}
	return 0
}

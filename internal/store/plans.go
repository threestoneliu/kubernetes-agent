package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Plan status values (enforced semantically by callers, not by SQL).
const (
	PlanStatusPending   = "pending"
	PlanStatusApproved  = "approved"
	PlanStatusExecuted  = "executed"
	PlanStatusCancelled = "cancelled"
	PlanStatusDenied    = "denied"
)

type Plan struct {
	ID         string
	SessionID  string
	OpsJSON    string
	DiffsJSON  string
	Risk       string
	Status     string
	CreatedAt  int64
	ExecutedAt *int64
}

func (d *DB) CreatePlan(ctx context.Context, p Plan) error {
	now := time.Now().Unix()
	_, err := d.ExecContext(ctx,
		`INSERT INTO plans (id, session_id, ops_json, diffs_json, risk, status, created_at, executed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.SessionID, p.OpsJSON, p.DiffsJSON, p.Risk, p.Status, now, p.ExecutedAt)
	return err
}

func (d *DB) GetPlan(ctx context.Context, id string) (Plan, error) {
	var p Plan
	var executedTS sql.NullInt64
	var createdTS int64
	err := d.QueryRowContext(ctx,
		`SELECT id, session_id, ops_json, diffs_json, risk, status, created_at, executed_at
		 FROM plans WHERE id = ?`, id).
		Scan(&p.ID, &p.SessionID, &p.OpsJSON, &p.DiffsJSON, &p.Risk, &p.Status, &createdTS, &executedTS)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.CreatedAt = createdTS
	if executedTS.Valid {
		v := executedTS.Int64
		p.ExecutedAt = &v
	}
	return p, nil
}

func (d *DB) UpdatePlanStatus(ctx context.Context, id, status string) error {
	res, err := d.ExecContext(ctx,
		`UPDATE plans SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) MarkExecuted(ctx context.Context, id string) error {
	now := time.Now().Unix()
	res, err := d.ExecContext(ctx,
		`UPDATE plans SET status = ?, executed_at = ? WHERE id = ?`,
		PlanStatusExecuted, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

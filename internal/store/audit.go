package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type AuditEntry struct {
	ID        int64
	SessionID *string
	ClusterID *string
	Action    string
	Target    *string
	Status    string
	Message   *string
	CreatedAt int64
}

// AuditFilter narrows ListAudit results. Zero-value fields are ignored.
type AuditFilter struct {
	SessionID string
	ClusterID string
	Action    string
	Limit     int
}

// Append inserts a new audit log row and returns the auto-generated ID.
func (d *DB) Append(ctx context.Context, e AuditEntry) (int64, error) {
	return d.AppendAudit(ctx, e)
}

// AppendAudit inserts a new audit log row and returns the auto-generated ID.
func (d *DB) AppendAudit(ctx context.Context, e AuditEntry) (int64, error) {
	now := time.Now().Unix()
	res, err := d.ExecContext(ctx, `
		INSERT INTO audit_log (session_id, cluster_id, action, target, status, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.SessionID, e.ClusterID, e.Action, e.Target, e.Status, e.Message, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListAudit returns audit entries matching the filter, ordered by id descending.
func (d *DB) ListAudit(ctx context.Context, f AuditFilter) ([]AuditEntry, error) {
	var (
		conds []string
		args  []any
	)
	if f.SessionID != "" {
		conds = append(conds, "session_id = ?")
		args = append(args, f.SessionID)
	}
	if f.ClusterID != "" {
		conds = append(conds, "cluster_id = ?")
		args = append(args, f.ClusterID)
	}
	if f.Action != "" {
		conds = append(conds, "action = ?")
		args = append(args, f.Action)
	}
	q := `SELECT id, session_id, cluster_id, action, target, status, message, created_at
	      FROM audit_log`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY id DESC"
	if f.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, f.Limit)
	}
	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var sessionID, clusterID, target, message sql.NullString
		if err := rows.Scan(&e.ID, &sessionID, &clusterID, &e.Action, &target, &e.Status, &message, &e.CreatedAt); err != nil {
			return nil, err
		}
		if sessionID.Valid {
			v := sessionID.String
			e.SessionID = &v
		}
		if clusterID.Valid {
			v := clusterID.String
			e.ClusterID = &v
		}
		if target.Valid {
			v := target.String
			e.Target = &v
		}
		if message.Valid {
			v := message.String
			e.Message = &v
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

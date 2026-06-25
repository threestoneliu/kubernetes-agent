package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Session struct {
	ID        string
	Title     string
	ClusterID *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (d *DB) CreateSession(ctx context.Context, s Session) error {
	now := time.Now().Unix()
	_, err := d.ExecContext(ctx,
		`INSERT INTO sessions (id, title, cluster_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		s.ID, s.Title, s.ClusterID, now, now)
	return err
}

func (d *DB) GetSession(ctx context.Context, id string) (Session, error) {
	var s Session
	var clusterID sql.NullString
	var createdTS, updatedTS int64
	err := d.QueryRowContext(ctx,
		`SELECT id, title, cluster_id, created_at, updated_at FROM sessions WHERE id = ?`, id).
		Scan(&s.ID, &s.Title, &clusterID, &createdTS, &updatedTS)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	if err != nil {
		return s, err
	}
	if clusterID.Valid {
		v := clusterID.String
		s.ClusterID = &v
	}
	s.CreatedAt = time.Unix(createdTS, 0)
	s.UpdatedAt = time.Unix(updatedTS, 0)
	return s, nil
}

func (d *DB) ListSessions(ctx context.Context) ([]Session, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, title, cluster_id, created_at, updated_at FROM sessions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var s Session
		var clusterID sql.NullString
		var createdTS, updatedTS int64
		if err := rows.Scan(&s.ID, &s.Title, &clusterID, &createdTS, &updatedTS); err != nil {
			return nil, err
		}
		if clusterID.Valid {
			v := clusterID.String
			s.ClusterID = &v
		}
		s.CreatedAt = time.Unix(createdTS, 0)
		s.UpdatedAt = time.Unix(updatedTS, 0)
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListSessionsFiltered returns sessions matching the optional title
// query, sorted by the requested column, paginated via limit/offset.
// sort ∈ {created_at, updated_at, title}, order ∈ {asc, desc}.
// q is matched case-insensitively against title via COLLATE NOCASE.
func (d *DB) ListSessionsFiltered(ctx context.Context, q, sort, order string, limit, offset int) ([]Session, error) {
	allowedSort := map[string]string{
		"created_at": "created_at",
		"updated_at": "updated_at",
		"title":      "title",
	}
	col, ok := allowedSort[sort]
	if !ok {
		return nil, fmt.Errorf("invalid sort %q", sort)
	}
	switch order {
	case "asc", "desc":
	default:
		return nil, fmt.Errorf("invalid order %q", order)
	}
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	var (
		rows *sql.Rows
		err  error
	)
	if q == "" {
		rows, err = d.QueryContext(ctx,
			`SELECT id, title, cluster_id, created_at, updated_at FROM sessions
			 ORDER BY `+col+` COLLATE NOCASE `+strings.ToUpper(order)+` LIMIT ? OFFSET ?`,
			limit, offset)
	} else {
		rows, err = d.QueryContext(ctx,
			`SELECT id, title, cluster_id, created_at, updated_at FROM sessions
			 WHERE title LIKE ? COLLATE NOCASE
			 ORDER BY `+col+` COLLATE NOCASE `+strings.ToUpper(order)+` LIMIT ? OFFSET ?`,
			"%"+q+"%", limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var s Session
		var clusterID sql.NullString
		var createdTS, updatedTS int64
		if err := rows.Scan(&s.ID, &s.Title, &clusterID, &createdTS, &updatedTS); err != nil {
			return nil, err
		}
		if clusterID.Valid {
			v := clusterID.String
			s.ClusterID = &v
		}
		s.CreatedAt = time.Unix(createdTS, 0)
		s.UpdatedAt = time.Unix(updatedTS, 0)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) UpdateSessionTitle(ctx context.Context, id, title string) error {
	now := time.Now().Unix()
	res, err := d.ExecContext(ctx,
		`UPDATE sessions SET title = ?, updated_at = ? WHERE id = ?`,
		title, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSession removes a session row (and its plans / messages /
// audit entries via ON DELETE CASCADE) and returns the number of
// rows actually deleted.
func (d *DB) DeleteSession(ctx context.Context, id string) (int64, error) {
	res, err := d.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return 0, ErrNotFound
	}
	return n, nil
}

// DeleteAllSessions removes every session row. Returns the count.
func (d *DB) DeleteAllSessions(ctx context.Context) (int64, error) {
	res, err := d.ExecContext(ctx, `DELETE FROM sessions`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// DeleteSessionsWithoutScheduledTasks removes sessions that are not bound to any
// scheduled task. Returns the count of deleted sessions.
func (d *DB) DeleteSessionsWithoutScheduledTasks(ctx context.Context) (int64, error) {
	res, err := d.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE id NOT IN (SELECT DISTINCT session_id FROM scheduled_tasks)
	`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

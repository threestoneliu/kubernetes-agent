package store

import (
	"context"
	"database/sql"
	"errors"
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

func (d *DB) DeleteSession(ctx context.Context, id string) error {
	res, err := d.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

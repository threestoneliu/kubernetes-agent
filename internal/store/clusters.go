package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Cluster struct {
	ID             string
	Name           string
	Server         string
	User           string
	KubeconfigBlob []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (d *DB) CreateCluster(ctx context.Context, c Cluster) error {
	now := time.Now().Unix()
	_, err := d.ExecContext(ctx,
		`INSERT INTO clusters (id, name, server, user, kubeconfig_blob, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Server, c.User, c.KubeconfigBlob, now, now)
	return err
}

func (d *DB) GetCluster(ctx context.Context, id string) (Cluster, error) {
	var c Cluster
	var ts int64
	err := d.QueryRowContext(ctx,
		`SELECT id, name, server, user, kubeconfig_blob, created_at, updated_at FROM clusters WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Server, &c.User, &c.KubeconfigBlob, &ts, &ts)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	c.CreatedAt = time.Unix(ts, 0)
	c.UpdatedAt = time.Unix(ts, 0)
	return c, err
}

func (d *DB) ListClusters(ctx context.Context) ([]Cluster, error) {
	rows, err := d.QueryContext(ctx, `SELECT id, name, server, user, kubeconfig_blob, created_at, updated_at FROM clusters ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cluster
	for rows.Next() {
		var c Cluster
		var ts int64
		if err := rows.Scan(&c.ID, &c.Name, &c.Server, &c.User, &c.KubeconfigBlob, &ts, &ts); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(ts, 0)
		c.UpdatedAt = time.Unix(ts, 0)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) DeleteCluster(ctx context.Context, id string) error {
	res, err := d.ExecContext(ctx, `DELETE FROM clusters WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

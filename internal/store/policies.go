package store

import (
	"context"
	"time"
)

type Policy struct {
	ID        string
	Name      string
	YAML      string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (d *DB) UpsertPolicy(ctx context.Context, p Policy) error {
	now := time.Now().Unix()
	enabled := 0
	if p.Enabled {
		enabled = 1
	}
	// Upsert: if id exists, update fields + bump updated_at; if not, insert.
	_, err := d.ExecContext(ctx, `
		INSERT INTO policies (id, name, yaml, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			yaml = excluded.yaml,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`, p.ID, p.Name, p.YAML, enabled, now, now)
	return err
}

func (d *DB) ListEnabledPolicies(ctx context.Context) ([]Policy, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, name, yaml, enabled, created_at, updated_at
		 FROM policies WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Policy
	for rows.Next() {
		var p Policy
		var enabled int
		var createdTS, updatedTS int64
		if err := rows.Scan(&p.ID, &p.Name, &p.YAML, &enabled, &createdTS, &updatedTS); err != nil {
			return nil, err
		}
		p.Enabled = enabled != 0
		p.CreatedAt = time.Unix(createdTS, 0)
		p.UpdatedAt = time.Unix(updatedTS, 0)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) SetEnabled(ctx context.Context, id string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	res, err := d.ExecContext(ctx,
		`UPDATE policies SET enabled = ?, updated_at = ? WHERE id = ?`,
		v, time.Now().Unix(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SeedDefaultsIfEmpty inserts 4 placeholder default policies if the policies
// table is empty. Task 6 (policy engine) will later replace the empty YAML
// bodies with real rule definitions and re-seed.
func (d *DB) SeedDefaultsIfEmpty(ctx context.Context) error {
	var n int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM policies`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	defaults := []Policy{
		{ID: "default-no-delete-system-ns", Name: "default-no-delete-system-ns", Enabled: true},
		{ID: "default-no-apply-cluster-resources", Name: "default-no-apply-cluster-resources", Enabled: true},
		{ID: "default-no-unsafe-fields", Name: "default-no-unsafe-fields", Enabled: true},
		{ID: "default-confirm-prod-ns", Name: "default-confirm-prod-ns", Enabled: true},
	}
	for _, p := range defaults {
		if err := d.UpsertPolicy(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

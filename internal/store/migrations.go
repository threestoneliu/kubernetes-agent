package store

import (
	"fmt"
)

var migrations = []string{
	// 1: initial schema
	`CREATE TABLE IF NOT EXISTS clusters (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		server TEXT NOT NULL,
		user TEXT NOT NULL,
		kubeconfig_blob BLOB NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		cluster_id TEXT REFERENCES clusters(id) ON DELETE SET NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		role TEXT NOT NULL,
		content TEXT,
		tool_calls TEXT,
		tool_call_id TEXT,
		reasoning TEXT,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);
	CREATE TABLE IF NOT EXISTS plans (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		ops_json TEXT NOT NULL,
		diffs_json TEXT NOT NULL,
		risk TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		executed_at INTEGER
	);
	CREATE TABLE IF NOT EXISTS policies (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		yaml TEXT NOT NULL,
		enabled INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT,
		cluster_id TEXT,
		action TEXT NOT NULL,
		target TEXT,
		status TEXT NOT NULL,
		message TEXT,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	);`,
	// 2: scheduled tasks
	`CREATE TABLE IF NOT EXISTS scheduled_tasks (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		cron_expr   TEXT,
		once_at     INTEGER,
		session_id  TEXT NOT NULL,
		enabled     INTEGER DEFAULT 1,
		created_by  TEXT,
		cluster_id  TEXT,
		created_at  INTEGER,
		next_run    INTEGER,
		last_run    INTEGER,
		run_count   INTEGER DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS scheduled_runs (
		id      TEXT PRIMARY KEY,
		task_id TEXT REFERENCES scheduled_tasks(id) ON DELETE CASCADE,
		run_at  INTEGER,
		status  TEXT,
		error   TEXT
	);`,
	// 3: add source column to messages (for existing dbs created before migration 1)
	`ALTER TABLE messages ADD COLUMN source TEXT DEFAULT 'user';`,
}

func (d *DB) Migrate() error {
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}
	for i, m := range migrations {
		v := i + 1
		var n int
		if err := d.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, v).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(m); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d: %w", v, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, strftime('%s','now'))`, v); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

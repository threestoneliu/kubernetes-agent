package store

import (
	"context"
	"database/sql"
	"time"
)

type Message struct {
	ID         string
	SessionID  string
	Role       string
	Content    *string
	ToolCalls  *string
	ToolCallID *string
	Reasoning  *string
	CreatedAt  int64
}

type messageRow struct {
	id, sessionID, role                     string
	content, toolCalls, toolCallID, reasoning sql.NullString
	createdAt                                int64
}

func scanMessage(scanner interface {
	Scan(dest ...any) error
}) (Message, error) {
	var row messageRow
	if err := scanner.Scan(
		&row.id, &row.sessionID, &row.role,
		&row.content, &row.toolCalls, &row.toolCallID, &row.reasoning,
		&row.createdAt,
	); err != nil {
		return Message{}, err
	}
	m := Message{
		ID:        row.id,
		SessionID: row.sessionID,
		Role:      row.role,
		CreatedAt: row.createdAt,
	}
	if row.content.Valid {
		v := row.content.String
		m.Content = &v
	}
	if row.toolCalls.Valid {
		v := row.toolCalls.String
		m.ToolCalls = &v
	}
	if row.toolCallID.Valid {
		v := row.toolCallID.String
		m.ToolCallID = &v
	}
	if row.reasoning.Valid {
		v := row.reasoning.String
		m.Reasoning = &v
	}
	return m, nil
}

// BatchInsertMessages inserts multiple messages atomically in a single transaction.
// If CreatedAt is zero for a given message, the current Unix time is used.
func (d *DB) BatchInsertMessages(ctx context.Context, msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO messages (id, session_id, role, content, tool_calls, tool_call_id, reasoning, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	now := time.Now().Unix()
	for _, m := range msgs {
		ts := m.CreatedAt
		if ts == 0 {
			ts = now
		}
		if _, err := stmt.ExecContext(ctx,
			m.ID, m.SessionID, m.Role, m.Content, m.ToolCalls, m.ToolCallID, m.Reasoning, ts,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) ListMessagesBySession(ctx context.Context, sessionID string) ([]Message, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, session_id, role, content, tool_calls, tool_call_id, reasoning, created_at
		 FROM messages WHERE session_id = ? ORDER BY ROWID ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

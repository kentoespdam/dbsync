package storage

import (
	"context"
	"database/sql"
	"time"
)


type Checkpoint struct {
	ID                 int64
	ConnectionID       int64
	TableName          string
	LastBatchCompleted int
	LastPKValue        string // serialized (single PK as plain string, composite as JSON array)
	StartedAt          time.Time
	UpdatedAt          time.Time
	Status             string // 'running' | 'interrupted' | 'completed' | 'failed'
}

type CheckpointRepo struct {
	db *sql.DB
}

func (d *DB) Checkpoints() *CheckpointRepo {
	return &CheckpointRepo{db: d.db}
}

func (r *CheckpointRepo) Get(ctx context.Context, connID int64, table string) (Checkpoint, error) {
	query := `
		SELECT id, connection_id, table_name, last_batch_completed, last_pk_value, started_at, updated_at, status
		FROM sync_checkpoints
		WHERE connection_id = ? AND table_name = ?
	`
	var c Checkpoint
	err := r.db.QueryRowContext(ctx, query, connID, table).Scan(
		&c.ID, &c.ConnectionID, &c.TableName, &c.LastBatchCompleted, &c.LastPKValue, &c.StartedAt, &c.UpdatedAt, &c.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Checkpoint{}, ErrNotFound
		}
		return Checkpoint{}, err
	}
	return c, nil
}

func (r *CheckpointRepo) Upsert(ctx context.Context, c Checkpoint) error {
	query := `
		INSERT INTO sync_checkpoints (connection_id, table_name, last_batch_completed, last_pk_value, started_at, status)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(connection_id, table_name) DO UPDATE SET
			last_batch_completed = excluded.last_batch_completed,
			last_pk_value = excluded.last_pk_value,
			status = excluded.status,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := r.db.ExecContext(ctx, query, c.ConnectionID, c.TableName, c.LastBatchCompleted, c.LastPKValue, c.StartedAt, c.Status)
	return err
}

func (r *CheckpointRepo) MarkInterrupted(ctx context.Context, connID int64, table string) error {
	query := `UPDATE sync_checkpoints SET status = 'interrupted', updated_at = CURRENT_TIMESTAMP WHERE connection_id = ? AND table_name = ?`
	_, err := r.db.ExecContext(ctx, query, connID, table)
	return err
}

func (r *CheckpointRepo) MarkCompleted(ctx context.Context, connID int64, table string) error {
	query := `UPDATE sync_checkpoints SET status = 'completed', updated_at = CURRENT_TIMESTAMP WHERE connection_id = ? AND table_name = ?`
	_, err := r.db.ExecContext(ctx, query, connID, table)
	return err
}

func (r *CheckpointRepo) MarkFailed(ctx context.Context, connID int64, table string) error {
	query := `UPDATE sync_checkpoints SET status = 'failed', updated_at = CURRENT_TIMESTAMP WHERE connection_id = ? AND table_name = ?`
	_, err := r.db.ExecContext(ctx, query, connID, table)
	return err
}

func (r *CheckpointRepo) ListActive(ctx context.Context) ([]Checkpoint, error) {
	query := `
		SELECT id, connection_id, table_name, last_batch_completed, last_pk_value, started_at, updated_at, status
		FROM sync_checkpoints
		WHERE status IN ('running', 'interrupted')
		ORDER BY updated_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checkpoints []Checkpoint
	for rows.Next() {
		var c Checkpoint
		err := rows.Scan(
			&c.ID, &c.ConnectionID, &c.TableName, &c.LastBatchCompleted, &c.LastPKValue, &c.StartedAt, &c.UpdatedAt, &c.Status,
		)
		if err != nil {
			return nil, err
		}
		checkpoints = append(checkpoints, c)
	}
	return checkpoints, nil
}

func (r *CheckpointRepo) Delete(ctx context.Context, connID int64, table string) error {
	query := `DELETE FROM sync_checkpoints WHERE connection_id = ? AND table_name = ?`
	_, err := r.db.ExecContext(ctx, query, connID, table)
	return err
}

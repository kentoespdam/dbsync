package storage

import (
	"context"
	"database/sql"
	"time"
)

type HistoryRecord struct {
	ID              int64
	ConnectionID    int64
	TableName       string
	StartedAt       time.Time
	FinishedAt      sql.NullTime
	DurationSeconds sql.NullInt64
	TotalRows       sql.NullInt64
	SuccessRows     sql.NullInt64
	FailedRows      sql.NullInt64
	Status          string // 'running' | 'completed' | 'failed' | 'interrupted'
	ErrorSummary    sql.NullString
}

type HistoryRepo struct {
	db *sql.DB
}

func (d *DB) History() *HistoryRepo {
	return &HistoryRepo{db: d.db}
}

func (r *HistoryRepo) Begin(ctx context.Context, connID int64, table string) (int64, error) {
	query := `
		INSERT INTO sync_history (connection_id, table_name, started_at, status)
		VALUES (?, ?, ?, 'running')
	`
	res, err := r.db.ExecContext(ctx, query, connID, table, time.Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *HistoryRepo) Finish(ctx context.Context, id int64, h HistoryRecord) error {
	query := `
		UPDATE sync_history SET
			finished_at = ?,
			duration_seconds = ?,
			total_rows = ?,
			success_rows = ?,
			failed_rows = ?,
			status = ?,
			error_summary = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		h.FinishedAt,
		h.DurationSeconds,
		h.TotalRows,
		h.SuccessRows,
		h.FailedRows,
		h.Status,
		h.ErrorSummary,
		id,
	)
	return err
}

func (r *HistoryRepo) ListAll(ctx context.Context, limit int) ([]HistoryRecord, error) {
	query := `
		SELECT id, connection_id, table_name, started_at, finished_at, duration_seconds, total_rows, success_rows, failed_rows, status, error_summary
		FROM sync_history
		ORDER BY started_at DESC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []HistoryRecord
	for rows.Next() {
		var h HistoryRecord
		err := rows.Scan(
			&h.ID, &h.ConnectionID, &h.TableName, &h.StartedAt, &h.FinishedAt, &h.DurationSeconds, &h.TotalRows, &h.SuccessRows, &h.FailedRows, &h.Status, &h.ErrorSummary,
		)
		if err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}

func (r *HistoryRepo) ListByConnection(ctx context.Context, connID int64, limit int) ([]HistoryRecord, error) {
	query := `
		SELECT id, connection_id, table_name, started_at, finished_at, duration_seconds, total_rows, success_rows, failed_rows, status, error_summary
		FROM sync_history
		WHERE connection_id = ?
		ORDER BY started_at DESC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, connID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []HistoryRecord
	for rows.Next() {
		var h HistoryRecord
		err := rows.Scan(
			&h.ID, &h.ConnectionID, &h.TableName, &h.StartedAt, &h.FinishedAt, &h.DurationSeconds, &h.TotalRows, &h.SuccessRows, &h.FailedRows, &h.Status, &h.ErrorSummary,
		)
		if err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}

func (r *HistoryRepo) List(ctx context.Context, connID int64, table string, limit int) ([]HistoryRecord, error) {
	query := `
		SELECT id, connection_id, table_name, started_at, finished_at, duration_seconds, total_rows, success_rows, failed_rows, status, error_summary
		FROM sync_history
		WHERE connection_id = ? AND table_name = ?
		ORDER BY started_at DESC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, connID, table, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []HistoryRecord
	for rows.Next() {
		var h HistoryRecord
		err := rows.Scan(
			&h.ID, &h.ConnectionID, &h.TableName, &h.StartedAt, &h.FinishedAt, &h.DurationSeconds, &h.TotalRows, &h.SuccessRows, &h.FailedRows, &h.Status, &h.ErrorSummary,
		)
		if err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}

func (r *HistoryRepo) Latest(ctx context.Context, connID int64, table string) (HistoryRecord, error) {
	query := `
		SELECT id, connection_id, table_name, started_at, finished_at, duration_seconds, total_rows, success_rows, failed_rows, status, error_summary
		FROM sync_history
		WHERE connection_id = ? AND table_name = ?
		ORDER BY started_at DESC
		LIMIT 1
	`
	var h HistoryRecord
	err := r.db.QueryRowContext(ctx, query, connID, table).Scan(
		&h.ID, &h.ConnectionID, &h.TableName, &h.StartedAt, &h.FinishedAt, &h.DurationSeconds, &h.TotalRows, &h.SuccessRows, &h.FailedRows, &h.Status, &h.ErrorSummary,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return HistoryRecord{}, ErrNotFound
		}
		return HistoryRecord{}, err
	}
	return h, nil
}

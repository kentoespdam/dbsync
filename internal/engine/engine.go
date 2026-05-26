package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/logger"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
)

type cryptoDecryptor interface {
	Decrypt(b64 string, key []byte) ([]byte, error)
}

type defaultDecryptor struct{}

func (d defaultDecryptor) Decrypt(b64 string, key []byte) ([]byte, error) {
	return crypto.Decrypt(b64, key)
}

type Options struct {
	ConnectionID int64
	TableName    string
	BatchSize    int // default 1000
	DryRun       bool
}

type Event interface{ isEvent() }

type ProgressEvent struct {
	Batch    int
	RowsDone int
}

func (e ProgressEvent) isEvent() {}

type BatchErrorEvent struct {
	Batch int
	Err   error
}

func (e BatchErrorEvent) isEvent() {}

type RowErrorEvent struct {
	Batch int
	PK    any
	Err   error
}

func (e RowErrorEvent) isEvent() {}

type DoneEvent struct {
	TotalRows int
	Status    string // 'completed', 'failed', 'interrupted'
	Err       error  // nil if success
}

func (e DoneEvent) isEvent() {}

type Engine struct {
	store  *storage.DB
	crypto cryptoDecryptor
	key    []byte
	logger *logger.Logger
}

func New(store *storage.DB, key []byte, log *logger.Logger) *Engine {
	return &Engine{
		store:  store,
		crypto: defaultDecryptor{},
		key:    key,
		logger: log,
	}
}

func (e *Engine) Run(ctx context.Context, opts Options) (<-chan Event, error) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 1000
	}

	events := make(chan Event, 16)

	go func() {
		defer close(events)
		startTime := time.Now()
		totalRows := 0
		successRows := 0
		failedRows := 0
		status := "completed"
		var finalErr error

		// 1. Load connection
		conn, err := e.store.Connections().GetByID(ctx, opts.ConnectionID)
		if err != nil {
			events <- DoneEvent{Status: "failed", Err: fmt.Errorf("load connection: %w", err)}
			return
		}

		// 2. Decrypt passwords
		srcPass, err := e.crypto.Decrypt(conn.SourcePassword, e.key)
		if err != nil {
			events <- DoneEvent{Status: "failed", Err: fmt.Errorf("decrypt source password: %w", err)}
			return
		}
		destPass, err := e.crypto.Decrypt(conn.DestPassword, e.key)
		if err != nil {
			events <- DoneEvent{Status: "failed", Err: fmt.Errorf("decrypt dest password: %w", err)}
			return
		}

		// 3. Connect source+dest
		srcPool, err := mysql.Open(mysql.Config{
			Host:     conn.SourceHost,
			Port:     conn.SourcePort,
			User:     conn.SourceUser,
			Password: string(srcPass),
			DBName:   conn.SourceDB,
		})
		if err != nil {
			events <- DoneEvent{Status: "failed", Err: fmt.Errorf("open source pool: %w", err)}
			return
		}
		defer srcPool.Close()

		destPool, err := mysql.Open(mysql.Config{
			Host:     conn.DestHost,
			Port:     conn.DestPort,
			User:     conn.DestUser,
			Password: string(destPass),
			DBName:   conn.DestDB,
		})
		if err != nil {
			events <- DoneEvent{Status: "failed", Err: fmt.Errorf("open dest pool: %w", err)}
			return
		}
		defer destPool.Close()

		// 4. Load mappings
		mappings, err := e.store.Mappings().ListByTable(ctx, opts.ConnectionID, opts.TableName)
		if err != nil {
			events <- DoneEvent{Status: "failed", Err: fmt.Errorf("load mappings: %w", err)}
			return
		}
		if len(mappings) == 0 {
			events <- DoneEvent{Status: "failed", Err: fmt.Errorf("no mappings found for table %s", opts.TableName)}
			return
		}

		// 5. Detect PK and describe columns (for deserialization)
		pkCols, err := mysql.DetectPK(ctx, srcPool.DB(), conn.SourceDB, opts.TableName)
		if err != nil {
			events <- DoneEvent{Status: "failed", Err: fmt.Errorf("detect PK: %w", err)}
			return
		}
		if len(pkCols) == 0 {
			events <- DoneEvent{Status: "failed", Err: fmt.Errorf("no primary key found for table %s", opts.TableName)}
			return
		}

		allCols, err := mysql.DescribeColumns(ctx, srcPool.DB(), conn.SourceDB, opts.TableName)
		if err != nil {
			events <- DoneEvent{Status: "failed", Err: fmt.Errorf("describe columns: %w", err)}
			return
		}
		colMap := make(map[string]mysql.Column)
		for _, col := range allCols {
			colMap[col.Name] = col
		}

		// 6. Resume check
		var lastPK []any
		var lastBatchN int
		cp, err := e.store.Checkpoints().Get(ctx, opts.ConnectionID, opts.TableName)
		if err == nil {
			if cp.Status == "completed" {
				// Start fresh
			} else {
				// Resume from last PK
				lastPK, err = deserializePK(cp.LastPKValue, pkCols, colMap)
				if err != nil {
					e.logger.BatchError(0, err, "deserialize checkpoint PK")
					// fallback to fresh start if checkpoint corrupted? PRD says CLI auto-resume.
				}
				lastBatchN = cp.LastBatchCompleted
			}
		} else if !errors.Is(err, storage.ErrNotFound) {
			events <- DoneEvent{Status: "failed", Err: fmt.Errorf("check checkpoint: %w", err)}
			return
		}

		// 7. Begin History (if not dry run)
		var historyID int64
		if !opts.DryRun {
			historyID, err = e.store.History().Begin(ctx, opts.ConnectionID, opts.TableName)
			if err != nil {
				events <- DoneEvent{Status: "failed", Err: fmt.Errorf("begin history: %w", err)}
				return
			}
			// Update checkpoint to running
			err = e.store.Checkpoints().Upsert(ctx, storage.Checkpoint{
				ConnectionID:       opts.ConnectionID,
				TableName:          opts.TableName,
				LastBatchCompleted: lastBatchN,
				LastPKValue:        serializePK(lastPK),
				StartedAt:          time.Now(),
				Status:             "running",
			})
			if err != nil {
				events <- DoneEvent{Status: "failed", Err: fmt.Errorf("upsert checkpoint: %w", err)}
				return
			}
		}

		// 8. Loop batches
		resolvedMappings := Resolve(mappings)
		batchN := lastBatchN

		for {
			select {
			case <-ctx.Done():
				status = "interrupted"
				finalErr = ctx.Err()
				goto finalize
			default:
			}

			batchN++
			rows, nextPK, err := mysql.SelectBatch(ctx, srcPool.DB(), conn.SourceDB, opts.TableName, pkCols, lastPK, opts.BatchSize)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					status = "interrupted"
					finalErr = err
				} else {
					status = "failed"
					finalErr = fmt.Errorf("select batch %d: %w", batchN, err)
					events <- BatchErrorEvent{Batch: batchN, Err: finalErr}
					e.logger.BatchError(batchN, finalErr, "SELECT * FROM ...")
				}
				goto finalize
			}

			if len(rows) == 0 {
				break // Done
			}

			if opts.DryRun {
				totalRows += len(rows)
				successRows += len(rows)
				events <- ProgressEvent{Batch: batchN, RowsDone: totalRows}
				lastPK = nextPK
				continue
			}

			// Upsert batch
			tx, err := destPool.DB().BeginTx(ctx, nil)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					status = "interrupted"
					finalErr = err
				} else {
					status = "failed"
					finalErr = fmt.Errorf("begin dest tx: %w", err)
				}
				goto finalize
			}

			if _, err := tx.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
				tx.Rollback()
				if errors.Is(err, context.Canceled) {
					status = "interrupted"
					finalErr = err
				} else {
					status = "failed"
					finalErr = fmt.Errorf("disable FK checks: %w", err)
				}
				goto finalize
			}

			count, err := mysql.Upsert(ctx, tx, conn.DestDB, opts.TableName, pkCols, resolvedMappings, rows)
			if err != nil {
				tx.Rollback()
				if errors.Is(err, context.Canceled) {
					status = "interrupted"
					finalErr = err
				} else {
					status = "failed"
					finalErr = fmt.Errorf("upsert batch %d: %w", batchN, err)
					events <- BatchErrorEvent{Batch: batchN, Err: finalErr}
					e.logger.BatchError(batchN, finalErr, "UPSERT")
				}
				goto finalize
			}

			if err := tx.Commit(); err != nil {
				if errors.Is(err, context.Canceled) {
					status = "interrupted"
					finalErr = err
				} else {
					status = "failed"
					finalErr = fmt.Errorf("commit batch %d: %w", batchN, err)
				}
				goto finalize
			}

			totalRows += count
			successRows += count
			lastPK = nextPK

			// Update checkpoint atomic
			err = e.store.Checkpoints().Upsert(ctx, storage.Checkpoint{
				ConnectionID:       opts.ConnectionID,
				TableName:          opts.TableName,
				LastBatchCompleted: batchN,
				LastPKValue:        serializePK(lastPK),
				Status:             "running",
				StartedAt:          time.Now(), // This might be better as the original started at, but Upsert update SETs it if row exists. 
				// Wait, my Upsert in checkpoints.go does:
				// ON CONFLICT(connection_id, table_name) DO UPDATE SET
				// last_batch_completed = excluded.last_batch_completed,
				// last_pk_value = excluded.last_pk_value,
				// status = excluded.status,
				// updated_at = CURRENT_TIMESTAMP
				// So StartedAt is only used for INSERT.
			})
			if err != nil {
				status = "failed"
				finalErr = fmt.Errorf("update checkpoint: %w", err)
				goto finalize
			}

			events <- ProgressEvent{Batch: batchN, RowsDone: totalRows}
		}

	finalize:
		if !opts.DryRun {
			// Update History
			finalCtx := context.Background() // Use fresh context for finalization to ensure it runs even if ctx is canceled
			h := storage.HistoryRecord{
				FinishedAt:      sql.NullTime{Time: time.Now(), Valid: true},
				DurationSeconds: sql.NullInt64{Int64: int64(time.Since(startTime).Seconds()), Valid: true},
				TotalRows:       sql.NullInt64{Int64: int64(totalRows), Valid: true},
				SuccessRows:     sql.NullInt64{Int64: int64(successRows), Valid: true},
				FailedRows:      sql.NullInt64{Int64: int64(failedRows), Valid: true},
				Status:          status,
			}
			if finalErr != nil {
				h.ErrorSummary = sql.NullString{String: finalErr.Error(), Valid: true}
			}

			// Update Checkpoint status
			switch status {
			case "completed":
				e.store.Checkpoints().MarkCompleted(finalCtx, opts.ConnectionID, opts.TableName)
			case "failed":
				e.store.Checkpoints().MarkFailed(finalCtx, opts.ConnectionID, opts.TableName)
			case "interrupted":
				e.store.Checkpoints().MarkInterrupted(finalCtx, opts.ConnectionID, opts.TableName)
			}

			e.store.History().Finish(finalCtx, historyID, h)
		}

		events <- DoneEvent{TotalRows: totalRows, Status: status, Err: finalErr}
	}()

	return events, nil
}

func serializePK(values []any) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return fmt.Sprint(values[0])
	}
	b, _ := json.Marshal(values)
	return string(b)
}

func deserializePK(s string, pkCols []string, colMap map[string]mysql.Column) ([]any, error) {
	if s == "" {
		return nil, nil
	}

	if len(pkCols) == 1 {
		return []any{s}, nil // Simple string conversion for now, mysql driver handles it
	}

	var vals []any
	if err := json.Unmarshal([]byte(s), &vals); err != nil {
		return nil, err
	}
	return vals, nil
}

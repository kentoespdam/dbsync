package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/kentoespdam/dbsync/internal/mysql"
	"github.com/kentoespdam/dbsync/internal/storage"
)

func (s *syncSession) syncLoop(ctx context.Context) {
	batchN := s.lastBatchN
	for {
		select {
		case <-ctx.Done():
			s.status, s.finalErr = "interrupted", ctx.Err()
			return
		default:
		}

		batchN++
		rows, nextPK, err := mysql.SelectBatch(ctx, s.srcPool.DB(), s.srcPool.Config().DBName, s.opts.TableName, s.sourceCols, s.pkCols, s.lastPK, s.opts.BatchSize)
		if err != nil {
			s.handleBatchError(batchN, err)
			return
		}
		if len(rows) == 0 {
			break
		}

		if s.opts.DryRun {
			s.totalRows += len(rows)
			s.successRows += len(rows)
			s.lastPK = nextPK
			s.events <- ProgressEvent{Batch: batchN, RowsDone: s.totalRows}
			continue
		}

		count, err := s.upsertBatch(ctx, rows)
		if err != nil {
			s.handleBatchError(batchN, err)
			return
		}

		s.totalRows += count
		s.successRows += count
		s.lastPK = nextPK
		s.lastBatchN = batchN
		
		if err := s.updateCheckpoint(ctx); err != nil {
			s.status, s.finalErr = "failed", err
			return
		}
		s.events <- ProgressEvent{Batch: batchN, RowsDone: s.totalRows}
	}
}

func (s *syncSession) upsertBatch(ctx context.Context, rows []mysql.Row) (int, error) {
	conn, err := s.destPool.DB().Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("get dest conn: %w", err)
	}
	defer conn.Close()

	// Safety: Ensure FK checks are reset even if cancellation occurs
	defer conn.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS=1")

	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return 0, fmt.Errorf("disable FK checks: %w", err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin dest tx: %w", err)
	}

	count, err := mysql.Upsert(ctx, tx, s.destPool.Config().DBName, s.opts.TableName, s.pkCols, s.mappings, rows)
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("upsert: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return count, nil
}

func (s *syncSession) handleBatchError(batchN int, err error) {
	if errors.Is(err, context.Canceled) {
		s.status, s.finalErr = "interrupted", err
	} else {
		s.status, s.finalErr = "failed", err
		s.events <- BatchErrorEvent{Batch: batchN, Err: err}
		s.e.logger.BatchError(batchN, err, "batch execution")
	}
}

func (s *syncSession) updateCheckpoint(ctx context.Context) error {
	return s.e.store.Checkpoints().Upsert(ctx, storage.Checkpoint{
		ConnectionID:       s.opts.ConnectionID,
		TableName:          s.opts.TableName,
		LastBatchCompleted: s.lastBatchN,
		LastPKValue:        serializePK(s.lastPK),
		Status:             "running",
		StartedAt:          s.startTime,
	})
}

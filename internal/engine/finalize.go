package engine

import (
	"context"
	"database/sql"
	"time"

	"github.com/user/dbsync/internal/storage"
)

func (s *syncSession) finalize() {
	if s.opts.DryRun {
		s.events <- DoneEvent{TotalRows: s.totalRows, Status: s.status, Err: s.finalErr}
		return
	}

	// Use Background context for finalization to ensure it runs even if parent is canceled
	ctx := context.Background()

	h := storage.HistoryRecord{
		FinishedAt:      sql.NullTime{Time: time.Now(), Valid: true},
		DurationSeconds: sql.NullInt64{Int64: int64(time.Since(s.startTime).Seconds()), Valid: true},
		TotalRows:       sql.NullInt64{Int64: int64(s.totalRows), Valid: true},
		SuccessRows:     sql.NullInt64{Int64: int64(s.successRows), Valid: true},
		FailedRows:      sql.NullInt64{Int64: int64(s.failedRows), Valid: true},
		Status:          s.status,
	}
	if s.finalErr != nil {
		h.ErrorSummary = sql.NullString{String: s.finalErr.Error(), Valid: true}
	}

	switch s.status {
	case "completed":
		s.e.store.Checkpoints().MarkCompleted(ctx, s.opts.ConnectionID, s.opts.TableName)
	case "failed":
		s.e.store.Checkpoints().MarkFailed(ctx, s.opts.ConnectionID, s.opts.TableName)
	case "interrupted":
		s.e.store.Checkpoints().MarkInterrupted(ctx, s.opts.ConnectionID, s.opts.TableName)
	}

	s.e.store.History().Finish(ctx, s.historyID, h)
	s.events <- DoneEvent{TotalRows: s.totalRows, Status: s.status, Err: s.finalErr}
}

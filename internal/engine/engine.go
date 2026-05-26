package engine

import (
	"context"
	"fmt"

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
	Err       error // nil if success
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
		totalRows := 0

		// 1. Load connection
		conn, err := e.store.Connections().GetByID(ctx, opts.ConnectionID)
		if err != nil {
			events <- DoneEvent{Err: fmt.Errorf("load connection: %w", err)}
			return
		}

		// 2. Decrypt passwords
		srcPass, err := e.crypto.Decrypt(conn.SourcePassword, e.key)
		if err != nil {
			events <- DoneEvent{Err: fmt.Errorf("decrypt source password: %w", err)}
			return
		}
		destPass, err := e.crypto.Decrypt(conn.DestPassword, e.key)
		if err != nil {
			events <- DoneEvent{Err: fmt.Errorf("decrypt dest password: %w", err)}
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
			events <- DoneEvent{Err: fmt.Errorf("open source pool: %w", err)}
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
			events <- DoneEvent{Err: fmt.Errorf("open dest pool: %w", err)}
			return
		}
		defer destPool.Close()

		// 4. Load mappings
		mappings, err := e.store.Mappings().ListByTable(ctx, opts.ConnectionID, opts.TableName)
		if err != nil {
			events <- DoneEvent{Err: fmt.Errorf("load mappings: %w", err)}
			return
		}
		if len(mappings) == 0 {
			events <- DoneEvent{Err: fmt.Errorf("no mappings found for table %s", opts.TableName)}
			return
		}

		// 5. Detect PK
		pkCols, err := mysql.DetectPK(ctx, srcPool.DB(), conn.SourceDB, opts.TableName)
		if err != nil {
			events <- DoneEvent{Err: fmt.Errorf("detect PK: %w", err)}
			return
		}
		if len(pkCols) == 0 {
			events <- DoneEvent{Err: fmt.Errorf("no primary key found for table %s", opts.TableName)}
			return
		}

		// 6. Resolve mappings
		resolvedMappings := Resolve(mappings)

		// 7. Loop 1 iterasi saja (Issue 004 MVP)
		batchNum := 1
		
		// Start transaction at dest
		tx, err := destPool.DB().BeginTx(ctx, nil)
		if err != nil {
			events <- DoneEvent{Err: fmt.Errorf("begin dest tx: %w", err)}
			return
		}
		defer tx.Rollback()

		// Disable FK checks for this session
		if _, err := tx.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
			events <- DoneEvent{Err: fmt.Errorf("disable FK checks: %w", err)}
			return
		}

		// SelectBatch (first batch, lastPK = nil)
		rows, _, err := mysql.SelectBatch(ctx, srcPool.DB(), conn.SourceDB, opts.TableName, pkCols, nil, opts.BatchSize)
		if err != nil {
			events <- BatchErrorEvent{Batch: batchNum, Err: err}
			e.logger.BatchError(batchNum, err, "SELECT * FROM ...")
			events <- DoneEvent{Err: err}
			return
		}

		if len(rows) > 0 {
			// Upsert
			count, err := mysql.Upsert(ctx, tx, conn.DestDB, opts.TableName, pkCols, resolvedMappings, rows)
			if err != nil {
				events <- BatchErrorEvent{Batch: batchNum, Err: err}
				e.logger.BatchError(batchNum, err, "INSERT INTO ... ON DUPLICATE KEY UPDATE")
				events <- DoneEvent{Err: err}
				return
			}

			if err := tx.Commit(); err != nil {
				events <- DoneEvent{Err: fmt.Errorf("commit dest tx: %w", err)}
				return
			}
			
			totalRows += count
			events <- ProgressEvent{Batch: batchNum, RowsDone: totalRows}
		} else {
			_ = tx.Commit() // Empty batch, just commit/close
		}

		events <- DoneEvent{TotalRows: totalRows}
	}()

	return events, nil
}

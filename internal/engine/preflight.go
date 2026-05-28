package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kentoespdam/dbsync/internal/mysql"
	"github.com/kentoespdam/dbsync/internal/storage"
)

func (s *syncSession) preflight(ctx context.Context) error {
	conn, err := s.e.store.Connections().GetByID(ctx, s.opts.ConnectionID)
	if err != nil {
		return fmt.Errorf("load connection: %w", err)
	}

	srcPass, err := s.e.crypto.Decrypt(conn.SourcePassword, s.e.key)
	if err != nil {
		return fmt.Errorf("decrypt source pass: %w", err)
	}
	destPass, err := s.e.crypto.Decrypt(conn.DestPassword, s.e.key)
	if err != nil {
		return fmt.Errorf("decrypt dest pass: %w", err)
	}

	s.srcPool, err = mysql.Open(mysql.Config{
		Host: conn.SourceHost, Port: conn.SourcePort, User: conn.SourceUser,
		Password: string(srcPass), DBName: conn.SourceDB,
	})
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}

	s.destPool, err = mysql.Open(mysql.Config{
		Host: conn.DestHost, Port: conn.DestPort, User: conn.DestUser,
		Password: string(destPass), DBName: conn.DestDB,
	})
	if err != nil {
		return fmt.Errorf("open dest: %w", err)
	}

	mappings, err := s.e.store.Mappings().ListByTable(ctx, s.opts.ConnectionID, s.opts.TableName)
	if err != nil || len(mappings) == 0 {
		return fmt.Errorf("load mappings: %w (count: %d)", err, len(mappings))
	}
	s.mappings = Resolve(mappings)

	s.pkCols, err = mysql.DetectPK(ctx, s.srcPool.DB(), conn.SourceDB, s.opts.TableName)
	if err != nil || len(s.pkCols) == 0 {
		return fmt.Errorf("detect PK: %w", err)
	}

	// Calculate source columns to fetch
	colSet := make(map[string]bool)
	for _, pk := range s.pkCols {
		colSet[pk] = true
	}
	for _, m := range mappings {
		if m.SourceColumn.Valid {
			colSet[m.SourceColumn.String] = true
		}
	}
	s.sourceCols = make([]string, 0, len(colSet))
	for col := range colSet {
		s.sourceCols = append(s.sourceCols, col)
	}

	allCols, err := mysql.DescribeColumns(ctx, s.srcPool.DB(), conn.SourceDB, s.opts.TableName)
	if err != nil {
		return fmt.Errorf("describe cols: %w", err)
	}
	s.colMap = make(map[string]mysql.Column)
	for _, col := range allCols {
		s.colMap[col.Name] = col
	}

	return s.resumeCheck(ctx)
}

func (s *syncSession) resumeCheck(ctx context.Context) error {
	cp, err := s.e.store.Checkpoints().Get(ctx, s.opts.ConnectionID, s.opts.TableName)
	if err == nil && cp.Status != "completed" {
		s.lastPK, err = deserializePK(cp.LastPKValue, s.pkCols, s.colMap)
		if err != nil {
			s.e.logger.BatchError(0, err, "deserialize checkpoint PK")
		}
		s.lastBatchN = cp.LastBatchCompleted
	} else if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("check checkpoint: %w", err)
	}

	if !s.opts.DryRun {
		s.historyID, err = s.e.store.History().Begin(ctx, s.opts.ConnectionID, s.opts.TableName)
		if err != nil {
			return fmt.Errorf("begin history: %w", err)
		}
		return s.e.store.Checkpoints().Upsert(ctx, storage.Checkpoint{
			ConnectionID: s.opts.ConnectionID, TableName: s.opts.TableName,
			LastBatchCompleted: s.lastBatchN, LastPKValue: serializePK(s.lastPK),
			StartedAt: time.Now(), Status: "running",
		})
	}
	return nil
}

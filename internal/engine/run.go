package engine

import (
	"context"
	"time"

	"github.com/kentoespdam/dbsync/internal/mysql"
)

type syncSession struct {
	e          *Engine
	opts       Options
	events     chan Event
	startTime  time.Time
	totalRows  int
	successRows int
	failedRows  int
	status     string
	finalErr   error
	
	srcPool  *mysql.Pool
	destPool *mysql.Pool
	pkCols   []string
	sourceCols []string
	colMap   map[string]mysql.Column
	mappings []mysql.ResolvedMapping
	
	lastPK     []any
	lastBatchN int
	historyID  int64
}

func (e *Engine) Run(ctx context.Context, opts Options) (<-chan Event, error) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 1000
	}
	events := make(chan Event, 16)
	s := &syncSession{
		e:      e,
		opts:   opts,
		events: events,
		status: "completed",
	}
	go s.run(ctx)
	return events, nil
}

func (s *syncSession) run(ctx context.Context) {
	defer close(s.events)
	s.startTime = time.Now()

	if err := s.preflight(ctx); err != nil {
		s.events <- DoneEvent{Status: "failed", Err: err}
		return
	}
	defer s.cleanup()

	s.syncLoop(ctx)
	s.finalize()
}

func (s *syncSession) cleanup() {
	if s.srcPool != nil {
		s.srcPool.Close()
	}
	if s.destPool != nil {
		s.destPool.Close()
	}
}

package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kentoespdam/dbsync/internal/paths"
)

// Logger writes structured log entries to a JSONL file.
type Logger struct {
	file *os.File
	enc  *json.Encoder
	mu   sync.Mutex
}

// Entry represents a single log line in the JSONL file.
type Entry struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"` // "row_error" | "batch_error"
	Batch       int       `json:"batch"`
	RowPK       any       `json:"row_pk,omitempty"`
	Error       string    `json:"error"`
	SQLTemplate string    `json:"sql_template,omitempty"`
}

// New creates a new Logger that writes to a timestamped file in <exeDir>/logs/.
func New(connectionName, tableName string) (*Logger, error) {
	logDir, err := paths.LogsDir()
	if err != nil {
		return nil, fmt.Errorf("resolve logs dir: %w", err)
	}

	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	ts := time.Now().Format("20060102-150405")
	fileName := fmt.Sprintf("sync-%s-%s-%s.jsonl", ts, connectionName, tableName)
	filePath := filepath.Join(logDir, fileName)

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	return &Logger{
		file: f,
		enc:  json.NewEncoder(f),
	}, nil
}

// Path returns the absolute path to the log file.
func (l *Logger) Path() string {
	return l.file.Name()
}

// RowError logs an error that occurred for a specific row.
func (l *Logger) RowError(batch int, pk any, err error, sqlTemplate string) {
	l.log(Entry{
		Level:       "row_error",
		Batch:       batch,
		RowPK:       pk,
		Error:       err.Error(),
		SQLTemplate: sqlTemplate,
	})
}

// BatchError logs an error that occurred for an entire batch.
func (l *Logger) BatchError(batch int, err error, sqlTemplate string) {
	l.log(Entry{
		Level:       "batch_error",
		Batch:       batch,
		Error:       err.Error(),
		SQLTemplate: sqlTemplate,
	})
}

func (l *Logger) log(e Entry) {
	e.Timestamp = time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.enc.Encode(e)
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	if l.file == nil {
		return nil
	}
	return l.file.Close()
}



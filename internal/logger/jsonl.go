package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
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

// New creates a new Logger that writes to a timestamped file in ~/.local/share/dbsync/logs/.
func New(connectionName, tableName string) (*Logger, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	logDir := filepath.Join(home, ".local", "share", "dbsync", "logs")
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

// RowError logs an error that occurred for a specific row.
func (l *Logger) RowError(batch int, pk any, err error, sqlTemplate string) {
	l.log(Entry{
		Level:       "row_error",
		Batch:       batch,
		RowPK:       pk,
		Error:       SanitizeError(err),
		SQLTemplate: sqlTemplate,
	})
}

// BatchError logs an error that occurred for an entire batch.
func (l *Logger) BatchError(batch int, err error, sqlTemplate string) {
	l.log(Entry{
		Level:       "batch_error",
		Batch:       batch,
		Error:       SanitizeError(err),
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

var (
	// regex to find 'values' inside single or double quotes
	quoteRegex = regexp.MustCompile(`'[^']*'|"[^"]*"`)
)

// SanitizeError redacts potential row values from error messages.
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	
	// MySQL error messages often contain sensitive data in quotes.
	// Example: Duplicate entry 'john@example.com' for key 'users.email'
	// We want to redact the 'john@example.com' but maybe keep the key name?
	// The requirement says: "jika error mengandung pattern near '...', strip nilai di dalam quotes"
	
	// Simplest approach: redact all quoted strings in messages that look like data errors.
	// But let's follow the "near" or "entry" pattern more specifically if possible, 
	// or just redact all quotes to be safe.
	
	return quoteRegex.ReplaceAllStringFunc(msg, func(s string) string {
		// If it's a very short quoted string (like a column name or key name), maybe keep it?
		// No, let's just redact all to be safe and simple as per "Simple > clever".
		return "'[REDACTED]'"
	})
}

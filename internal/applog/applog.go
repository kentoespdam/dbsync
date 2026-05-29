package applog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kentoespdam/dbsync/internal/paths"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Init must be called once, paling awal di main().
// Sets slog.Default to a text handler writing to <exeDir>/logs/dbsync.log
// (rotated by lumberjack), with AddSource:true.
// Returns an io.Closer to flush/close lumberjack on shutdown.
func Init() (io.Closer, error) {
	logDir, err := paths.LogsDir()
	if err != nil {
		return nil, fmt.Errorf("resolve logs dir: %w", err)
	}

	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}

	logPath := filepath.Join(logDir, "dbsync.log")
	
	// Fail-fast: check if we can open/create the log file.
	// Lumberjack opens lazily, so we do a manual check.
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	f.Close()

	return initWithPath(logPath)
}

func initWithPath(path string) (io.Closer, error) {
	lj := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    10, // MB
		MaxBackups: 5,
		MaxAge:     30, // days
		Compress:   true,
		LocalTime:  true,
	}

	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     resolveLevel(),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				source, ok := a.Value.Any().(*slog.Source)
				if ok {
					// trim source path agar relative ke module root (cari dbsync/ dan potong)
					if idx := strings.Index(source.File, "dbsync/"); idx != -1 {
						source.File = source.File[idx+len("dbsync/"):]
					}
				}
			}
			return a
		},
	}

	handler := slog.NewTextHandler(lj, opts)

	slog.SetDefault(slog.New(handler))

	return lj, nil
}

func resolveLevel() slog.Level {
	switch strings.ToLower(os.Getenv("DBSYNC_LOG_LEVEL")) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// TestSilent routes slog.Default to io.Discard for the duration of t.
// Opt-out via DBSYNC_TEST_LOG=1.
func TestSilent(t *testing.T) {
	if os.Getenv("DBSYNC_TEST_LOG") == "1" {
		return
	}

	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() {
		slog.SetDefault(old)
	})
}

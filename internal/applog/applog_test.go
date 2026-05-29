package applog

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveLevel(t *testing.T) {
	tests := []struct {
		env  string
		want slog.Level
	}{
		{"", slog.LevelInfo},
		{"DEBUG", slog.LevelDebug},
		{"debug", slog.LevelDebug},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"INVALID", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			t.Setenv("DBSYNC_LOG_LEVEL", tt.env)
			if got := resolveLevel(); got != tt.want {
				t.Errorf("resolveLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitWithPath(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	closer, err := initWithPath(logFile)
	if err != nil {
		t.Fatalf("initWithPath failed: %v", err)
	}
	defer closer.Close()

	slog.Error("test message", "err", errors.New("secret 'password'"))

	// Close to flush
	closer.Close()

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	sContent := string(content)
	if !strings.Contains(sContent, "test message") {
		t.Error("log file does not contain test message")
	}
	if !strings.Contains(sContent, "secret 'password'") {
		t.Error("log file does not contain raw error")
	}
}

func TestTestSilent(t *testing.T) {
	oldDefault := slog.Default()

	t.Run("silent", func(t *testing.T) {
		TestSilent(t)
		if slog.Default() == oldDefault {
			t.Error("TestSilent did not change slog.Default")
		}
	})

	if slog.Default() != oldDefault {
		t.Error("slog.Default was not restored after TestSilent")
	}
}

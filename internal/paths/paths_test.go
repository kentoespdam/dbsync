package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogsDir(t *testing.T) {
	dir, err := LogsDir()
	if err != nil {
		t.Fatalf("LogsDir failed: %v", err)
	}

	if dir == "" {
		t.Error("LogsDir returned empty string")
	}

	if !strings.HasSuffix(dir, "logs") {
		t.Errorf("LogsDir path %q does not end with 'logs'", dir)
	}

	// Verify it's absolute
	if !filepath.IsAbs(dir) {
		t.Errorf("LogsDir path %q is not absolute", dir)
	}

	// In test context, it should be relative to the test binary
	exe, _ := os.Executable()
	expected := filepath.Join(filepath.Dir(exe), "logs")
	
	// Note: os.Executable in 'go test' points to a temp binary.
	// We just want to make sure it matches the logic.
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

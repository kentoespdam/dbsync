package applog

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping rotation test in short mode")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "rotate.log")

	closer, err := initWithPath(logFile)
	if err != nil {
		t.Fatalf("initWithPath failed: %v", err)
	}
	defer closer.Close()

	// Write ~11MB of data
	// Each log line is ~100 bytes. 110,000 lines should be ~11MB.
	data := "some data to fill the log file and trigger rotation. " +
		"this needs to be long enough to reach 10MB quickly. " +
		"1234567890 1234567890 1234567890"

	for i := 0; i < 110000; i++ {
		slog.Info(data, "i", i)
	}

	closer.Close()

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read tmp dir: %v", err)
	}

	foundRotate := false
	for _, f := range files {
		if f.Name() != "rotate.log" {
			t.Logf("found rotated file: %s", f.Name())
			foundRotate = true
		}
	}

	if !foundRotate {
		t.Error("expected rotated log file, but none found")
	}
}

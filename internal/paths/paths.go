package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// AppDir returns the directory next to the running binary.
// Resolved via os.Executable() (follows symlinks).
func AppDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	// Follow symlinks to find the actual binary location
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}
	return filepath.Dir(resolved), nil
}

// LogsDir returns "<dir(executable)>/logs". Caller is responsible
// for MkdirAll.
func LogsDir() (string, error) {
	dir, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "logs"), nil
}

package config

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMasterKeyEnv(t *testing.T) {
	keyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Setenv("DBSYNC_MASTER_KEY", keyHex)
	defer os.Unsetenv("DBSYNC_MASTER_KEY")

	key, err := LoadMasterKey(context.Background())
	if err != nil {
		t.Fatalf("LoadMasterKey failed: %v", err)
	}

	if hex.EncodeToString(key) != keyHex {
		t.Errorf("Expected %s, got %s", keyHex, hex.EncodeToString(key))
	}
}

func TestLoadMasterKeyEnvInvalid(t *testing.T) {
	os.Setenv("DBSYNC_MASTER_KEY", "too-short")
	defer os.Unsetenv("DBSYNC_MASTER_KEY")

	_, err := LoadMasterKey(context.Background())
	if err == nil {
		t.Error("LoadMasterKey should have failed with invalid env key length")
	}
}

func TestSaltPath(t *testing.T) {
	tmpDir := t.TempDir()
	saltPathOverride = filepath.Join(tmpDir, "salt")
	defer func() { saltPathOverride = "" }()

	path, err := SaltPath()
	if err != nil {
		t.Fatalf("SaltPath failed: %v", err)
	}

	if path != saltPathOverride {
		t.Errorf("Expected %s, got %s", saltPathOverride, path)
	}
}

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

func TestEnsureConsistentState(t *testing.T) {
	setup := func(t *testing.T) (salt, db string) {
		t.Helper()
		tmp := t.TempDir()
		saltPathOverride = filepath.Join(tmp, "dbsync.salt")
		dbPathOverride = filepath.Join(tmp, "dbsync.db")
		t.Cleanup(func() {
			saltPathOverride = ""
			dbPathOverride = ""
		})
		return saltPathOverride, dbPathOverride
	}

	t.Run("both missing -> no-op", func(t *testing.T) {
		setup(t)
		wiped, err := EnsureConsistentState()
		if err != nil || wiped {
			t.Fatalf("expected no-op, got wiped=%v err=%v", wiped, err)
		}
	})

	t.Run("salt missing wipes db", func(t *testing.T) {
		_, db := setup(t)
		if err := os.WriteFile(db, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		wiped, err := EnsureConsistentState()
		if err != nil || !wiped {
			t.Fatalf("expected wipe, got wiped=%v err=%v", wiped, err)
		}
		if _, err := os.Stat(db); !os.IsNotExist(err) {
			t.Fatalf("db should be gone, stat err=%v", err)
		}
	})

	t.Run("db missing wipes salt", func(t *testing.T) {
		salt, _ := setup(t)
		if err := os.WriteFile(salt, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		wiped, err := EnsureConsistentState()
		if err != nil || !wiped {
			t.Fatalf("expected wipe, got wiped=%v err=%v", wiped, err)
		}
		if _, err := os.Stat(salt); !os.IsNotExist(err) {
			t.Fatalf("salt should be gone, stat err=%v", err)
		}
	})

	t.Run("both present -> no-op", func(t *testing.T) {
		salt, db := setup(t)
		_ = os.WriteFile(salt, []byte("x"), 0600)
		_ = os.WriteFile(db, []byte("x"), 0600)
		wiped, err := EnsureConsistentState()
		if err != nil || wiped {
			t.Fatalf("expected no-op, got wiped=%v err=%v", wiped, err)
		}
	})
}

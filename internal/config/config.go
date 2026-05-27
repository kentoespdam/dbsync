package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/paths"
	"golang.org/x/term"
)

// LoadMasterKey loads or derives the 32-byte master key.
func LoadMasterKey(ctx context.Context) ([]byte, error) {
	// 1. Check env var
	if envKey := os.Getenv("DBSYNC_MASTER_KEY"); envKey != "" {
		if len(envKey) != 64 {
			return nil, errors.New("DBSYNC_MASTER_KEY must be 64 hex characters (32 bytes)")
		}
		return hex.DecodeString(envKey)
	}

	// 2. Check if terminal
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, errors.New("DBSYNC_MASTER_KEY not set and stdin is not a terminal. Set the env var or run dbsync interactively.")
	}

	// 3. Load or generate salt
	saltPath, err := SaltPath()
	if err != nil {
		return nil, err
	}

	var salt []byte
	var password string
	if _, err = os.Stat(saltPath); os.IsNotExist(err) {
		// First setup
		fmt.Println("No salt found. This looks like a new setup.")
		password, err = promptPasswordPair()
		if err != nil {
			return nil, err
		}

		salt = make([]byte, crypto.MinSaltSize)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return nil, err
		}

		// Ensure dir exists
		if err := os.MkdirAll(filepath.Dir(saltPath), 0700); err != nil {
			return nil, err
		}

		if err := os.WriteFile(saltPath, salt, 0600); err != nil {
			return nil, err
		}
		fmt.Printf("Master salt generated and saved to %s\n", saltPath)
	} else {
		// Existing setup
		salt, err = os.ReadFile(saltPath)
		if err != nil {
			return nil, err
		}

		// Prompt for password
		fmt.Print("Enter Master Password: ")
		pw, err := term.ReadPassword(fd)
		fmt.Println() // New line after password input
		if err != nil {
			return nil, err
		}
		password = string(pw)
	}

	return crypto.DeriveKey(password, salt)
}

func promptPasswordPair() (string, error) {
	fd := int(os.Stdin.Fd())
	fmt.Print("Create Master Password: ")
	p1, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", err
	}

	fmt.Print("Confirm Master Password: ")
	p2, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", err
	}

	if string(p1) != string(p2) {
		return "", errors.New("passwords do not match")
	}

	return string(p1), nil
}

var (
	saltPathOverride string
	dbPathOverride   string
)

// portableDir returns the directory next to the running binary.
// Falls back to CWD when the executable path cannot be resolved (e.g. `go run`).
func portableDir() (string, error) {
	dir, err := paths.AppDir()
	if err != nil {
		return os.Getwd()
	}
	return dir, nil
}

// DBPath returns the path to the SQLite database file, co-located with the binary.
func DBPath() (string, error) {
	if dbPathOverride != "" {
		return dbPathOverride, nil
	}
	dir, err := portableDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dbsync.db"), nil
}

// SaltPath returns the path to the salt file, co-located with the binary.
func SaltPath() (string, error) {
	if saltPathOverride != "" {
		return saltPathOverride, nil
	}
	dir, err := portableDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dbsync.salt"), nil
}

// EnsureConsistentState enforces the invariant that salt and db live or die together.
// If one exists without the other, the orphan is removed so the next startup runs
// the first-run flow cleanly. Returns true when a wipe happened.
func EnsureConsistentState() (wiped bool, err error) {
	saltPath, err := SaltPath()
	if err != nil {
		return false, err
	}
	dbPath, err := DBPath()
	if err != nil {
		return false, err
	}

	_, saltErr := os.Stat(saltPath)
	_, dbErr := os.Stat(dbPath)
	saltMissing := os.IsNotExist(saltErr)
	dbMissing := os.IsNotExist(dbErr)

	switch {
	case saltMissing && !dbMissing:
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		return true, nil
	case !saltMissing && dbMissing:
		if err := os.Remove(saltPath); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

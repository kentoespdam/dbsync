package storage

import (
	"database/sql"
	"embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	db *sql.DB
}

// Open opens a connection to the SQLite database and runs migrations.
func Open(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %v", err)
	}

	d := &DB{db: db}
	if err := d.runMigrations(); err != nil {
		db.Close()
		return nil, err
	}

	return d, nil
}

func (d *DB) runMigrations() error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}

		if _, err := d.db.Exec(string(content)); err != nil {
			return fmt.Errorf("migration %s failed: %v", entry.Name(), err)
		}
	}

	return nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) Connections() *ConnectionRepo {
	return &ConnectionRepo{db: d.db}
}

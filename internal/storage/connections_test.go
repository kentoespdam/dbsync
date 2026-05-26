package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestConnectionRepo_CRUD(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	repo := db.Connections()
	ctx := context.Background()

	conn := Connection{
		Name:           "test-conn",
		SourceHost:     "localhost",
		SourcePort:     3306,
		SourceUser:     "root",
		SourcePassword: "encrypted-source",
		SourceDB:       "source_db",
		DestHost:       "remote",
		DestPort:       3307,
		DestUser:       "admin",
		DestPassword:   "encrypted-dest",
		DestDB:         "dest_db",
	}

	// Insert
	id, err := repo.Insert(ctx, conn)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if id == 0 {
		t.Fatal("Insert returned 0 ID")
	}

	// GetByID
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Name != conn.Name {
		t.Errorf("Expected name %s, got %s", conn.Name, got.Name)
	}

	// GetByName
	got, err = repo.GetByName(ctx, conn.Name)
	if err != nil {
		t.Fatalf("GetByName failed: %v", err)
	}
	if got.ID != id {
		t.Errorf("Expected ID %d, got %d", id, got.ID)
	}

	// Unique name constraint
	_, err = repo.Insert(ctx, conn)
	if err == nil {
		t.Error("Insert should have failed due to UNIQUE name constraint")
	}

	// List
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(list))
	}

	// Update
	got.SourceHost = "updated-host"
	err = repo.Update(ctx, got)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	updated, _ := repo.GetByID(ctx, id)
	if updated.SourceHost != "updated-host" {
		t.Errorf("Update failed to change SourceHost")
	}

	// Delete
	err = repo.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = repo.GetByID(ctx, id)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound after Delete, got %v", err)
	}
}

func TestMigrationIdempotency(t *testing.T) {
	// Creating a real file to test idempotency
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("First Open failed: %v", err)
	}
	db.Close()

	db, err = Open(dbPath)
	if err != nil {
		t.Fatalf("Second Open failed: %v", err)
	}
	db.Close()
}

package storage

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestCheckpointRepo(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "dbsync_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	connID, _ := db.Connections().Insert(ctx, Connection{Name: "test"})

	repo := db.Checkpoints()

	// 1. Get not found
	_, err = repo.Get(ctx, connID, "table1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// 2. Upsert
	cp := Checkpoint{
		ConnectionID:       connID,
		TableName:          "table1",
		LastBatchCompleted: 5,
		LastPKValue:        "100",
		StartedAt:          time.Now(),
		Status:             "running",
	}
	err = repo.Upsert(ctx, cp)
	if err != nil {
		t.Fatal(err)
	}

	// 3. Get and verify
	cp2, err := repo.Get(ctx, connID, "table1")
	if err != nil {
		t.Fatal(err)
	}
	if cp2.LastPKValue != "100" || cp2.Status != "running" {
		t.Errorf("unexpected checkpoint data: %+v", cp2)
	}

	// 4. Update status
	err = repo.MarkCompleted(ctx, connID, "table1")
	if err != nil {
		t.Fatal(err)
	}
	cp3, _ := repo.Get(ctx, connID, "table1")
	if cp3.Status != "completed" {
		t.Errorf("expected status completed, got %s", cp3.Status)
	}

	// 5. List active
	active, _ := repo.ListActive(ctx)
	if len(active) != 0 {
		t.Errorf("expected 0 active checkpoints, got %d", len(active))
	}

	repo.Upsert(ctx, Checkpoint{ConnectionID: connID, TableName: "table2", Status: "interrupted", StartedAt: time.Now()})
	active, _ = repo.ListActive(ctx)
	if len(active) != 1 || active[0].TableName != "table2" {
		t.Errorf("expected 1 active checkpoint (table2), got %v", active)
	}

	// 6. Delete
	err = repo.Delete(ctx, connID, "table2")
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Get(ctx, connID, "table2")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

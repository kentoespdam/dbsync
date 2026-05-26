package storage

import (
	"context"
	"database/sql"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestHistoryRepo(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "dbsync_test_history_*.db")
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

	repo := db.History()

	// 1. Begin
	id, err := repo.Begin(ctx, connID, "table1")
	if err != nil {
		t.Fatal(err)
	}

	// 2. Latest
	latest, err := repo.Latest(ctx, connID, "table1")
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != id || latest.Status != "running" {
		t.Errorf("expected running record, got %+v", latest)
	}

	// 3. Finish
	h := HistoryRecord{
		FinishedAt:      sql.NullTime{Time: time.Now(), Valid: true},
		DurationSeconds: sql.NullInt64{Int64: 10, Valid: true},
		TotalRows:       sql.NullInt64{Int64: 100, Valid: true},
		SuccessRows:     sql.NullInt64{Int64: 95, Valid: true},
		FailedRows:      sql.NullInt64{Int64: 5, Valid: true},
		Status:          "completed",
		ErrorSummary:    sql.NullString{String: "no errors", Valid: true},
	}
	err = repo.Finish(ctx, id, h)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Verify finished
	latest2, _ := repo.Latest(ctx, connID, "table1")
	if latest2.Status != "completed" || latest2.TotalRows.Int64 != 100 {
		t.Errorf("expected completed 100 rows, got %+v", latest2)
	}

	// 5. List
	repo.Begin(ctx, connID, "table1") // start another
	list, _ := repo.List(ctx, connID, "table1", 10)
	if len(list) != 2 {
		t.Errorf("expected 2 records, got %d", len(list))
	}
	if list[0].Status != "running" || list[1].Status != "completed" {
		t.Errorf("list order or status incorrect")
	}

	// 6. FK Cascade
	db.Connections().Delete(ctx, connID)
	list2, _ := repo.List(ctx, connID, "table1", 10)
	if len(list2) != 0 {
		t.Errorf("expected history to be deleted via FK cascade, got %d", len(list2))
	}
}

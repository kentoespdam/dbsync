package storage

import (
	"context"
	"database/sql"
	"testing"

	"github.com/user/dbsync/internal/mysql"
)

func TestMappingRepo(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	connRepo := db.Connections()
	mappingRepo := db.Mappings()

	// Create a connection for FK
	connID, err := connRepo.Insert(ctx, Connection{
		Name:           "test-conn",
		SourceHost:     "h",
		SourceUser:     "u",
		SourcePassword: "p",
		SourceDB:       "s",
		DestHost:       "h",
		DestUser:       "u",
		DestPassword:   "p",
		DestDB:         "d",
	})
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}

	t.Run("Insert and ListByTable", func(t *testing.T) {
		m := Mapping{
			ConnectionID: connID,
			TableName:    "users",
			SourceColumn: sql.NullString{String: "src_id", Valid: true},
			DestColumn:   "dest_id",
		}
		id, err := mappingRepo.Insert(ctx, m)
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
		if id == 0 {
			t.Fatal("Expected non-zero ID")
		}

		mappings, err := mappingRepo.ListByTable(ctx, connID, "users")
		if err != nil {
			t.Fatalf("ListByTable failed: %v", err)
		}
		if len(mappings) != 1 {
			t.Fatalf("Expected 1 mapping, got %d", len(mappings))
		}
		if mappings[0].DestColumn != "dest_id" {
			t.Errorf("Expected dest_id, got %s", mappings[0].DestColumn)
		}
	})

	t.Run("BulkInsert", func(t *testing.T) {
		ms := []Mapping{
			{
				ConnectionID: connID,
				TableName:    "products",
				SourceColumn: sql.NullString{String: "s1", Valid: true},
				DestColumn:   "d1",
			},
			{
				ConnectionID: connID,
				TableName:    "products",
				SourceColumn: sql.NullString{String: "s2", Valid: true},
				DestColumn:   "d2",
			},
		}
		err := mappingRepo.BulkInsert(ctx, ms)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		mappings, err := mappingRepo.ListByTable(ctx, connID, "products")
		if err != nil {
			t.Fatalf("ListByTable failed: %v", err)
		}
		if len(mappings) != 2 {
			t.Fatalf("Expected 2 mappings, got %d", len(mappings))
		}
	})

	t.Run("Upsert", func(t *testing.T) {
		m := Mapping{
			ConnectionID: connID,
			TableName:    "users",
			SourceColumn: sql.NullString{String: "new_src", Valid: true},
			DestColumn:   "dest_id", // existing dest_column for users
		}
		err := mappingRepo.Upsert(ctx, m)
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}

		mappings, err := mappingRepo.ListByTable(ctx, connID, "users")
		if err != nil {
			t.Fatalf("ListByTable failed: %v", err)
		}
		if len(mappings) != 1 {
			t.Fatalf("Expected 1 mapping (upsert), got %d", len(mappings))
		}
		if mappings[0].SourceColumn.String != "new_src" {
			t.Errorf("Expected updated source_column new_src, got %s", mappings[0].SourceColumn.String)
		}
	})

	t.Run("Validation", func(t *testing.T) {
		m := Mapping{
			ConnectionID: connID,
			TableName:    "users",
			DestColumn:   "fail",
			// Both SourceColumn and DefaultValue are invalid (NULL)
		}
		_, err := mappingRepo.Insert(ctx, m)
		if err == nil {
			t.Error("Expected error when both source and default are NULL")
		}
	})

	t.Run("FK CASCADE", func(t *testing.T) {
		err := connRepo.Delete(ctx, connID)
		if err != nil {
			t.Fatalf("Delete connection failed: %v", err)
		}

		exists, err := mappingRepo.Exists(ctx, connID, "users")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("Mappings should have been deleted by FK CASCADE")
		}
	})
}

func TestAutoMap(t *testing.T) {
	connID := int64(1)
	table := "test"

	tests := []struct {
		name       string
		sourceCols []mysql.Column
		destCols   []mysql.Column
		wantMap    int
		wantWarn   int
		wantUnmap  int
	}{
		{
			name: "All-match",
			sourceCols: []mysql.Column{
				{Name: "id", IsNullable: false},
				{Name: "name", IsNullable: true},
			},
			destCols: []mysql.Column{
				{Name: "id", IsNullable: false},
				{Name: "name", IsNullable: true},
			},
			wantMap:   2,
			wantWarn:  0,
			wantUnmap: 0,
		},
		{
			name: "Extra dest NOT NULL",
			sourceCols: []mysql.Column{
				{Name: "id", IsNullable: false},
			},
			destCols: []mysql.Column{
				{Name: "id", IsNullable: false},
				{Name: "tenant_id", IsNullable: false},
			},
			wantMap:   1,
			wantWarn:  1,
			wantUnmap: 0,
		},
		{
			name: "Extra dest nullable",
			sourceCols: []mysql.Column{
				{Name: "id", IsNullable: false},
			},
			destCols: []mysql.Column{
				{Name: "id", IsNullable: false},
				{Name: "deleted_at", IsNullable: true},
			},
			wantMap:   1,
			wantWarn:  0,
			wantUnmap: 0,
		},
		{
			name: "Extra source informational",
			sourceCols: []mysql.Column{
				{Name: "id", IsNullable: false},
				{Name: "old_col", IsNullable: true},
			},
			destCols: []mysql.Column{
				{Name: "id", IsNullable: false},
			},
			wantMap:   1,
			wantWarn:  0,
			wantUnmap: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := AutoMap(connID, table, tt.sourceCols, tt.destCols)
			if len(res.Mappings) != tt.wantMap {
				t.Errorf("got %d mappings, want %d", len(res.Mappings), tt.wantMap)
			}
			if len(res.Warnings) != tt.wantWarn {
				t.Errorf("got %d warnings, want %d", len(res.Warnings), tt.wantWarn)
			}
			if len(res.UnmappedSource) != tt.wantUnmap {
				t.Errorf("got %d unmapped source, want %d", len(res.UnmappedSource), tt.wantUnmap)
			}
		})
	}
}

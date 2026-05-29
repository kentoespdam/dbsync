package storage

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kentoespdam/dbsync/internal/mysql"
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
			{
				ConnectionID: connID,
				TableName:    "products",
				DestColumn:   "skipped",
				// invalid mapping
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
			t.Fatalf("Expected 2 mappings (1 skipped), got %d", len(mappings))
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

	t.Run("ValueMap Round-trip", func(t *testing.T) {
		// Need a new connection since the old one was deleted by FK CASCADE test
		cID, _ := connRepo.Insert(ctx, Connection{
			Name: "test-conn-vmap", SourceHost: "h", SourceUser: "u", SourcePassword: "p", SourceDB: "s",
			DestHost: "h", DestUser: "u", DestPassword: "p", DestDB: "d",
		})

		vmapJSON := `{"Draft":"DRAFT","Ditampilkan":"PUBLISHED"}`
		m := Mapping{
			ConnectionID: cID,
			TableName:    "articles",
			SourceColumn: sql.NullString{String: "status", Valid: true},
			DestColumn:   "status",
			ValueMap:     sql.NullString{String: vmapJSON, Valid: true},
		}

		_, err := mappingRepo.Insert(ctx, m)
		if err != nil {
			t.Fatalf("Insert with ValueMap failed: %v", err)
		}

		mappings, err := mappingRepo.ListByTable(ctx, cID, "articles")
		if err != nil {
			t.Fatalf("ListByTable failed: %v", err)
		}
		if len(mappings) != 1 {
			t.Fatalf("Expected 1 mapping, got %d", len(mappings))
		}
		if !mappings[0].ValueMap.Valid || mappings[0].ValueMap.String != vmapJSON {
			t.Errorf("ValueMap round-trip failed: got %q, want %q", mappings[0].ValueMap.String, vmapJSON)
		}

		// Test Upsert overwrite
		newVmap := `{"Draft":"DRAFT"}`
		m.ValueMap.String = newVmap
		err = mappingRepo.Upsert(ctx, m)
		if err != nil {
			t.Fatalf("Upsert overwrite ValueMap failed: %v", err)
		}
		mappings, _ = mappingRepo.ListByTable(ctx, cID, "articles")
		if mappings[0].ValueMap.String != newVmap {
			t.Errorf("Upsert ValueMap update failed: got %q, want %q", mappings[0].ValueMap.String, newVmap)
		}
	})

	t.Run("ValueMap JSON Check", func(t *testing.T) {
		cID, _ := connRepo.Insert(ctx, Connection{
			Name: "test-conn-vmap-check", SourceHost: "h", SourceUser: "u", SourcePassword: "p", SourceDB: "s",
			DestHost: "h", DestUser: "u", DestPassword: "p", DestDB: "d",
		})
		m := Mapping{
			ConnectionID: cID,
			TableName:    "err",
			DestColumn:   "c",
			ValueMap:     sql.NullString{String: "invalid json", Valid: true},
		}
		_, err := mappingRepo.Insert(ctx, m)
		if err == nil {
			t.Error("Expected error from CHECK constraint for invalid JSON")
		}
	})
}

func TestValidateMapping(t *testing.T) {
	enumCol := mysql.Column{
		Name:       "status",
		ColumnType: "enum('DRAFT','PUBLISHED','DELETED')",
	}
	nonEnumCol := mysql.Column{
		Name:       "name",
		ColumnType: "varchar(255)",
	}

	tests := []struct {
		name    string
		vmap    string
		destCol mysql.Column
		wantErr bool
	}{
		{
			name:    "Valid ENUM values",
			vmap:    `{"Draft":"DRAFT","Pub":"PUBLISHED"}`,
			destCol: enumCol,
			wantErr: false,
		},
		{
			name:    "Invalid ENUM value",
			vmap:    `{"Draft":"DRAFT","X":"UNKNOWN"}`,
			destCol: enumCol,
			wantErr: true,
		},
		{
			name:    "Non-ENUM column",
			vmap:    `{"a":"b"}`,
			destCol: nonEnumCol,
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			vmap:    `{invalid}`,
			destCol: enumCol,
			wantErr: true,
		},
		{
			name:    "Empty ValueMap",
			vmap:    "",
			destCol: enumCol,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Mapping{DestColumn: tt.destCol.Name}
			if tt.vmap != "" {
				m.ValueMap = sql.NullString{String: tt.vmap, Valid: true}
			}
			err := ValidateMapping(m, tt.destCol)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMapping() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
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
			if len(res.Mappings) != len(tt.destCols) {
				t.Errorf("got %d mappings, want %d (len(destCols))", len(res.Mappings), len(tt.destCols))
			}
			if len(res.Warnings) != tt.wantWarn {
				t.Errorf("got %d warnings, want %d", len(res.Warnings), tt.wantWarn)
			}
			if len(res.UnmappedSource) != tt.wantUnmap {
				t.Errorf("got %d unmapped source, want %d", len(res.UnmappedSource), tt.wantUnmap)
			}

			// Verify synthetic mappings
			for i, m := range res.Mappings {
				dc := tt.destCols[i]
				if m.DestColumn != dc.Name {
					t.Errorf("Mapping %d DestColumn = %s, want %s", i, m.DestColumn, dc.Name)
				}
				// If no match in source, it should be invalid
				hasMatch := false
				for _, sc := range tt.sourceCols {
					if sc.Name == dc.Name {
						hasMatch = true
						break
					}
				}
				if !hasMatch {
					if m.SourceColumn.Valid {
						t.Errorf("Mapping %d (no match) SourceColumn.Valid is true", i)
					}
					if m.DefaultValue.Valid {
						t.Errorf("Mapping %d (no match) DefaultValue.Valid is true", i)
					}
				}
			}
		})
	}
}

func TestAutoMapEnumMismatch(t *testing.T) {
	connID := int64(1)
	table := "test"

	tests := []struct {
		name            string
		sourceCols      []mysql.Column
		destCols        []mysql.Column
		wantMismatch    int
		expectedSuggest  string
	}{
		{
			name: "Identical domain case",
			sourceCols: []mysql.Column{
				{Name: "status", ColumnType: "enum('DRAFT','PUBLISHED','DELETED')"},
			},
			destCols: []mysql.Column{
				{Name: "status", ColumnType: "enum('DRAFT','PUBLISHED','DELETED')"},
			},
			wantMismatch:   0,
		},
		{
			name: "Different case",
			sourceCols: []mysql.Column{
				{Name: "status", ColumnType: "enum('Draft','Ditampilkan')"},
			},
			destCols: []mysql.Column{
				{Name: "status", ColumnType: "enum('DRAFT','PUBLISHED')"},
			},
			wantMismatch:    1,
			expectedSuggest:  "Draft=DRAFT,Ditampilkan=PUBLISHED",
		},
		{
			name: "Dest superset",
			sourceCols: []mysql.Column{
				{Name: "status", ColumnType: "enum('Draft','Ditampilkan')"},
			},
			destCols: []mysql.Column{
				{Name: "status", ColumnType: "enum('DRAFT','PUBLISHED','DELETED')"},
			},
			wantMismatch:    1,
			expectedSuggest: "Draft=DRAFT,Ditampilkan=PUBLISHED",
		},
		{
			name: "Source superset",
			sourceCols: []mysql.Column{
				{Name: "status", ColumnType: "enum('A','B','C')"},
			},
			destCols: []mysql.Column{
				{Name: "status", ColumnType: "enum('A','B')"},
			},
			wantMismatch:    1,
			expectedSuggest: "A=A,B=B",
		},
		{
			name: "One not ENUM",
			sourceCols: []mysql.Column{
				{Name: "status", ColumnType: "enum('Draft','Ditampilkan')"},
			},
			destCols: []mysql.Column{
				{Name: "name", ColumnType: "varchar(255)"},
			},
			wantMismatch: 0,
		},
		{
			name: "Both not ENUM",
			sourceCols: []mysql.Column{
				{Name: "name", ColumnType: "varchar(255)"},
			},
			destCols: []mysql.Column{
				{Name: "name", ColumnType: "varchar(255)"},
			},
			wantMismatch: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := AutoMap(connID, table, tt.sourceCols, tt.destCols)
			if len(res.EnumMismatches) != tt.wantMismatch {
				t.Errorf("got %d mismatches, want %d", len(res.EnumMismatches), tt.wantMismatch)
			}

			if tt.wantMismatch > 0 {
				mismatch := res.EnumMismatches[0]
				if mismatch.DestColumn != tt.destCols[0].Name {
					t.Errorf("DestColumn = %s, want %s", mismatch.DestColumn, tt.destCols[0].Name)
				}
				if !stringSlicesEqual(mismatch.SourceValues, tt.sourceCols[0].EnumValues()) {
					t.Errorf("SourceValues = %v, want %v", mismatch.SourceValues, tt.sourceCols[0].EnumValues())
				}
				if !stringSlicesEqual(mismatch.DestValues, tt.destCols[0].EnumValues()) {
					t.Errorf("DestValues = %v, want %v", mismatch.DestValues, tt.destCols[0].EnumValues())
				}
				if mismatch.Suggested != tt.expectedSuggest {
					t.Errorf("Suggested = %s, want %s", mismatch.Suggested, tt.expectedSuggest)
				}
			}
		})
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

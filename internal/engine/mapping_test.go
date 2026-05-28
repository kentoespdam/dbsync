package engine

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/kentoespdam/dbsync/internal/mysql"
	"github.com/kentoespdam/dbsync/internal/storage"
)

func TestResolve(t *testing.T) {
	mappings := []storage.Mapping{
		{
			DestColumn:   "id",
			SourceColumn: sql.NullString{String: "id", Valid: true},
		},
		{
			DestColumn:   "created_at",
			DefaultValue: sql.NullString{String: "NOW()", Valid: true},
		},
		{
			DestColumn:   "status",
			SourceColumn: sql.NullString{String: "source_status", Valid: true},
			DefaultValue: sql.NullString{String: "'active'", Valid: true},
		},
		{
			DestColumn:   "version",
			DefaultValue: sql.NullString{String: "1", Valid: true},
		},
	}

	resolved := Resolve(mappings)
	assert.Equal(t, 4, len(resolved))

	row := mysql.Row{
		"id":            int64(100),
		"source_status": nil,
	}

	// Case 1: Source only
	val, err := resolved[0].ValueFn(row)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), val)

	// Case 2: Default NOW()
	val, err = resolved[1].ValueFn(row)
	assert.NoError(t, err)
	assert.IsType(t, time.Time{}, val)
	assert.WithinDuration(t, time.Now(), val.(time.Time), time.Second)

	// Case 3: Source is nil, use default
	val, err = resolved[2].ValueFn(row)
	assert.NoError(t, err)
	assert.Equal(t, "active", val)

	// Case 3: Source is present, use source
	row["source_status"] = "pending"
	val, err = resolved[2].ValueFn(row)
	assert.NoError(t, err)
	assert.Equal(t, "pending", val)

	// Default number
	val, err = resolved[3].ValueFn(row)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), val)
}

func TestParseDefaultValue(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"NOW()", "time"},
		{"123", int64(123)},
		{"'hello'", "hello"},
		{"\"world\"", "world"},
		{"normal", "normal"},
	}

	for _, tt := range tests {
		res := parseDefaultValue(tt.input)
		if tt.expected == "time" {
			fn := res.(func() any)
			assert.IsType(t, time.Time{}, fn())
		} else {
			assert.Equal(t, tt.expected, res)
		}
	}
}

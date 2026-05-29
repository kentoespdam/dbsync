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

	resolved, err := Resolve(mappings)
	assert.NoError(t, err)
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

func TestResolve_ValueMap(t *testing.T) {
	mappings := []storage.Mapping{
		{
			DestColumn:   "status",
			SourceColumn: sql.NullString{String: "src_status", Valid: true},
			ValueMap:     sql.NullString{String: `{"Draft":"DRAFT","Published":"PUBLISHED"}`, Valid: true},
		},
		{
			DestColumn:   "type_id",
			SourceColumn: sql.NullString{String: "src_type", Valid: true},
			DefaultValue: sql.NullString{String: "0", Valid: true},
			ValueMap:     sql.NullString{String: `{"1":"active","0":"inactive"}`, Valid: true},
		},
	}

	resolved, err := Resolve(mappings)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(resolved))

	t.Run("Hit", func(t *testing.T) {
		row := mysql.Row{"src_status": "Draft"}
		val, err := resolved[0].ValueFn(row)
		assert.NoError(t, err)
		assert.Equal(t, "DRAFT", val)
	})

	t.Run("Miss", func(t *testing.T) {
		row := mysql.Row{"src_status": "Unknown"}
		val, err := resolved[0].ValueFn(row)
		assert.Error(t, err)
		assert.Nil(t, val)
		
		missErr, ok := err.(*ValueMapMissError)
		assert.True(t, ok)
		assert.Equal(t, "status", missErr.Column)
		assert.Equal(t, "Unknown", missErr.Value)
	})

	t.Run("NilSourcePassthroughToDefaultThenMap", func(t *testing.T) {
		row := mysql.Row{"src_type": nil}
		val, err := resolved[1].ValueFn(row)
		assert.NoError(t, err)
		assert.Equal(t, "inactive", val) // 0 -> inactive
	})

	t.Run("NumericSourceToStringKeyLookup", func(t *testing.T) {
		row := mysql.Row{"src_type": int64(1)}
		val, err := resolved[1].ValueFn(row)
		assert.NoError(t, err)
		assert.Equal(t, "active", val) // 1 -> active
	})
	
	t.Run("NullValuePassthrough", func(t *testing.T) {
		m := []storage.Mapping{
			{
				DestColumn:   "opt",
				SourceColumn: sql.NullString{String: "src_opt", Valid: true},
				ValueMap:     sql.NullString{String: `{"a":"b"}`, Valid: true},
			},
		}
		res, err := Resolve(m)
		assert.NoError(t, err)
		row := mysql.Row{"src_opt": nil}
		val, err := res[0].ValueFn(row)
		assert.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		m := []storage.Mapping{
			{
				DestColumn:   "status",
				SourceColumn: sql.NullString{String: "src_status", Valid: true},
				ValueMap:     sql.NullString{String: `{"Draft":invalid}`, Valid: true},
			},
		}
		_, err := Resolve(m)
		assert.Error(t, err)
	})

	t.Run("ValueMapWithoutSourceColumn", func(t *testing.T) {
		m := []storage.Mapping{
			{
				DestColumn: "status",
				ValueMap:   sql.NullString{String: `{"Draft":"DRAFT"}`, Valid: true},
			},
		}
		_, err := Resolve(m)
		assert.Error(t, err)
	})
}

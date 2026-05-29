package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kentoespdam/dbsync/internal/mysql"
	"github.com/kentoespdam/dbsync/internal/storage"
)

// ValueMapMissError indicates a source value was not found in the translation map (bd-13b).
type ValueMapMissError struct {
	Column string
	Value  any
}

func (e *ValueMapMissError) Error() string {
	return fmt.Sprintf("value %v not found in ValueMap for column %s", e.Value, e.Column)
}

// Resolve converts storage mappings into engine-ready resolved mappings (bd-13b).
func Resolve(mappings []storage.Mapping) ([]mysql.ResolvedMapping, error) {
	resolved := make([]mysql.ResolvedMapping, len(mappings))
	for i, m := range mappings {
		m := m // capture for closure
		res := mysql.ResolvedMapping{
			DestColumn: m.DestColumn,
		}

		var valueMap map[string]string
		if m.ValueMap.Valid && m.ValueMap.String != "" {
			if !m.SourceColumn.Valid {
				return nil, fmt.Errorf("value_map requires source_column for column %s (bd-13b)", m.DestColumn)
			}
			if err := json.Unmarshal([]byte(m.ValueMap.String), &valueMap); err != nil {
				return nil, fmt.Errorf("invalid value_map JSON for column %s: %w (bd-13b)", m.DestColumn, err)
			}
		}

		// Base value resolution (Source, Default, or both)
		var baseFn func(row mysql.Row) (any, error)

		if m.SourceColumn.Valid && !m.DefaultValue.Valid {
			// Case 1: Source only
			baseFn = func(row mysql.Row) (any, error) {
				val, ok := row[m.SourceColumn.String]
				if !ok {
					return nil, fmt.Errorf("column %s not found in source row", m.SourceColumn.String)
				}
				return val, nil
			}
		} else if !m.SourceColumn.Valid && m.DefaultValue.Valid {
			// Case 2: Default only
			defaultVal := parseDefaultValue(m.DefaultValue.String)
			baseFn = func(row mysql.Row) (any, error) {
				if fn, ok := defaultVal.(func() any); ok {
					return fn(), nil
				}
				return defaultVal, nil
			}
		} else if m.SourceColumn.Valid && m.DefaultValue.Valid {
			// Case 3: Both (Source with Default fallback)
			defaultVal := parseDefaultValue(m.DefaultValue.String)
			baseFn = func(row mysql.Row) (any, error) {
				val, ok := row[m.SourceColumn.String]
				if ok && val != nil {
					return val, nil
				}
				if fn, ok := defaultVal.(func() any); ok {
					return fn(), nil
				}
				return defaultVal, nil
			}
		} else {
			// fallback for incomplete mapping (e.g. only DestColumn)
			baseFn = func(row mysql.Row) (any, error) { return nil, nil }
		}

		// Wrap baseFn with ValueMap if present
		res.ValueFn = func(row mysql.Row) (any, error) {
			val, err := baseFn(row)
			if err != nil {
				return nil, err
			}

			if val == nil || valueMap == nil {
				return val, nil
			}

			// ValueMap lookup (convert val to string key)
			key := fmt.Sprint(val)
			if mappedVal, ok := valueMap[key]; ok {
				return mappedVal, nil
			}

			return nil, &ValueMapMissError{Column: m.DestColumn, Value: val}
		}

		resolved[i] = res
	}
	return resolved, nil
}

func parseDefaultValue(s string) any {
	upper := strings.ToUpper(s)
	if upper == "NOW()" || upper == "CURRENT_TIMESTAMP" || upper == "CURRENT_TIMESTAMP()" {
		return func() any { return time.Now() }
	}

	// Try number
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	// String literal - strip quotes if present
	if len(s) >= 2 && ((s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"')) {
		return s[1 : len(s)-1]
	}

	return s
}

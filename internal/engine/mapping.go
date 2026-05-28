package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kentoespdam/dbsync/internal/mysql"
	"github.com/kentoespdam/dbsync/internal/storage"
)

// Resolve converts storage mappings into engine-ready resolved mappings.
func Resolve(mappings []storage.Mapping) []mysql.ResolvedMapping {
	resolved := make([]mysql.ResolvedMapping, len(mappings))
	for i, m := range mappings {
		m := m // capture for closure
		res := mysql.ResolvedMapping{
			DestColumn: m.DestColumn,
		}

		if m.SourceColumn.Valid && !m.DefaultValue.Valid {
			// Case 1: Source only
			res.ValueFn = func(row mysql.Row) (any, error) {
				val, ok := row[m.SourceColumn.String]
				if !ok {
					return nil, fmt.Errorf("column %s not found in source row", m.SourceColumn.String)
				}
				return val, nil
			}
		} else if !m.SourceColumn.Valid && m.DefaultValue.Valid {
			// Case 2: Default only
			defaultVal := parseDefaultValue(m.DefaultValue.String)
			res.ValueFn = func(row mysql.Row) (any, error) {
				if fn, ok := defaultVal.(func() any); ok {
					return fn(), nil
				}
				return defaultVal, nil
			}
		} else if m.SourceColumn.Valid && m.DefaultValue.Valid {
			// Case 3: Both (Source with Default fallback)
			defaultVal := parseDefaultValue(m.DefaultValue.String)
			res.ValueFn = func(row mysql.Row) (any, error) {
				val, ok := row[m.SourceColumn.String]
				if ok && val != nil {
					return val, nil
				}
				if fn, ok := defaultVal.(func() any); ok {
					return fn(), nil
				}
				return defaultVal, nil
			}
		}

		resolved[i] = res
	}
	return resolved
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

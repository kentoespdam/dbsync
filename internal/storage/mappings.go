package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kentoespdam/dbsync/internal/mysql"
)

type Mapping struct {
	ID           int64
	ConnectionID int64
	TableName    string
	SourceColumn sql.NullString // nullable per PRD case 2
	DestColumn   string
	DefaultValue sql.NullString // nullable per PRD case 1
	ValueMap     sql.NullString // JSON {"src":"dest"}, bd-13a
	CreatedAt    time.Time
}

// ValidateMapping validates ValueMap against destination column metadata. (bd-13a)
// If destCol is an ENUM, it ensures all target values in ValueMap exist in EnumValues.
func ValidateMapping(m Mapping, destCol mysql.Column) error {
	if !m.ValueMap.Valid {
		return nil
	}

	var vmap map[string]string
	if err := json.Unmarshal([]byte(m.ValueMap.String), &vmap); err != nil {
		return fmt.Errorf("value_map: invalid JSON: %v", err)
	}

	enumValues := destCol.EnumValues()
	if len(enumValues) == 0 {
		return nil // Not an ENUM, skip validation
	}

	enumSet := make(map[string]bool)
	for _, v := range enumValues {
		enumSet[v] = true
	}

	for k, v := range vmap {
		if !enumSet[v] {
			return fmt.Errorf("value_map: value %q (for key %q) not in dest ENUM domain %v", v, k, enumValues)
		}
	}

	return nil
}

type EnumDomainMismatch struct {
	DestColumn   string   // bd-13d
	SourceValues []string // from source ENUM
	DestValues   []string // from dest ENUM
	Suggested    string   // command line ready-to-copy
}

type AutoMapResult struct {
	Mappings        []Mapping               // generated mappings
	Warnings       []string                 // dest NOT NULL cols without match/default
	UnmappedSource []string                 // source cols without match in dest
	EnumMismatches []EnumDomainMismatch     // bd-13d
}

func AutoMap(connID int64, table string, sourceCols, destCols []mysql.Column) AutoMapResult {
	var res AutoMapResult
	sourceMap := make(map[string]mysql.Column)
	for _, sc := range sourceCols {
		sourceMap[sc.Name] = sc
	}

	usedSource := make(map[string]bool)
	for _, dc := range destCols {
		m := Mapping{
			ConnectionID: connID, TableName: table, DestColumn: dc.Name,
		}
		if sc, ok := sourceMap[dc.Name]; ok {
			m.SourceColumn = sql.NullString{String: sc.Name, Valid: true}
			usedSource[sc.Name] = true
		} else if !dc.IsNullable {
			res.Warnings = append(res.Warnings, fmt.Sprintf("dest column %s is NOT NULL but has no match in source", dc.Name))
		}
		res.Mappings = append(res.Mappings, m)
	}

	for _, sc := range sourceCols {
		if !usedSource[sc.Name] {
			res.UnmappedSource = append(res.UnmappedSource, sc.Name)
		}
	}

	for _, dc := range destCols {
		if sc, ok := sourceMap[dc.Name]; ok {
			srcVals := sc.EnumValues()
			destVals := dc.EnumValues()
			if len(srcVals) > 0 && len(destVals) > 0 {
				if !stringSetsEqual(srcVals, destVals) {
					suggested := generateSuggestedValueMap(srcVals, destVals)
					res.EnumMismatches = append(res.EnumMismatches, EnumDomainMismatch{
						DestColumn:   dc.Name,
						SourceValues: srcVals,
						DestValues:   destVals,
						Suggested:    suggested,
					})
				}
			}
		}
	}
	return res
}

func stringSetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool)
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		if !set[v] {
			return false
		}
	}
	return true
}

func generateSuggestedValueMap(srcVals, destVals []string) string {
	var pairs []string
	minLen := len(srcVals)
	if len(destVals) < minLen {
		minLen = len(destVals)
	}
	for i := 0; i < minLen; i++ {
		pairs = append(pairs, fmt.Sprintf("%s=%s", srcVals[i], destVals[i]))
	}
	return strings.Join(pairs, ",")
}

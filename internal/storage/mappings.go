package storage

import (
	"database/sql"
	"fmt"
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
	CreatedAt    time.Time
}

type AutoMapResult struct {
	Mappings       []Mapping // generated mappings
	Warnings       []string  // dest NOT NULL cols without match/default
	UnmappedSource []string  // source cols without match in dest
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
	return res
}

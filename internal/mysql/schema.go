package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// Column represents metadata for a table column.
type Column struct {
	Name       string
	DataType   string // e.g. "varchar", "int", "bigint"
	ColumnType string // e.g. "varchar(255)", "tinyint(1)", "enum('a','b')"
	IsNullable bool
	ColumnKey  string // "PRI", "UNI", "MUL", ""
}

// IsBool returns true if the column is a boolean (tinyint(1) in MySQL).
func (c Column) IsBool() bool {
	return strings.EqualFold(c.ColumnType, "tinyint(1)")
}

var enumRegex = regexp.MustCompile(`'([^']*)'`)

// EnumValues returns the allowed values for an ENUM column.
func (c Column) EnumValues() []string {
	if !strings.HasPrefix(strings.ToLower(c.ColumnType), "enum(") {
		return nil
	}

	matches := enumRegex.FindAllStringSubmatch(c.ColumnType, -1)
	values := make([]string, 0, len(matches))
	for _, m := range matches {
		values = append(values, m[1])
	}
	return values
}

// DetectPK returns the primary key column names in their ordinal order.
func DetectPK(ctx context.Context, db *sql.DB, schema, table string) ([]string, error) {
	query := `
		SELECT COLUMN_NAME
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY'
		ORDER BY ORDINAL_POSITION
	`
	rows, err := db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("detect PK: %w", err)
	}
	defer rows.Close()

	var pks []string
	for rows.Next() {
		var pk string
		if err := rows.Scan(&pk); err != nil {
			return nil, fmt.Errorf("scan PK: %w", err)
		}
		pks = append(pks, pk)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return pks, nil
}

// DescribeColumns returns metadata for all columns in a table.
func DescribeColumns(ctx context.Context, db *sql.DB, schema, table string) ([]Column, error) {
	query := `
		SELECT COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`
	rows, err := db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("describe columns: %w", err)
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		var isNullable string
		if err := rows.Scan(&col.Name, &col.DataType, &col.ColumnType, &isNullable, &col.ColumnKey); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		col.IsNullable = (isNullable == "YES")
		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return columns, nil
}

// ListTables returns a list of base tables in the given schema.
func ListTables(ctx context.Context, db *sql.DB, schema string) ([]string, error) {
	query := `
		SELECT TABLE_NAME
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME
	`
	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tables, nil
}

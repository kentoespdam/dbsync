package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Row represents a single row of data from a MySQL table.
type Row map[string]any

// ResolvedMapping defines how to produce a value for a destination column from a source row.
type ResolvedMapping struct {
	DestColumn string
	ValueFn    func(row Row) (any, error)
}

// CountRows returns the total number of rows in a table.
func CountRows(ctx context.Context, db *sql.DB, schema, table string) (int, error) {
	quotedSchema := fmt.Sprintf("`%s`", strings.ReplaceAll(schema, "`", "``"))
	quotedTable := fmt.Sprintf("`%s`", strings.ReplaceAll(table, "`", "``"))
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", quotedSchema, quotedTable)

	var count int
	err := db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count rows: %w", err)
	}
	return count, nil
}

// SelectBatch fetches a batch of rows from a table using primary key-based pagination.
func SelectBatch(ctx context.Context, db *sql.DB, schema, table string, selectCols []string, pkCols []string, lastPK []any, limit int) (rows []Row, nextPK []any, err error) {
	if len(pkCols) == 0 {
		return nil, nil, fmt.Errorf("pkCols cannot be empty")
	}

	quotedSchema := fmt.Sprintf("`%s`", strings.ReplaceAll(schema, "`", "``"))
	quotedTable := fmt.Sprintf("`%s`", strings.ReplaceAll(table, "`", "``"))
	
	quotedPKCols := make([]string, len(pkCols))
	for i, col := range pkCols {
		quotedPKCols[i] = fmt.Sprintf("`%s`", strings.ReplaceAll(col, "`", "``"))
	}

	selectClause := "*"
	if len(selectCols) > 0 {
		quotedSelectCols := make([]string, len(selectCols))
		for i, col := range selectCols {
			quotedSelectCols[i] = fmt.Sprintf("`%s`", strings.ReplaceAll(col, "`", "``"))
		}
		selectClause = strings.Join(quotedSelectCols, ", ")
	}

	var whereClause string
	var args []any
	if lastPK != nil {
		if len(pkCols) == 1 {
			whereClause = fmt.Sprintf(" WHERE %s > ?", quotedPKCols[0])
			args = append(args, lastPK[0])
		} else {
			// Composite PK: (pk1, pk2, ...) > (?, ?, ...)
			placeholders := make([]string, len(pkCols))
			for i := range placeholders {
				placeholders[i] = "?"
			}
			whereClause = fmt.Sprintf(" WHERE (%s) > (%s)", 
				strings.Join(quotedPKCols, ", "), 
				strings.Join(placeholders, ", "))
			args = append(args, lastPK...)
		}
	}

	orderBy := strings.Join(quotedPKCols, ", ")
	query := fmt.Sprintf("SELECT %s FROM %s.%s%s ORDER BY %s LIMIT %d",
		selectClause, quotedSchema, quotedTable, whereClause, orderBy, limit)

	dbRows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("select batch: %w", err)
	}
	defer dbRows.Close()

	cols, err := dbRows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("get columns: %w", err)
	}

	for dbRows.Next() {
		columns := make([]any, len(cols))
		columnPointers := make([]any, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := dbRows.Scan(columnPointers...); err != nil {
			return nil, nil, fmt.Errorf("scan row: %w", err)
		}

		row := make(Row)
		for i, colName := range cols {
			val := columns[i]
			if b, ok := val.([]byte); ok {
				row[colName] = string(b)
			} else {
				row[colName] = val
			}
		}
		rows = append(rows, row)
	}

	if err := dbRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("rows error: %w", err)
	}

	if len(rows) > 0 {
		lastRow := rows[len(rows)-1]
		nextPK = make([]any, len(pkCols))
		for i, col := range pkCols {
			nextPK[i] = lastRow[col]
		}
	}

	return rows, nextPK, nil
}

// Upsert performs a batch insert or update (on duplicate key) operation.
func Upsert(ctx context.Context, tx *sql.Tx, schema, table string, pkCols []string, mappings []ResolvedMapping, rows []Row) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	quotedSchema := fmt.Sprintf("`%s`", strings.ReplaceAll(schema, "`", "``"))
	quotedTable := fmt.Sprintf("`%s`", strings.ReplaceAll(table, "`", "``"))

	destCols := make([]string, len(mappings))
	quotedDestCols := make([]string, len(mappings))
	for i, m := range mappings {
		destCols[i] = m.DestColumn
		quotedDestCols[i] = fmt.Sprintf("`%s`", strings.ReplaceAll(m.DestColumn, "`", "``"))
	}

	// Build placeholders (?, ?, ...), (?, ?, ...), ...
	rowPlaceholders := "(" + strings.Repeat("?, ", len(mappings)-1) + "?)"
	valuePlaceholders := make([]string, len(rows))
	args := make([]any, 0, len(rows)*len(mappings))
	for i, row := range rows {
		for _, m := range mappings {
			val, err := m.ValueFn(row)
			if err != nil {
				return 0, fmt.Errorf("resolve value for row %d, col %s: %w", i, m.DestColumn, err)
			}
			args = append(args, val)
		}
		valuePlaceholders[i] = rowPlaceholders
	}

	// ON DUPLICATE KEY UPDATE col1 = VALUES(col1), ...
	// Skip primary keys in UPDATE clause
	pkMap := make(map[string]bool)
	for _, pk := range pkCols {
		pkMap[pk] = true
	}

	var updateParts []string
	for _, col := range destCols {
		if pkMap[col] {
			continue
		}
		quotedCol := fmt.Sprintf("`%s`", strings.ReplaceAll(col, "`", "``"))
		updateParts = append(updateParts, fmt.Sprintf("%s = VALUES(%s)", quotedCol, quotedCol))
	}

	query := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES %s",
		quotedSchema, quotedTable, strings.Join(quotedDestCols, ", "), strings.Join(valuePlaceholders, ", "))

	if len(updateParts) > 0 {
		query += " ON DUPLICATE KEY UPDATE " + strings.Join(updateParts, ", ")
	}

	_, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("upsert exec: %w", err)
	}

	return len(rows), nil
}

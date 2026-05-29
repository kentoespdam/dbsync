package storage

import (
	"context"
	"database/sql"
	"fmt"
)

func (r *MappingRepo) Upsert(ctx context.Context, m Mapping) error {
	if !m.SourceColumn.Valid && !m.DefaultValue.Valid && !m.ValueMap.Valid {
		return fmt.Errorf("mapping must have at least a source column, default value, or value map")
	}

	query := `
		INSERT INTO sync_column_mappings (connection_id, table_name, source_column, dest_column, default_value, value_map)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (connection_id, table_name, dest_column) DO UPDATE SET
			source_column = excluded.source_column,
			default_value = excluded.default_value,
			value_map = excluded.value_map
	`
	_, err := r.db.ExecContext(ctx, query, m.ConnectionID, m.TableName, m.SourceColumn, m.DestColumn, m.DefaultValue, m.ValueMap)
	return err
}

func (r *MappingRepo) DeleteByTable(ctx context.Context, connID int64, table string) error {
	query := `DELETE FROM sync_column_mappings WHERE connection_id = ? AND table_name = ?`
	_, err := r.db.ExecContext(ctx, query, connID, table)
	return err
}

func (r *MappingRepo) DeleteByDest(ctx context.Context, connID int64, table, dest string) error {
	query := `DELETE FROM sync_column_mappings WHERE connection_id = ? AND table_name = ? AND dest_column = ?`
	_, err := r.db.ExecContext(ctx, query, connID, table, dest)
	return err
}

func (r *MappingRepo) ListDistinctTables(ctx context.Context, connID int64) ([]string, error) {
	query := `SELECT DISTINCT table_name FROM sync_column_mappings WHERE connection_id = ? ORDER BY table_name`
	rows, err := r.db.QueryContext(ctx, query, connID)
	if err != nil { return nil, err }
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil { return nil, err }
		tables = append(tables, table)
	}
	return tables, nil
}

func (r *MappingRepo) Exists(ctx context.Context, connID int64, table string) (bool, error) {
	query := `SELECT 1 FROM sync_column_mappings WHERE connection_id = ? AND table_name = ? LIMIT 1`
	var exists int
	err := r.db.QueryRowContext(ctx, query, connID, table).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows { return false, nil }
		return false, err
	}
	return true, nil
}

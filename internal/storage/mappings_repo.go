package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type MappingRepo struct {
	db *sql.DB
}

func (d *DB) Mappings() *MappingRepo {
	return &MappingRepo{db: d.db}
}

func (r *MappingRepo) Insert(ctx context.Context, m Mapping) (int64, error) {
	if !m.SourceColumn.Valid && !m.DefaultValue.Valid && !m.ValueMap.Valid {
		return 0, fmt.Errorf("mapping must have at least a source column, default value, or value map")
	}

	query := `
		INSERT INTO sync_column_mappings (connection_id, table_name, source_column, dest_column, default_value, value_map)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, query, m.ConnectionID, m.TableName, m.SourceColumn, m.DestColumn, m.DefaultValue, m.ValueMap)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *MappingRepo) BulkInsert(ctx context.Context, ms []Mapping) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()

	query := `INSERT INTO sync_column_mappings (connection_id, table_name, source_column, dest_column, default_value, value_map) VALUES (?, ?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil { return err }
	defer stmt.Close()

	for _, m := range ms {
		if !m.SourceColumn.Valid && !m.DefaultValue.Valid && !m.ValueMap.Valid { continue }
		if _, err := stmt.ExecContext(ctx, m.ConnectionID, m.TableName, m.SourceColumn, m.DestColumn, m.DefaultValue, m.ValueMap); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *MappingRepo) ListByTable(ctx context.Context, connID int64, table string) ([]Mapping, error) {
	query := `
		SELECT id, connection_id, table_name, source_column, dest_column, default_value, value_map, created_at
		FROM sync_column_mappings
		WHERE connection_id = ? AND table_name = ?
		ORDER BY dest_column ASC
	`
	rows, err := r.db.QueryContext(ctx, query, connID, table)
	if err != nil { return nil, err }
	defer rows.Close()

	var mappings []Mapping
	for rows.Next() {
		var m Mapping
		if err := rows.Scan(&m.ID, &m.ConnectionID, &m.TableName, &m.SourceColumn, &m.DestColumn, &m.DefaultValue, &m.ValueMap, &m.CreatedAt); err != nil {
			return nil, err
		}
		mappings = append(mappings, m)
	}
	return mappings, nil
}

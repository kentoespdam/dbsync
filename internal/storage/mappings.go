package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/user/dbsync/internal/mysql"
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
	Mappings       []Mapping // mapping yang di-generate
	Warnings       []string  // dest NOT NULL kolom yang tidak ada di source dan tidak punya default
	UnmappedSource []string  // kolom source yang tidak ada di dest (informational)
}

func AutoMap(connID int64, table string, sourceCols, destCols []mysql.Column) AutoMapResult {
	var res AutoMapResult
	sourceMap := make(map[string]mysql.Column)
	for _, sc := range sourceCols {
		sourceMap[sc.Name] = sc
	}

	usedSource := make(map[string]bool)

	for _, dc := range destCols {
		if sc, ok := sourceMap[dc.Name]; ok {
			res.Mappings = append(res.Mappings, Mapping{
				ConnectionID: connID,
				TableName:    table,
				SourceColumn: sql.NullString{String: sc.Name, Valid: true},
				DestColumn:   dc.Name,
				DefaultValue: sql.NullString{Valid: false},
			})
			usedSource[sc.Name] = true
		} else {
			if !dc.IsNullable {
				res.Warnings = append(res.Warnings, fmt.Sprintf("dest column %s is NOT NULL but has no match in source", dc.Name))
			}
		}
	}

	for _, sc := range sourceCols {
		if !usedSource[sc.Name] {
			res.UnmappedSource = append(res.UnmappedSource, sc.Name)
		}
	}

	return res
}

type MappingRepo struct {
	db *sql.DB
}

func (d *DB) Mappings() *MappingRepo {
	return &MappingRepo{db: d.db}
}

func (r *MappingRepo) Insert(ctx context.Context, m Mapping) (int64, error) {
	if !m.SourceColumn.Valid && !m.DefaultValue.Valid {
		return 0, fmt.Errorf("mapping must have at least a source column or a default value")
	}

	query := `
		INSERT INTO sync_column_mappings (connection_id, table_name, source_column, dest_column, default_value)
		VALUES (?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, query, m.ConnectionID, m.TableName, m.SourceColumn, m.DestColumn, m.DefaultValue)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *MappingRepo) BulkInsert(ctx context.Context, ms []Mapping) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO sync_column_mappings (connection_id, table_name, source_column, dest_column, default_value)
		VALUES (?, ?, ?, ?, ?)
	`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range ms {
		if !m.SourceColumn.Valid && !m.DefaultValue.Valid {
			return fmt.Errorf("mapping for %s must have at least a source column or a default value", m.DestColumn)
		}
		_, err := stmt.ExecContext(ctx, m.ConnectionID, m.TableName, m.SourceColumn, m.DestColumn, m.DefaultValue)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *MappingRepo) ListByTable(ctx context.Context, connID int64, table string) ([]Mapping, error) {
	query := `
		SELECT id, connection_id, table_name, source_column, dest_column, default_value, created_at
		FROM sync_column_mappings
		WHERE connection_id = ? AND table_name = ?
		ORDER BY dest_column ASC
	`
	rows, err := r.db.QueryContext(ctx, query, connID, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mappings []Mapping
	for rows.Next() {
		var m Mapping
		err := rows.Scan(&m.ID, &m.ConnectionID, &m.TableName, &m.SourceColumn, &m.DestColumn, &m.DefaultValue, &m.CreatedAt)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, m)
	}
	return mappings, nil
}

func (r *MappingRepo) Upsert(ctx context.Context, m Mapping) error {
	if !m.SourceColumn.Valid && !m.DefaultValue.Valid {
		return fmt.Errorf("mapping must have at least a source column or a default value")
	}

	query := `
		INSERT INTO sync_column_mappings (connection_id, table_name, source_column, dest_column, default_value)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (connection_id, table_name, dest_column) DO UPDATE SET
			source_column = excluded.source_column,
			default_value = excluded.default_value
	`
	_, err := r.db.ExecContext(ctx, query, m.ConnectionID, m.TableName, m.SourceColumn, m.DestColumn, m.DefaultValue)
	return err
}

func (r *MappingRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM sync_column_mappings WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
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

func (r *MappingRepo) Exists(ctx context.Context, connID int64, table string) (bool, error) {
	query := `SELECT 1 FROM sync_column_mappings WHERE connection_id = ? AND table_name = ? LIMIT 1`
	var exists int
	err := r.db.QueryRowContext(ctx, query, connID, table).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

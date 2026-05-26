package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Connection struct {
	ID             int64
	Name           string
	SourceHost     string
	SourcePort     int
	SourceUser     string
	SourcePassword string // encrypted ciphertext (base64)
	SourceDB       string
	DestHost       string
	DestPort       int
	DestUser       string
	DestPassword   string // encrypted ciphertext (base64)
	DestDB         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ConnectionRepo struct {
	db *sql.DB
}

func (r *ConnectionRepo) Insert(ctx context.Context, c Connection) (int64, error) {
	query := `
		INSERT INTO connections (
			name, source_host, source_port, source_user, source_password, source_db,
			dest_host, dest_port, dest_user, dest_password, dest_db
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, query,
		c.Name, c.SourceHost, c.SourcePort, c.SourceUser, c.SourcePassword, c.SourceDB,
		c.DestHost, c.DestPort, c.DestUser, c.DestPassword, c.DestDB,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *ConnectionRepo) GetByName(ctx context.Context, name string) (Connection, error) {
	query := `SELECT * FROM connections WHERE name = ?`
	row := r.db.QueryRowContext(ctx, query, name)
	return r.scanConnection(row)
}

func (r *ConnectionRepo) GetByID(ctx context.Context, id int64) (Connection, error) {
	query := `SELECT * FROM connections WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanConnection(row)
}

func (r *ConnectionRepo) List(ctx context.Context) ([]Connection, error) {
	query := `SELECT * FROM connections ORDER BY name ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var connections []Connection
	for rows.Next() {
		c, err := r.scanConnection(rows)
		if err != nil {
			return nil, err
		}
		connections = append(connections, c)
	}
	return connections, nil
}

func (r *ConnectionRepo) Update(ctx context.Context, c Connection) error {
	query := `
		UPDATE connections SET
			name = ?, source_host = ?, source_port = ?, source_user = ?, source_password = ?, source_db = ?,
			dest_host = ?, dest_port = ?, dest_user = ?, dest_password = ?, dest_db = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		c.Name, c.SourceHost, c.SourcePort, c.SourceUser, c.SourcePassword, c.SourceDB,
		c.DestHost, c.DestPort, c.DestUser, c.DestPassword, c.DestDB,
		c.ID,
	)
	return err
}

func (r *ConnectionRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM connections WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *ConnectionRepo) scanConnection(scanner interface {
	Scan(dest ...interface{}) error
}) (Connection, error) {
	var c Connection
	err := scanner.Scan(
		&c.ID, &c.Name, &c.SourceHost, &c.SourcePort, &c.SourceUser, &c.SourcePassword, &c.SourceDB,
		&c.DestHost, &c.DestPort, &c.DestUser, &c.DestPassword, &c.DestDB,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Connection{}, ErrNotFound
		}
		return Connection{}, err
	}
	return c, nil
}

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Config represents the MySQL connection configuration.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string // plaintext
	DBName   string
}

// Pool wraps the *sql.DB connection pool.
type Pool struct {
	db *sql.DB
}

// Open initializes a MySQL connection pool with the given configuration.
// It applies best practices for connection pooling and verifies reachability.
func Open(cfg Config) (*Pool, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&timeout=30s&readTimeout=30s&writeTimeout=30s&charset=utf8mb4",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, redactError(err, cfg.Password)
	}

	// Pool tuning as per requirements
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify reachability with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, redactError(err, cfg.Password)
	}

	return &Pool{db: db}, nil
}

// Close closes the connection pool.
func (p *Pool) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// DB returns the underlying *sql.DB.
// Use this primarily for tests; prefer high-level functions in production code.
func (p *Pool) DB() *sql.DB {
	return p.db
}

// redactError ensures that passwords are not leaked in error messages.
func redactError(err error, password string) error {
	if err == nil || password == "" {
		return err
	}
	msg := err.Error()
	if strings.Contains(msg, password) {
		redacted := strings.ReplaceAll(msg, password, "***")
		return fmt.Errorf("%s", redacted)
	}
	return err
}

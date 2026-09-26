package app

import (
	"database/sql"
	"fmt"

	"cad-development/internal/db"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

type Database struct {
	Conn *sql.DB
	Q    *db.Queries
}

type DatabaseConfig struct {
	DSN           string
	MigrationsDir string
	MaxOpenConns  int
}

func OpenDatabase(config DatabaseConfig) (*Database, error) {
	if config.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}
	if config.MigrationsDir == "" {
		return nil, fmt.Errorf("database migrations directory is required")
	}
	conn, err := sql.Open("sqlite", config.DSN)
	if err != nil {
		return nil, err
	}
	if config.MaxOpenConns > 0 {
		conn.SetMaxOpenConns(config.MaxOpenConns)
	}
	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := goose.SetDialect("sqlite3"); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := goose.Up(conn, config.MigrationsDir); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Database{Conn: conn, Q: db.New(conn)}, nil
}

func (d *Database) Close() error {
	if d == nil || d.Conn == nil {
		return nil
	}
	return d.Conn.Close()
}

package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

type Database struct {
	Conn *sql.DB
	Q    *Queries
}

type DatabaseConfig struct {
	DSN          string
	Migrations   fs.FS
	MaxOpenConns int
}

func OpenDatabase(config DatabaseConfig) (*Database, error) {
	if config.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}
	if config.Migrations == nil {
		return nil, fmt.Errorf("database migrations are required")
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
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, config.Migrations)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, err := provider.Up(context.Background()); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Database{Conn: conn, Q: New(conn)}, nil
}

func (d *Database) Close() error {
	if d == nil || d.Conn == nil {
		return nil
	}
	return d.Conn.Close()
}

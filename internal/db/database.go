package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

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
		closeAndLog(conn)
		return nil, err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, config.Migrations)
	if err != nil {
		closeAndLog(conn)
		return nil, err
	}
	n, err := provider.Up(context.Background())
	if err != nil {
		closeAndLog(conn)
		return nil, err
	}
	if len(n) > 0 {
		slog.Info("applied migrations", "count", len(n))
	}
	return &Database{Conn: conn, Q: New(conn)}, nil
}

// closeAndLog closes a connection that will not be returned, surfacing the error
// without failing the caller's happy path.
func closeAndLog(conn *sql.DB) {
	if err := conn.Close(); err != nil {
		slog.Error("close database", "err", err)
	}
}

func (d *Database) Close() error {
	if d == nil || d.Conn == nil {
		return nil
	}
	return d.Conn.Close()
}

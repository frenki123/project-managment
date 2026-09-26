package dbtest

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cad-development/internal/db"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func Open(t *testing.T) (*sql.DB, *db.Queries) {
	t.Helper()
	name := strings.NewReplacer("/", "_", "\\", "_").Replace(t.Name())
	conn, err := sql.Open("sqlite", "file:"+name+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	conn.SetMaxOpenConns(1)
	if err := conn.Ping(); err != nil {
		t.Fatal(err)
	}
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(conn, filepath.Join(repoRoot(t), "sql", "migrations")); err != nil {
		t.Fatal(err)
	}
	return conn, db.New(conn)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 12 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("go.mod not found")
	return ""
}

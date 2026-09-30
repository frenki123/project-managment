package testkit

import (
	"io/fs"
	"strings"
	"testing"

	assets "cad-development"
	"cad-development/internal/db"
)

func Open(t *testing.T) *db.Queries {
	return OpenDatabase(t).Q
}

func OpenDatabase(t *testing.T) *db.Database {
	t.Helper()
	database, err := db.OpenDatabase(db.DatabaseConfig{
		DSN:          "file:" + strings.NewReplacer("/", "_", "\\", "_").Replace(t.Name()) + "?mode=memory&cache=shared&_pragma=foreign_keys(1)",
		Migrations:   embeddedMigrations(t),
		MaxOpenConns: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func embeddedMigrations(t *testing.T) fs.FS {
	t.Helper()
	migrations, err := fs.Sub(assets.FS, "sql/migrations")
	if err != nil {
		t.Fatal(err)
	}
	return migrations
}

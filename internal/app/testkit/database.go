package testkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cad-development/internal/app"
	"cad-development/internal/db"
)

func Open(t *testing.T) *db.Queries {
	return OpenDatabase(t).Q
}

func OpenDatabase(t *testing.T) *app.Database {
	t.Helper()
	name := strings.NewReplacer("/", "_", "\\", "_").Replace(t.Name())
	database, err := app.OpenDatabase(app.DatabaseConfig{
		DSN:           "file:" + name + "?mode=memory&cache=shared&_pragma=foreign_keys(1)",
		MigrationsDir: filepath.Join(repoRoot(t), "sql", "migrations"),
		MaxOpenConns:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
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

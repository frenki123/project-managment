package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"cad-development/internal/app"
)

func TestOpenDatabaseRunsMigrations(t *testing.T) {
	migrations := t.TempDir()
	if err := os.WriteFile(filepath.Join(migrations, "00001_test.sql"), []byte(`-- +goose Up
CREATE TABLE app_test (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE app_test;
`), 0o644); err != nil {
		t.Fatal(err)
	}

	database, err := app.OpenDatabase(app.DatabaseConfig{
		DSN:          "file:test-app?mode=memory&cache=shared",
		Migrations:   os.DirFS(migrations),
		MaxOpenConns: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if _, err := database.Conn.Exec("INSERT INTO app_test (id) VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.Conn.QueryRow("SELECT COUNT(*) FROM app_test").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("got row count %d", count)
	}
}

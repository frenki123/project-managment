package db_test

import (
	"os"
	"path/filepath"
	"testing"

	"cad-development/internal/db"
)

func TestOpenDatabaseRunsMigrations(t *testing.T) {
	migrations := t.TempDir()
	if err := os.WriteFile(filepath.Join(migrations, "00001_test.sql"), []byte(`-- +goose Up
CREATE TABLE app_test (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE app_test;
`), 0o600); err != nil {
		t.Fatal(err)
	}

	database, err := db.OpenDatabase(db.DatabaseConfig{
		DSN:          "file:test-app?mode=memory&cache=shared",
		Migrations:   os.DirFS(migrations),
		MaxOpenConns: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	result, err := database.Conn.Exec("INSERT INTO app_test (id) VALUES (1)")
	if err != nil {
		t.Fatal(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatal(err)
	}
	if affected != 1 {
		t.Fatalf("insert affected %d rows, want 1", affected)
	}
	var count int
	if err := database.Conn.QueryRow("SELECT COUNT(*) FROM app_test").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("got row count %d", count)
	}
}

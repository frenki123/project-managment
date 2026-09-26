package app_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"cad-development/internal/app/testkit"
)

func TestSQLiteConfigurationAndGridIndexes(t *testing.T) {
	database := testkit.OpenDatabase(t)
	ctx := t.Context()

	var foreignKeys int
	if err := database.Conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign keys are disabled: %d", foreignKeys)
	}

	assertPlanUsesIndex(t, database.Conn, `
		EXPLAIN QUERY PLAN
		SELECT id, name FROM subprojects
		WHERE project_id = ? ORDER BY name COLLATE NOCASE, id`, "idx_subprojects_project_name")
	assertPlanUsesIndex(t, database.Conn, `
		EXPLAIN QUERY PLAN
		SELECT id, name FROM tasks
		WHERE project_id = ? ORDER BY name COLLATE NOCASE, id`, "idx_tasks_project_name")
}

func assertPlanUsesIndex(t *testing.T, conn interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, query, index string) {
	t.Helper()
	rows, err := conn.QueryContext(t.Context(), query, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(detail, index) {
			return
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	t.Fatalf("query plan did not use %s", index)
}

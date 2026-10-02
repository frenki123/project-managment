package db_test

import (
	"errors"
	"testing"

	"cad-development/internal/db"
	"cad-development/internal/db/testkit"
)

type codeError struct {
	code int
	msg  string
}

func (e codeError) Error() string { return e.msg }
func (e codeError) Code() int     { return e.code }

func TestUniqueViolationMatchesExactColumn(t *testing.T) {
	err := codeError{code: 2067, msg: "UNIQUE constraint failed: projects.name, projects.id"}
	if !db.UniqueViolation(err, "projects.name") {
		t.Fatal("expected exact unique column match")
	}
	if db.UniqueViolation(err, "project.name") || db.UniqueViolation(codeError{code: 2067, msg: "UNIQUE constraint failed: subprojects.name"}, "projects.name") {
		t.Fatal("matched an unrelated unique constraint")
	}
}

func TestForeignKeyViolationUsesPrimaryCode(t *testing.T) {
	if !db.ForeignKeyViolation(codeError{code: 787}) {
		t.Fatal("expected foreign-key violation")
	}
	if db.ForeignKeyViolation(codeError{code: 19}) {
		t.Fatal("accepted the generic constraint code")
	}
	if db.ForeignKeyViolation(errors.New("foreign key")) {
		t.Fatal("accepted a non-SQLite error")
	}
}

func TestDriverUniqueConstraintCode(t *testing.T) {
	database := testkit.OpenDatabase(t)
	ctx := t.Context()
	result, err := database.Conn.ExecContext(ctx, `INSERT INTO projects (name, total_hours, start_date, end_date) VALUES (?, ?, ?, ?)`, "P", 1, "2026-01-05", "2026-01-05")
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
	duplicate, err := database.Conn.ExecContext(ctx, `INSERT INTO projects (name, total_hours, start_date, end_date) VALUES (?, ?, ?, ?)`, "P", 1, "2026-01-05", "2026-01-05")
	if !db.UniqueViolation(err, "projects.name") {
		t.Fatalf("expected real unique violation, got %v", err)
	}
	if duplicate != nil {
		t.Fatalf("failed insert returned a result: %#v", duplicate)
	}
	missing, err := database.Conn.ExecContext(ctx, `INSERT INTO tasks (name, project_id) VALUES (?, ?)`, "T", 999)
	if !db.ForeignKeyViolation(err) {
		t.Fatalf("expected real foreign-key violation, got %v", err)
	}
	if missing != nil {
		t.Fatalf("failed insert returned a result: %#v", missing)
	}
}

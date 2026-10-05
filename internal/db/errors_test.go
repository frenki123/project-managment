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

func TestCompositePrimaryKeyUniqueViolation(t *testing.T) {
	database := testkit.OpenDatabase(t)
	ctx := t.Context()
	if _, err := database.Conn.ExecContext(ctx, `INSERT INTO people (id, name, weekly_capacity) VALUES (1, 'Ada', 40)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Conn.ExecContext(ctx, `INSERT INTO tasks (id, name) VALUES (1, 'T')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Conn.ExecContext(ctx, `INSERT INTO task_developers (task_id, person_id) VALUES (1, 1)`); err != nil {
		t.Fatal(err)
	}
	_, err := database.Conn.ExecContext(ctx, `INSERT INTO task_developers (task_id, person_id) VALUES (1, 1)`)
	if !db.UniqueViolation(err, "task_developers.person_id") {
		t.Fatalf("expected composite primary-key violation, got %v", err)
	}
	var count int
	if err := database.Conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM task_developers").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duplicate insert persisted a row: task_developers=%d", count)
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
	_, err = database.Conn.ExecContext(ctx, `INSERT INTO projects (name, total_hours, start_date, end_date) VALUES (?, ?, ?, ?)`, "P", 1, "2026-01-05", "2026-01-05")
	if !db.UniqueViolation(err, "projects.name") {
		t.Fatalf("expected real unique violation, got %v", err)
	}
	var projectCount int
	if err := database.Conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects").Scan(&projectCount); err != nil {
		t.Fatal(err)
	}
	if projectCount != 1 {
		t.Fatalf("duplicate insert persisted a row: projects=%d", projectCount)
	}
	_, err = database.Conn.ExecContext(ctx, `INSERT INTO tasks (name, project_id) VALUES (?, ?)`, "T", 999)
	if !db.ForeignKeyViolation(err) {
		t.Fatalf("expected real foreign-key violation, got %v", err)
	}
	var taskCount int
	if err := database.Conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks").Scan(&taskCount); err != nil {
		t.Fatal(err)
	}
	if taskCount != 0 {
		t.Fatalf("foreign-key insert persisted a row: tasks=%d", taskCount)
	}
}

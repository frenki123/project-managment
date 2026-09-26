package subproject_test

import (
	"errors"
	"math"
	"net/http"
	"testing"

	"cad-development/internal/app"
	"cad-development/internal/app/testkit"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
)

func TestHoursCap(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: 10, StartDate: "2026-01-05", EndDate: "2026-02-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "A", TotalHours: 6}); err != nil {
		t.Fatal(err)
	}
	if _, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "B", TotalHours: 5}); err == nil {
		t.Fatal("expected hours cap")
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "B", TotalHours: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := subproject.Update(ctx, q, sp.ID, subproject.Input{ProjectID: p.ID, Name: "B", TotalHours: 5}); err == nil {
		t.Fatal("expected update cap")
	}
	got, err := subproject.Get(ctx, q, sp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalHours != 4 {
		t.Fatalf("rejected update changed hours: %v", got.TotalHours)
	}
	list, err := subproject.ListByProject(ctx, q, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("unexpected subproject count: %d", len(list))
	}
}

func TestHoursRejectNonFiniteValues(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: 10, StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "bad", TotalHours: value}); err == nil {
			t.Fatalf("expected non-finite value %v to be rejected", value)
		}
	}
}

func TestCannotMoveSubprojectWithTasks(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	first, err := project.Create(ctx, q, project.Input{Name: "First", TotalHours: 10, StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.Create(ctx, q, project.Input{Name: "Second", TotalHours: 10, StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: first.ID, Name: "Tracked", TotalHours: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.Create(ctx, q, task.Input{Name: "Task", ProjectID: new(first.ID), SubprojectID: new(sp.ID)}); err != nil {
		t.Fatal(err)
	}
	_, err = subproject.Update(ctx, q, sp.ID, subproject.Input{ProjectID: second.ID, Name: sp.Name, TotalHours: sp.TotalHours})
	var httpErr app.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusConflict {
		t.Fatalf("got %v", err)
	}
	got, err := subproject.Get(ctx, q, sp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID != first.ID {
		t.Fatalf("subproject moved after rejected update: %d", got.ProjectID)
	}
}

func TestCannotDeleteSubprojectWithTasks(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "Project", TotalHours: 10, StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "Tracked", TotalHours: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.Create(ctx, q, task.Input{Name: "Task", SubprojectID: &sp.ID}); err != nil {
		t.Fatal(err)
	}
	if err := subproject.Delete(ctx, q, sp.ID); err == nil {
		t.Fatal("expected delete with tasks to be rejected")
	}
	if _, err := subproject.Get(ctx, q, sp.ID); err != nil {
		t.Fatal("subproject was deleted after rejected delete")
	}
}

func TestDatabaseRejectsMismatchedTaskSubproject(t *testing.T) {
	ctx := t.Context()
	database := testkit.OpenDatabase(t)
	first, err := project.Create(ctx, database.Q, project.Input{Name: "First", TotalHours: 10, StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.Create(ctx, database.Q, project.Input{Name: "Second", TotalHours: 10, StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, database.Q, subproject.Input{ProjectID: first.ID, Name: "First subproject", TotalHours: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Conn.ExecContext(ctx, `INSERT INTO tasks (name, project_id, subproject_id) VALUES (?, ?, ?)`, "invalid", second.ID, sp.ID); err == nil {
		t.Fatal("expected mismatched task relationship to be rejected")
	}
}

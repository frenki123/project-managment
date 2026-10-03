package subproject_test

import (
	"errors"
	"math"
	"net/http"
	"testing"

	"cad-development/internal/db/testkit"
	"cad-development/internal/nullable"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/web"
)

func TestHoursCap(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	a, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "A", TotalHours: new(6.0)})
	if err != nil {
		t.Fatal(err)
	}
	if a.ProjectID != p.ID || a.Name != "A" || a.TotalHours != 6 {
		t.Fatalf("unexpected created subproject: %#v", a)
	}
	excess, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "B", TotalHours: new(5.0)})
	if err == nil {
		t.Fatal("expected hours cap")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Message != "subproject hours exceed project hours" {
		t.Fatalf("unexpected cap error: %v", err)
	}
	if excess != (subproject.Subproject{}) {
		t.Fatalf("rejected create returned a subproject: %#v", excess)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "B", TotalHours: new(4.0)})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := subproject.Update(ctx, q, sp.ID, subproject.Patch{
		ProjectID: nullable.Present(p.ID), Name: nullable.Present("B"), TotalHours: nullable.Present(5.0),
	})
	if err == nil {
		t.Fatal("expected update cap")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Message != "subproject hours exceed project hours" {
		t.Fatalf("unexpected cap error: %v", err)
	}
	if updated != (subproject.Subproject{}) {
		t.Fatalf("rejected update returned a subproject: %#v", updated)
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
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		created, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "bad", TotalHours: new(value)})
		if err == nil {
			t.Fatalf("expected non-finite value %v to be rejected", value)
		}
		if created != (subproject.Subproject{}) {
			t.Fatalf("rejected create returned a subproject: %#v", created)
		}
	}
}

func TestCannotMoveSubprojectWithTasks(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	first, err := project.Create(ctx, q, project.Input{Name: "First", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.Create(ctx, q, project.Input{Name: "Second", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: first.ID, Name: "Tracked", TotalHours: new(4.0)})
	if err != nil {
		t.Fatal(err)
	}
	item, err := task.Create(ctx, q, task.Input{Name: "Task", ProjectID: new(first.ID), SubprojectID: new(sp.ID)})
	if err != nil {
		t.Fatal(err)
	}
	if item.ProjectID == nil || *item.ProjectID != first.ID || item.SubprojectID == nil || *item.SubprojectID != sp.ID {
		t.Fatalf("unexpected task assignment: %#v", item)
	}
	moved, err := subproject.Update(ctx, q, sp.ID, subproject.Patch{
		ProjectID: nullable.Present(second.ID), Name: nullable.Present(sp.Name), TotalHours: nullable.Present(sp.TotalHours),
	})
	var httpErr web.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusConflict || httpErr.Message != "record is still used by other data" {
		t.Fatalf("got %v", err)
	}
	if moved != (subproject.Subproject{}) {
		t.Fatalf("rejected update returned a subproject: %#v", moved)
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
	p, err := project.Create(ctx, q, project.Input{Name: "Project", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "Tracked", TotalHours: new(1.0)})
	if err != nil {
		t.Fatal(err)
	}
	item, err := task.Create(ctx, q, task.Input{Name: "Task", SubprojectID: &sp.ID})
	if err != nil {
		t.Fatal(err)
	}
	if item.SubprojectID == nil || *item.SubprojectID != sp.ID {
		t.Fatalf("unexpected task assignment: %#v", item)
	}
	if err := subproject.Delete(ctx, q, sp.ID); err == nil {
		t.Fatal("expected delete with tasks to be rejected")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Message != "record is still used by other data" {
		t.Fatalf("unexpected delete error: %v", err)
	}
	kept, err := subproject.Get(ctx, q, sp.ID)
	if err != nil {
		t.Fatal("subproject was deleted after rejected delete")
	}
	if kept.ID != sp.ID {
		t.Fatalf("rejected delete returned a different subproject: %#v", kept)
	}
}

func TestDatabaseRejectsMismatchedTaskSubproject(t *testing.T) {
	ctx := t.Context()
	database := testkit.OpenDatabase(t)
	first, err := project.Create(ctx, database.Q, project.Input{Name: "First", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.Create(ctx, database.Q, project.Input{Name: "Second", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, database.Q, subproject.Input{ProjectID: first.ID, Name: "First subproject", TotalHours: new(1.0)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := database.Conn.ExecContext(ctx, `INSERT INTO tasks (name, project_id, subproject_id) VALUES (?, ?, ?)`, "invalid", second.ID, sp.ID)
	if err == nil {
		t.Fatal("expected mismatched task relationship to be rejected")
	}
	if result != nil {
		t.Fatalf("failed insert returned a result: %#v", result)
	}
}

package task_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/app/testkit"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestCreateValidatesNameAndSubprojectProject(t *testing.T) {
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
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: first.ID, Name: "First subproject", TotalHours: new(1.0)})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := task.Create(ctx, q, task.Input{Name: "   "}); err == nil {
		t.Fatal("expected blank task name to be rejected")
	}
	_, err = task.Create(ctx, q, task.Input{Name: "Wrong project", ProjectID: &second.ID, SubprojectID: &sp.ID})
	if httpErr, ok := errors.AsType[app.HTTPError](err); !ok || httpErr.Status != http.StatusBadRequest {
		t.Fatalf("got %v", err)
	}
}

func TestCreateSubprojectAdoptsItsProject(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "Project", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "Subproject", TotalHours: new(1.0)})
	if err != nil {
		t.Fatal(err)
	}
	item, err := task.Create(ctx, q, task.Input{Name: "Task", SubprojectID: &sp.ID})
	if err != nil {
		t.Fatal(err)
	}
	if item.ProjectID == nil || *item.ProjectID != p.ID {
		t.Fatalf("expected project %d, got %v", p.ID, item.ProjectID)
	}
}

func TestDeleteRejectsWeeklyHistory(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "Project", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "Task", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	hours := 1.0
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-01-05", weekly.Patch{PlannedHours: &hours}, time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), nil); err != nil {
		t.Fatal(err)
	}
	if err := task.Delete(ctx, q, tk.ID); err == nil {
		t.Fatal("expected delete with weekly history to be rejected")
	}
	if _, err := task.Get(ctx, q, tk.ID); err != nil {
		t.Fatal("task was deleted after rejected delete")
	}
}

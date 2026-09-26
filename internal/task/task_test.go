package task_test

import (
	"errors"
	"testing"

	"cad-development/internal/app/testkit"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
)

func TestCreateValidatesNameAndSubprojectProject(t *testing.T) {
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
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: first.ID, Name: "First subproject", TotalHours: 1})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := task.Create(ctx, q, task.Input{Name: "   "}); err == nil {
		t.Fatal("expected blank task name to be rejected")
	}
	_, err = task.Create(ctx, q, task.Input{Name: "Wrong project", ProjectID: &second.ID, SubprojectID: &sp.ID})
	var domainErr task.Error
	if !errors.As(err, &domainErr) || domainErr.Kind != task.InvalidInput {
		t.Fatalf("got %v", err)
	}
}

func TestCreateSubprojectAdoptsItsProject(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "Project", TotalHours: 10, StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "Subproject", TotalHours: 1})
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

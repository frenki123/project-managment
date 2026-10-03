package task_test

import (
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"cad-development/internal/db"
	"cad-development/internal/db/testkit"
	"cad-development/internal/nullable"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/web"
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

	if _, err := task.Create(ctx, q, task.Input{Name: "   "}); err == nil { //nolint:droppedvalue -- the error-return task is uninteresting; the unchanged task list is asserted via List below
		t.Fatal("expected blank task name to be rejected")
	}
	_, err = task.Create(ctx, q, task.Input{Name: "Wrong project", ProjectID: &second.ID, SubprojectID: &sp.ID}) //nolint:droppedvalue -- the error-return task is uninteresting; the unchanged task list is asserted via List below
	if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Status != http.StatusBadRequest {
		t.Fatalf("got %v", err)
	}
	items, err := task.List(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("rejected creates persisted tasks: %#v", items)
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

func TestUpdateCannotClearProjectWithSubproject(t *testing.T) {
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
	taskInput := task.Input{Name: "Task", ProjectID: &p.ID, SubprojectID: &sp.ID}
	item, err := task.Create(ctx, q, taskInput)
	if err != nil {
		t.Fatal(err)
	}
	_, err = task.Update(ctx, q, item.ID, task.Patch{ProjectID: nullable.Clear[*int64]()}) //nolint:droppedvalue -- the error-return value is uninteresting; the unchanged data is asserted via Get below
	if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Status != http.StatusBadRequest {
		t.Fatalf("got %v", err)
	}
	updated, err := task.Get(ctx, q, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProjectID == nil || *updated.ProjectID != p.ID || updated.SubprojectID == nil || *updated.SubprojectID != sp.ID {
		t.Fatalf("assignment changed after rejected update: project=%v subproject=%v", updated.ProjectID, updated.SubprojectID)
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
	cell, err := weekly.Save(ctx, q, tk.ID, testkit.MustWeek(t, "2026-01-05"), weekly.Patch{PlannedHours: nullable.Present(hours)}, time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != hours {
		t.Fatalf("save returned unexpected planned hours: %#v", cell)
	}
	if err := task.Delete(ctx, q, tk.ID); err == nil {
		t.Fatal("expected delete with weekly history to be rejected")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Message != "record is still used by other data" {
		t.Fatalf("unexpected delete error: %v", err)
	}
	kept, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal("task was deleted after rejected delete")
	}
	if kept.ID != tk.ID {
		t.Fatalf("rejected delete returned a different task: %#v", kept)
	}
}

func TestListTaskTotalsOnlyAggregatesRequestedScope(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	first, err := project.Create(ctx, q, project.Input{Name: "First", TotalHours: new(10.0), StartDate: "2026-09-07", EndDate: "2026-09-28"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.Create(ctx, q, project.Input{Name: "Second", TotalHours: new(10.0), StartDate: "2026-09-07", EndDate: "2026-09-28"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{Name: "Part", ProjectID: first.ID, TotalHours: new(5.0)})
	if err != nil {
		t.Fatal(err)
	}
	idea, err := task.Create(ctx, q, task.Input{Name: "Idea"})
	if err != nil {
		t.Fatal(err)
	}
	inPart, err := task.Create(ctx, q, task.Input{Name: "Part task", ProjectID: &first.ID, SubprojectID: &sp.ID})
	if err != nil {
		t.Fatal(err)
	}
	inSecond, err := task.Create(ctx, q, task.Input{Name: "Other task", ProjectID: &second.ID})
	if err != nil {
		t.Fatal(err)
	}
	hours := 4.0
	cell, err := weekly.Save(ctx, q, inPart.ID, testkit.MustWeek(t, "2026-09-07"), weekly.Patch{PlannedHours: nullable.Present(hours)}, time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != hours {
		t.Fatalf("save returned unexpected planned hours: %#v", cell)
	}
	for _, tc := range []struct {
		name string
		load func() ([]db.VTaskTotal, error)
		ids  []int64
	}{
		{"ideas", func() ([]db.VTaskTotal, error) { return q.ListTaskTotalsIdeas(ctx) }, []int64{idea.ID}},
		{"project", func() ([]db.VTaskTotal, error) {
			return q.ListTaskTotalsScoped(ctx, db.ListTaskTotalsScopedParams{ProjectID: nullable.Int64(&first.ID)})
		}, []int64{inPart.ID}},
		{"subproject", func() ([]db.VTaskTotal, error) {
			return q.ListTaskTotalsScoped(ctx, db.ListTaskTotalsScopedParams{SubprojectID: nullable.Int64(&sp.ID)})
		}, []int64{inPart.ID}},
		{"all", func() ([]db.VTaskTotal, error) { return q.ListTaskTotalsScoped(ctx, db.ListTaskTotalsScopedParams{}) }, []int64{idea.ID, inPart.ID, inSecond.ID}},
	} {
		rows, err := tc.load()
		if err != nil || len(rows) != len(tc.ids) {
			t.Fatalf("%s totals: %v %v", tc.name, rows, err)
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.TaskID)
			if row.TaskID == inPart.ID && row.PlannedHours != hours {
				t.Fatalf("%s lost calculated hours: %v", tc.name, rows)
			}
		}
		slices.Sort(ids)
		if !slices.Equal(ids, tc.ids) {
			t.Fatalf("%s included unrelated tasks: %v", tc.name, ids)
		}
	}
}

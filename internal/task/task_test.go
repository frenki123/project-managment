package task_test

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"cad-development/internal/db"
	"cad-development/internal/db/testkit"
	"cad-development/internal/nullable"
	"cad-development/internal/person"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/web"
	"cad-development/internal/weekly"
)

func TestLoadWeeksUsesProjectBoundedSeriesAndPreservesStoredProgress(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "Bounded", TotalHours: new(10.0), StartDate: "2026-09-09", EndDate: "2026-09-22"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "Tracked", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.Save(ctx, q, tk.ID, testkit.MustWeek(t, "2026-09-07"), weekly.Patch{Progress: nullable.Present(40.0)}, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	got, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Weeks) != 3 {
		t.Fatalf("got %d project weeks, want 3: %#v", len(got.Weeks), got.Weeks)
	}
	if got.Weeks[0].StoredProgress == nil || *got.Weeks[0].StoredProgress != 40 || got.Weeks[1].StoredProgress != nil || got.Weeks[1].Progress == nil || *got.Weeks[1].Progress != 40 {
		t.Fatalf("unexpected stored/effective progress: %#v", got.Weeks)
	}
}

func TestMapWeekSeriesRejectsMalformedDates(t *testing.T) {
	_, err := weekly.MapWeekSeries([]db.VTaskWeekSeries{{WeekStart: "not-a-date"}})
	if err == nil {
		t.Fatal("expected malformed week date to fail")
	}
}

func TestMapWeekSeriesDistinguishesZeroStoredProgress(t *testing.T) {
	rows, err := weekly.MapWeekSeries([]db.VTaskWeekSeries{{TaskID: 1, WeekStart: "2026-09-07", StoredProgress: sql.NullFloat64{Valid: true, Float64: 0}}})
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].StoredProgress == nil || *rows[0].StoredProgress != 0 {
		t.Fatalf("zero stored progress was lost: %#v", rows[0])
	}
}

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
	_, err = task.Update(ctx, q, item.ID, task.Patch{ProjectID: nullable.Clear[*int64]()})
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
	if _, err := weekly.Save(ctx, q, tk.ID, testkit.MustWeek(t, "2026-01-05"), weekly.Patch{PlannedHours: nullable.Present(hours)}, time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
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

func TestDeleteTaskWithDevelopersSucceeds(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "Project", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	dev, err := person.Create(ctx, q, person.Input{Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "Task", ProjectID: &p.ID, DeveloperIDs: []int64{dev.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Delete(ctx, q, tk.ID); err != nil {
		t.Fatalf("delete with developers failed: %v", err)
	}
	_, err = task.Get(ctx, q, tk.ID)
	if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Status != http.StatusNotFound || httpErr.Message != "task not found" {
		t.Fatalf("deleted task still readable: %v", err)
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
	if _, err := weekly.Save(ctx, q, inPart.ID, testkit.MustWeek(t, "2026-09-07"), weekly.Patch{PlannedHours: nullable.Present(hours)}, time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
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

func TestCreateAssignsDevelopers(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := person.Create(ctx, q, person.Input{Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "Task", DeveloperIDs: []int64{p.ID}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Developers) != 1 || got.Developers[0].ID != p.ID || got.Developers[0].Name != "Ada" {
		t.Fatalf("unexpected developers: %#v", got.Developers)
	}
}

func TestCreateRejectsUnknownDeveloper(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	_, err := task.Create(ctx, q, task.Input{Name: "Task", DeveloperIDs: []int64{999}})
	if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Status != http.StatusNotFound || httpErr.Message != "person not found" {
		t.Fatalf("expected person not found, got %v", err)
	}
	items, err := task.List(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("rejected create persisted a task: %#v", items)
	}
}

func TestCreateRejectsDuplicateDeveloper(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := person.Create(ctx, q, person.Input{Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = task.Create(ctx, q, task.Input{Name: "Task", DeveloperIDs: []int64{p.ID, p.ID}})
	var httpErr web.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadRequest || httpErr.Message != "duplicate developer" {
		t.Fatalf("expected duplicate developer, got %v", err)
	}
	items, err := task.List(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("rejected create persisted a task: %#v", items)
	}
}

func TestUpdateDeveloperReplaceClearPreserve(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	a, err := person.Create(ctx, q, person.Input{Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := person.Create(ctx, q, person.Input{Name: "Grace"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "Task", DeveloperIDs: []int64{a.ID}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := task.Update(ctx, q, tk.ID, task.Patch{DeveloperIDs: nullable.Present([]int64{b.ID})})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Developers) != 1 || got.Developers[0].ID != b.ID {
		t.Fatalf("replace failed: %#v", got.Developers)
	}
	got, err = task.Update(ctx, q, tk.ID, task.Patch{Name: nullable.Present("Renamed")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Renamed" || len(got.Developers) != 1 || got.Developers[0].ID != b.ID {
		t.Fatalf("name-only update changed developers: %#v", got)
	}
	got, err = task.Update(ctx, q, tk.ID, task.Patch{DeveloperIDs: nullable.Clear[[]int64]()})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Developers) != 0 {
		t.Fatalf("clear failed: %#v", got.Developers)
	}
}

func TestListResolvesDevelopers(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	ada, err := person.Create(ctx, q, person.Input{Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	grace, err := person.Create(ctx, q, person.Input{Name: "Grace"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := task.Create(ctx, q, task.Input{Name: "First", DeveloperIDs: []int64{ada.ID}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := task.Create(ctx, q, task.Input{Name: "Second", DeveloperIDs: []int64{ada.ID, grace.ID}})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := task.Create(ctx, q, task.Input{Name: "Empty"})
	if err != nil {
		t.Fatal(err)
	}
	items, err := task.List(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d tasks: %#v", len(items), items)
	}
	byID := make(map[int64]task.Task, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	if got := byID[first.ID]; len(got.Developers) != 1 || got.Developers[0].Name != "Ada" {
		t.Fatalf("first developers: %#v", got.Developers)
	}
	if got := byID[second.ID]; len(got.Developers) != 2 {
		t.Fatalf("second developers: %#v", got.Developers)
	}
	if got := byID[empty.ID]; got.Developers == nil {
		t.Fatalf("empty developers should be a non-nil slice: %#v", got.Developers)
	}
}

func TestDeletePersonReferencedByTaskIsConflict(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := person.Create(ctx, q, person.Input{Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.Create(ctx, q, task.Input{Name: "Task", DeveloperIDs: []int64{p.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := person.Delete(ctx, q, p.ID); err == nil {
		t.Fatal("expected delete to be rejected")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Message != "record is still used by other data" {
		t.Fatalf("unexpected delete error: %v", err)
	}
	got, err := person.Get(ctx, q, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != p.ID {
		t.Fatalf("rejected delete removed the person: %#v", got)
	}
}

func TestGetIncludesWeekAttributions(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(100.0), StartDate: "2026-01-05", EndDate: "2026-06-01"})
	if err != nil {
		t.Fatal(err)
	}
	dev, err := person.Create(ctx, q, person.Input{Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID, DeveloperIDs: []int64{dev.ID}})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	planned, spent := 8.0, 3.0
	if _, err := weekly.Save(ctx, q, tk.ID, testkit.MustWeek(t, "2026-01-05"), weekly.Patch{PlannedHours: nullable.Present(planned), SpentHours: nullable.Present(spent)}, now); err != nil {
		t.Fatal(err)
	}
	got, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, week := range got.Weeks {
		if week.WeekStart.String() == "2026-01-05" {
			found = true
			if len(week.Attributions) != 1 || week.Attributions[0].PersonID != dev.ID || week.Attributions[0].Name != "Ada" || week.Attributions[0].PlannedHours != planned || week.Attributions[0].SpentHours != spent {
				t.Fatalf("unexpected attributions: %#v", week.Attributions)
			}
		}
	}
	if !found {
		t.Fatalf("missing attributed week: %#v", got.Weeks)
	}
}

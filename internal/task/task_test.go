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

func TestTaskStageAutoResolution(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "Staged", TotalHours: new(10.0), StartDate: "2026-09-07", EndDate: "2026-10-05"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "Task", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	assertStatus := func(want string) {
		t.Helper()
		got, err := task.Get(ctx, q, tk.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != want {
			t.Fatalf("status = %q, want %q", got.Status, want)
		}
	}
	assertStatus("Planned")
	for _, step := range []struct {
		week     string
		progress float64
		want     string
	}{
		{"2026-09-07", 50, "In progress"},
		{"2026-09-14", 90, "In review"},
		{"2026-09-21", 100, "Done"},
	} {
		if _, err := weekly.Save(ctx, q, tk.ID, testkit.MustWeek(t, step.week), weekly.Patch{Progress: nullable.Present(step.progress)}, now); err != nil {
			t.Fatal(err)
		}
		assertStatus(step.want)
	}
}

func TestTaskManualStatusOverrideAndReset(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "Manual", TotalHours: new(10.0), StartDate: "2026-09-07", EndDate: "2026-09-28"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "Task", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	if _, err := weekly.Save(ctx, q, tk.ID, testkit.MustWeek(t, "2026-09-07"), weekly.Patch{Progress: nullable.Present(50.0)}, now); err != nil {
		t.Fatal(err)
	}
	got, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "In progress" {
		t.Fatalf("derived status = %q, want %q", got.Status, "In progress")
	}
	updated, err := task.Update(ctx, q, tk.ID, task.Patch{ManualStatus: nullable.Present(new("On hold"))})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ManualStatus == nil || *updated.ManualStatus != "On hold" || updated.Status != "On hold" {
		t.Fatalf("manual override = %#v, status = %q, want On hold", updated.ManualStatus, updated.Status)
	}
	updated, err = task.Update(ctx, q, tk.ID, task.Patch{ManualStatus: nullable.Clear[*string]()})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ManualStatus != nil || updated.Status != "In progress" {
		t.Fatalf("after reset manual = %#v, status = %q, want derived In progress", updated.ManualStatus, updated.Status)
	}
}

func TestTaskManualOnlyStageNotReachedByProgress(t *testing.T) {
	database := testkit.OpenDatabase(t)
	q := database.Q
	ctx := t.Context()
	// 'Done' stays the max stage (position 10, threshold 100), so a manual-only
	// middle stage may carry any threshold without breaking the pins.
	if _, err := database.Conn.Exec("UPDATE stages SET position = 10 WHERE name = 'Done'"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Conn.Exec("INSERT INTO stages (name, position, color, auto_reachable, progress_threshold) VALUES ('Blocked', 5, '', 0, 90)"); err != nil {
		t.Fatal(err)
	}
	p, err := project.Create(ctx, q, project.Input{Name: "Blocked", TotalHours: new(10.0), StartDate: "2026-09-07", EndDate: "2026-09-28"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "Task", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	if _, err := weekly.Save(ctx, q, tk.ID, testkit.MustWeek(t, "2026-09-07"), weekly.Patch{Progress: nullable.Present(100.0)}, now); err != nil {
		t.Fatal(err)
	}
	got, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "Done" {
		t.Fatalf("100%% progress resolved to %q, want %q (manual-only stage must not be auto-reachable)", got.Status, "Done")
	}
	updated, err := task.Update(ctx, q, tk.ID, task.Patch{ManualStatus: nullable.Present(new("Blocked"))})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ManualStatus == nil || *updated.ManualStatus != "Blocked" || updated.Status != "Blocked" {
		t.Fatalf("manual move = %#v, status = %q, want Blocked", updated.ManualStatus, updated.Status)
	}
}

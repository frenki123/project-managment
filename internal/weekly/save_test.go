package weekly_test

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"testing"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/app/testkit"
	"cad-development/internal/db"
	"cad-development/internal/nullable"
	"cad-development/internal/project"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestSaveProgressAndLock(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: new(100.0), StartDate: "2026-01-05", EndDate: "2026-06-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	pid := p.ID
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &pid})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	_, err = weekly.Save(ctx, q, tk.ID, "2026-04-27", weekly.Patch{Progress: nullable.Present(40.0)}, now)
	if err == nil {
		t.Fatal("expected locked April week to fail")
	}
	if _, err = weekly.Save(ctx, q, tk.ID, "2026-04-27", weekly.Patch{Progress: nullable.Present(40.0), Unlock: true}, now); err != nil {
		t.Fatal(err)
	}
	cell, err := weekly.Save(ctx, q, tk.ID, "2026-05-04", weekly.Patch{Progress: nullable.Present(30.0)}, now)
	if err != nil || cell.Progress == nil || *cell.Progress != 40 {
		t.Fatalf("below-carried progress should carry effective 40: %#v %v", cell, err)
	}
	row, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tk.ID, WeekStart: "2026-05-04"})
	if err != nil || row.Progress.Valid {
		t.Fatalf("below-carried progress should store NULL: %#v %v", row, err)
	}
	if _, err = weekly.Save(ctx, q, tk.ID, "2026-05-04", weekly.Patch{Progress: nullable.Present(50.0)}, now); err != nil {
		t.Fatal(err)
	}
	cell, err = weekly.Save(ctx, q, tk.ID, "2026-05-11", weekly.Patch{Progress: nullable.Present(45.0)}, now)
	if err != nil || cell.Progress == nil || *cell.Progress != 50 {
		t.Fatalf("below-carried progress should carry effective 50: %#v %v", cell, err)
	}
	row, err = q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tk.ID, WeekStart: "2026-05-11"})
	if err != nil || row.Progress.Valid {
		t.Fatalf("below-carried progress should store NULL: %#v %v", row, err)
	}
	idea, err := task.Create(ctx, q, task.Input{Name: "Idea"})
	if err != nil {
		t.Fatal(err)
	}
	h := 1.0
	if _, err = weekly.Save(ctx, q, idea.ID, "2026-05-04", weekly.Patch{PlannedHours: nullable.Present(h)}, now); err == nil {
		t.Fatal("ideas cannot be planned")
	}
}

func TestSaveRequestUnlockDoesNotPersist(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(10.0), StartDate: "2026-04-06", EndDate: "2026-05-04"})
	if err != nil {
		t.Fatal(err)
	}
	taskID := p.ID
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &taskID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	hours := 1.0
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-04-06", weekly.Patch{PlannedHours: nullable.Present(hours), Unlock: true}, now); err != nil {
		t.Fatalf("historical edit with request access was rejected: %v", err)
	}
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-04-06", weekly.Patch{PlannedHours: nullable.Present(hours)}, now); err == nil {
		t.Fatal("historical unlock should not persist")
	}
}

func TestCascadeProgressAndRelock(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: new(100.0), StartDate: "2026-03-02", EndDate: "2026-06-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-03-30", weekly.Patch{Progress: nullable.Present(20.0), Unlock: true}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-04-06", weekly.Patch{Progress: nullable.Present(40.0), Unlock: true}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-05-04", weekly.Patch{Progress: nullable.Present(70.0)}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-03-30", weekly.Patch{Progress: nullable.Present(20.0), Unlock: true}, now); err != nil {
		t.Fatalf("re-storing the carried value should be allowed: %v", err)
	}
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-03-30", weekly.Patch{Progress: nullable.Present(50.0), Unlock: true}, now); err != nil {
		t.Fatal(err)
	}
	cleared, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tk.ID, WeekStart: "2026-04-06"})
	if err != nil || cleared.Progress.Valid {
		t.Fatalf("raising an earlier week should clear the stored 40 in a later week: %#v %v", cleared, err)
	}
	result, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Weeks) < 2 {
		t.Fatalf("expected at least 2 weeks, got %#v", result.Weeks)
	}
	if result.Weeks[1].Progress == nil || *result.Weeks[1].Progress != 50 {
		t.Fatalf("later progress should cascade to 50, got %#v", result.Weeks)
	}
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-03-30", weekly.Patch{Progress: nullable.Present(60.0)}, now); err == nil {
		t.Fatal("relocked March should reject edits")
	}
	keepEarlier, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tk.ID, WeekStart: "2026-03-30"})
	if err != nil || !keepEarlier.Progress.Valid || keepEarlier.Progress.Float64 != 50 {
		t.Fatalf("rejected edit changed stored earlier progress: %#v %v", keepEarlier, err)
	}
	keepLater, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tk.ID, WeekStart: "2026-05-04"})
	if err != nil || !keepLater.Progress.Valid || keepLater.Progress.Float64 != 70 {
		t.Fatalf("rejected edit changed stored later progress: %#v %v", keepLater, err)
	}
	filter, err := task.ParseFilter(strconv.FormatInt(p.ID, 10), "")
	if err != nil {
		t.Fatal(err)
	}
	grid, err := task.LoadGrid(ctx, q, filter, now, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range grid.Rows[0].Cells {
		if cell.WeekStart == "2026-04-06" {
			if cell.Progress != 50 || cell.Stored || !cell.Locked {
				t.Fatalf("locked April carry changed: %#v", cell)
			}
			return
		}
	}
	t.Fatal("missing displayed April week")
}

func TestSaveRejectsInvalidPatches(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: new(100.0), StartDate: "2026-01-05", EndDate: "2026-06-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	taskID := p.ID
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &taskID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	values := []struct {
		name        string
		patch       weekly.Patch
		week        string
		wantMessage string
	}{
		{"empty patch", weekly.Patch{}, "2026-01-05", "at least one value is required"},
		{"negative hours", weekly.Patch{PlannedHours: nullable.Present(-1.0)}, "2026-01-05", "hours cannot be negative"},
		{"infinite hours", weekly.Patch{SpentHours: nullable.Present(math.Inf(1))}, "2026-01-05", "hours cannot be negative"},
		{"progress above 100", weekly.Patch{Progress: nullable.Present(101.0)}, "2026-01-05", "progress must be between 0 and 100"},
		{"non-Monday", weekly.Patch{PlannedHours: nullable.Present(1.0)}, "2026-01-06", "week_start must be a Monday"},
		{"outside project range", weekly.Patch{PlannedHours: nullable.Present(1.0)}, "2026-06-08", "week is outside the project date range"},
	}
	for _, tc := range values {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := weekly.Save(ctx, q, tk.ID, weekly.WeekStart(tc.week), tc.patch, now); err == nil {
				t.Fatal("expected invalid patch to fail")
			} else {
				var httpErr app.HTTPError
				if !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadRequest || httpErr.Message != tc.wantMessage {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
	result, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Weeks) != 0 {
		t.Fatalf("invalid patches should not create weekly data, got %#v", result.Weeks)
	}
}

func TestSavePartialPatchPreservesExistingValues(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(10.0), StartDate: "2026-09-01", EndDate: "2026-09-30"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	planned, spent, progress := 8.0, 3.0, 25.0
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-09-07", weekly.Patch{PlannedHours: nullable.Present(planned), SpentHours: nullable.Present(spent), Progress: nullable.Present(progress)}, now); err != nil {
		t.Fatal(err)
	}
	updatedSpent := 5.0
	cell, err := weekly.Save(ctx, q, tk.ID, "2026-09-07", weekly.Patch{SpentHours: nullable.Present(updatedSpent)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != planned || cell.SpentHours != updatedSpent || cell.Progress == nil || *cell.Progress != progress {
		t.Fatalf("partial patch changed untouched values: %#v", cell)
	}
	nextHours := 2.0
	cell, err = weekly.Save(ctx, q, tk.ID, "2026-09-14", weekly.Patch{PlannedHours: nullable.Present(nextHours)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != progress {
		t.Fatalf("carried progress should be effective in save response: %#v", cell)
	}
}

func TestClearProgressRestoresCarryForwardWithoutChangingHours(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(100.0), StartDate: "2026-03-02", EndDate: "2026-04-27"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	p20, p50, p70, hours := 20.0, 50.0, 70.0, 3.0
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-03-02", weekly.Patch{Progress: nullable.Present(p20), Unlock: true}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-03-30", weekly.Patch{Progress: nullable.Present(p50), PlannedHours: nullable.Present(hours), Unlock: true}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-04-06", weekly.Patch{Progress: nullable.Present(p70), Unlock: true}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-03-30", weekly.Patch{Progress: nullable.Clear[float64]()}, now); err == nil {
		t.Fatal("clear should not change relocked history")
	}
	cell, err := weekly.Save(ctx, q, tk.ID, "2026-03-30", weekly.Patch{Progress: nullable.Clear[float64](), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != hours || cell.Progress == nil || *cell.Progress != p20 {
		t.Fatalf("clear did not carry forward while preserving hours: %#v", cell)
	}
	row, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tk.ID, WeekStart: "2026-03-30"})
	if err != nil || row.Progress.Valid {
		t.Fatalf("explicit progress was not cleared: %#v %v", row, err)
	}
	later, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tk.ID, WeekStart: "2026-04-06"})
	if err != nil || !later.Progress.Valid || later.Progress.Float64 != p70 {
		t.Fatalf("clearing earlier progress changed later explicit progress: %#v %v", later, err)
	}
}

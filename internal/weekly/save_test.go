package weekly_test

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"testing"
	"time"

	"cad-development/internal/db"
	"cad-development/internal/db/testkit"
	"cad-development/internal/nullable"
	"cad-development/internal/person"
	"cad-development/internal/project"
	"cad-development/internal/task"
	"cad-development/internal/web"
	"cad-development/internal/weekly"
)

func TestSaveProgressAndLock(t *testing.T) {
	ctx := t.Context()
	q, pid, tkID := newProjectTask(t, 100.0, "2026-01-05", "2026-06-01")
	if pid == 0 {
		t.Fatal("expected a project ID")
	}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	if _, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-04-27"), weekly.Patch{Progress: nullable.Present(40.0)}, now); err == nil {
		t.Fatal("expected locked April week to fail")
	}
	row, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tkID, WeekStart: "2026-04-27"})
	if err == nil {
		t.Fatalf("locked save persisted a week: %#v", row)
	}
	cell, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-04-27"), weekly.Patch{Progress: nullable.Present(40.0), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != 40 {
		t.Fatalf("unlocked April save should store effective 40: %#v", cell)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-05-04"), weekly.Patch{Progress: nullable.Present(30.0)}, now)
	if err != nil || cell.Progress == nil || *cell.Progress != 40 {
		t.Fatalf("below-carried progress should carry effective 40: %#v %v", cell, err)
	}
	row, err = q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tkID, WeekStart: "2026-05-04"})
	if err != nil || row.Progress.Valid {
		t.Fatalf("below-carried progress should store NULL: %#v %v", row, err)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-05-04"), weekly.Patch{Progress: nullable.Present(50.0)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != 50 {
		t.Fatalf("explicit 50 should store effective 50: %#v", cell)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-05-11"), weekly.Patch{Progress: nullable.Present(45.0)}, now)
	if err != nil || cell.Progress == nil || *cell.Progress != 50 {
		t.Fatalf("below-carried progress should carry effective 50: %#v %v", cell, err)
	}
	row, err = q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tkID, WeekStart: "2026-05-11"})
	if err != nil || row.Progress.Valid {
		t.Fatalf("below-carried progress should store NULL: %#v %v", row, err)
	}
	idea, err := task.Create(ctx, q, task.Input{Name: "Idea"})
	if err != nil {
		t.Fatal(err)
	}
	h := 1.0
	if _, err := weekly.Save(ctx, q, idea.ID, testkit.MustWeek(t, "2026-05-04"), weekly.Patch{PlannedHours: nullable.Present(h)}, now); err == nil {
		t.Fatal("ideas cannot be planned")
	}
	result, err := task.Get(ctx, q, idea.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Weeks) != 0 {
		t.Fatalf("rejected idea plan persisted a week: %#v", result.Weeks)
	}
}

func TestSaveRequestUnlockDoesNotPersist(t *testing.T) {
	ctx := t.Context()
	q, pid, tkID := newProjectTask(t, 10.0, "2026-04-06", "2026-05-04")
	if pid == 0 {
		t.Fatal("expected a project ID")
	}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	hours := 1.0
	if _, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-04-06"), weekly.Patch{PlannedHours: nullable.Present(hours), Unlock: true}, now); err != nil {
		t.Fatalf("historical edit with request access was rejected: %v", err)
	}
	changedHours := 2.0
	if _, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-04-06"), weekly.Patch{PlannedHours: nullable.Present(changedHours)}, now); err == nil {
		t.Fatal("historical unlock should not persist")
	}
	row, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tkID, WeekStart: "2026-04-06"})
	if err != nil {
		t.Fatal(err)
	}
	if row.PlannedHours != hours {
		t.Fatalf("relocked save changed stored planned hours to %v, want %v", row.PlannedHours, hours)
	}
}

func TestCascadeProgressAndRelock(t *testing.T) {
	ctx := t.Context()
	q, pid, tkID := newProjectTask(t, 100.0, "2026-03-02", "2026-06-01")
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	cell, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-03-30"), weekly.Patch{Progress: nullable.Present(20.0), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != 20 {
		t.Fatalf("March save should store effective 20: %#v", cell)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-04-06"), weekly.Patch{Progress: nullable.Present(40.0), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != 40 {
		t.Fatalf("April save should store effective 40: %#v", cell)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-05-04"), weekly.Patch{Progress: nullable.Present(70.0)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != 70 {
		t.Fatalf("May save should store effective 70: %#v", cell)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-03-30"), weekly.Patch{Progress: nullable.Present(20.0), Unlock: true}, now)
	if err != nil {
		t.Fatalf("re-storing the carried value should be allowed: %v", err)
	}
	if cell.Progress == nil || *cell.Progress != 20 {
		t.Fatalf("re-storing the carried value should keep effective 20: %#v", cell)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-03-30"), weekly.Patch{Progress: nullable.Present(50.0), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != 50 {
		t.Fatalf("raising earlier progress should store effective 50: %#v", cell)
	}
	cleared, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tkID, WeekStart: "2026-04-06"})
	if err != nil || cleared.Progress.Valid {
		t.Fatalf("raising an earlier week should clear the stored 40 in a later week: %#v %v", cleared, err)
	}
	result, err := task.Get(ctx, q, tkID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Weeks) < 2 {
		t.Fatalf("expected at least 2 weeks, got %#v", result.Weeks)
	}
	var found bool
	for _, week := range result.Weeks {
		if week.WeekStart.String() == "2026-04-06" && week.Progress != nil && *week.Progress == 50 {
			found = true
		}
	}
	if !found {
		t.Fatalf("later progress should cascade to 50, got %#v", result.Weeks)
	}
	if _, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-03-30"), weekly.Patch{Progress: nullable.Present(60.0)}, now); err == nil {
		t.Fatal("relocked March should reject edits")
	}
	keepEarlier, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tkID, WeekStart: "2026-03-30"})
	if err != nil || !keepEarlier.Progress.Valid || keepEarlier.Progress.Float64 != 50 {
		t.Fatalf("rejected edit changed stored earlier progress: %#v %v", keepEarlier, err)
	}
	keepLater, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tkID, WeekStart: "2026-05-04"})
	if err != nil || !keepLater.Progress.Valid || keepLater.Progress.Float64 != 70 {
		t.Fatalf("rejected edit changed stored later progress: %#v %v", keepLater, err)
	}
	filter, err := task.ParseFilter(strconv.FormatInt(pid, 10), "")
	if err != nil {
		t.Fatal(err)
	}
	grid, err := task.LoadGrid(ctx, q, filter, now, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range grid.Rows[0].Cells {
		if cell.WeekStart.String() == "2026-04-06" {
			if cell.Progress != 50 || cell.Stored || !cell.Locked {
				t.Fatalf("locked April carry changed: %#v", cell)
			}
			return
		}
	}
	t.Fatal("missing displayed April week")
}

func TestBelowCarriedProgressDoesNotEraseStoredValue(t *testing.T) {
	ctx := t.Context()
	q, pid, tkID := newProjectTask(t, 100.0, "2026-03-02", "2026-06-01")
	if pid == 0 {
		t.Fatal("expected a project ID")
	}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	cell, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-03-30"), weekly.Patch{Progress: nullable.Present(50.0), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != 50 {
		t.Fatalf("March save should store effective 50: %#v", cell)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-04-06"), weekly.Patch{Progress: nullable.Present(60.0), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != 60 {
		t.Fatalf("April save should store effective 60: %#v", cell)
	}
	for _, value := range []float64{45, 55} {
		cell, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-04-06"), weekly.Patch{Progress: nullable.Present(value), Unlock: true}, now)
		if err != nil {
			t.Fatal(err)
		}
		if cell.Progress == nil || *cell.Progress != 60 {
			t.Fatalf("write %.0f should keep effective 60: %#v", value, cell)
		}
		row, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tkID, WeekStart: "2026-04-06"})
		if err != nil || !row.Progress.Valid || row.Progress.Float64 != 60 {
			t.Fatalf("write %.0f changed stored progress: %#v %v", value, row, err)
		}
	}
}

func TestSaveRejectsInvalidPatches(t *testing.T) {
	ctx := t.Context()
	q, pid, tkID := newProjectTask(t, 100.0, "2026-01-05", "2026-06-01")
	if pid == 0 {
		t.Fatal("expected a project ID")
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
			ws, err := weekly.Parse(tc.week)
			if err == nil {
				_, err = weekly.Save(ctx, q, tkID, ws, tc.patch, now)
			}
			var httpErr web.HTTPError
			if err == nil || !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadRequest || httpErr.Message != tc.wantMessage {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
	result, err := task.Get(ctx, q, tkID)
	if err != nil {
		t.Fatal(err)
	}
	for _, week := range result.Weeks {
		if week.PlannedHours != 0 || week.SpentHours != 0 || week.StoredProgress != nil {
			t.Fatalf("invalid patches should not create weekly data, got %#v", result.Weeks)
		}
	}
}

func TestSavePartialPatchPreservesExistingValues(t *testing.T) {
	ctx := t.Context()
	q, pid, tkID := newProjectTask(t, 10.0, "2026-09-01", "2026-09-30")
	if pid == 0 {
		t.Fatal("expected a project ID")
	}
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	planned, spent, progress := 8.0, 3.0, 25.0
	cell, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-09-07"), weekly.Patch{PlannedHours: nullable.Present(planned), SpentHours: nullable.Present(spent), Progress: nullable.Present(progress)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != planned || cell.SpentHours != spent || cell.Progress == nil || *cell.Progress != progress {
		t.Fatalf("initial save returned unexpected cell: %#v", cell)
	}
	updatedSpent := 5.0
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-09-07"), weekly.Patch{SpentHours: nullable.Present(updatedSpent)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != planned || cell.SpentHours != updatedSpent || cell.Progress == nil || *cell.Progress != progress {
		t.Fatalf("partial patch changed untouched values: %#v", cell)
	}
	nextHours := 2.0
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-09-14"), weekly.Patch{PlannedHours: nullable.Present(nextHours)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != progress {
		t.Fatalf("carried progress should be effective in save response: %#v", cell)
	}
}

func TestClearProgressRestoresCarryForwardWithoutChangingHours(t *testing.T) {
	ctx := t.Context()
	q, pid, tkID := newProjectTask(t, 100.0, "2026-03-02", "2026-04-27")
	if pid == 0 {
		t.Fatal("expected a project ID")
	}
	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	p20, p50, p70, hours := 20.0, 50.0, 70.0, 3.0
	cell, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-03-02"), weekly.Patch{Progress: nullable.Present(p20), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != p20 {
		t.Fatalf("March 2 save should store effective 20: %#v", cell)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-03-30"), weekly.Patch{Progress: nullable.Present(p50), PlannedHours: nullable.Present(hours), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != hours || cell.Progress == nil || *cell.Progress != p50 {
		t.Fatalf("March 30 save should store hours and effective 50: %#v", cell)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-04-06"), weekly.Patch{Progress: nullable.Present(p70), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != p70 {
		t.Fatalf("April save should store effective 70: %#v", cell)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-03-30"), weekly.Patch{Progress: nullable.Clear[float64]()}, now)
	if err == nil {
		t.Fatal("clear should not change relocked history")
	}
	row, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tkID, WeekStart: "2026-03-30"})
	if err != nil || !row.Progress.Valid || row.Progress.Float64 != p50 || row.PlannedHours != hours {
		t.Fatalf("rejected clear changed stored March 30 data: %#v %v", row, err)
	}
	cell, err = weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-03-30"), weekly.Patch{Progress: nullable.Clear[float64](), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != hours || cell.Progress == nil || *cell.Progress != p20 {
		t.Fatalf("clear did not carry forward while preserving hours: %#v", cell)
	}
	row, err = q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tkID, WeekStart: "2026-03-30"})
	if err != nil || row.Progress.Valid {
		t.Fatalf("explicit progress was not cleared: %#v %v", row, err)
	}
	later, err := q.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: tkID, WeekStart: "2026-04-06"})
	if err != nil || !later.Progress.Valid || later.Progress.Float64 != p70 {
		t.Fatalf("clearing earlier progress changed later explicit progress: %#v %v", later, err)
	}
}

func newProjectTask(t *testing.T, total float64, start, end string) (*db.Queries, int64, int64) {
	t.Helper()
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(total), StartDate: start, EndDate: end})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	return q, p.ID, tk.ID
}

func newTaskWithDeveloper(t *testing.T, start, end string) (*db.Queries, int64, int64) {
	t.Helper()
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(100.0), StartDate: start, EndDate: end})
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
	return q, tk.ID, dev.ID
}

func TestSaveAutoAttributesNewWeekToFirstDeveloper(t *testing.T) {
	ctx := t.Context()
	q, tkID, devID := newTaskWithDeveloper(t, "2026-01-05", "2026-06-01")
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	planned, spent := 8.0, 3.0
	cell, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-04-27"), weekly.Patch{PlannedHours: nullable.Present(planned), SpentHours: nullable.Present(spent), Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(cell.Attributions) != 1 || cell.Attributions[0].PersonID != devID || cell.Attributions[0].Name != "Ada" || cell.Attributions[0].PlannedHours != planned || cell.Attributions[0].SpentHours != spent {
		t.Fatalf("unexpected attributions: %#v", cell.Attributions)
	}
	rows, err := q.ListTaskWeekDevelopers(ctx, db.ListTaskWeekDevelopersParams{TaskID: tkID, WeekStart: "2026-04-27"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("attribution row missing: %#v %v", rows, err)
	}
	if rows[0].PersonID != devID || rows[0].PlannedHours != planned || rows[0].SpentHours != spent {
		t.Fatalf("unexpected attribution row: %#v", rows[0])
	}
}

func TestSaveDoesNotAutoAttributeWithoutDevelopers(t *testing.T) {
	ctx := t.Context()
	q, _, tkID := newProjectTask(t, 100.0, "2026-01-05", "2026-06-01")
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	cell, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-01-05"), weekly.Patch{PlannedHours: nullable.Present(8.0)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(cell.Attributions) != 0 {
		t.Fatalf("unexpected attributions: %#v", cell.Attributions)
	}
	rows, err := q.ListTaskWeekDevelopers(ctx, db.ListTaskWeekDevelopersParams{TaskID: tkID, WeekStart: "2026-01-05"})
	if err != nil || len(rows) != 0 {
		t.Fatalf("unexpected attribution rows: %#v %v", rows, err)
	}
}

func TestSaveDoesNotRebalanceExistingAttribution(t *testing.T) {
	ctx := t.Context()
	q, tkID, devID := newTaskWithDeveloper(t, "2026-01-05", "2026-06-01")
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	week := testkit.MustWeek(t, "2026-01-05")
	if _, err := weekly.Save(ctx, q, tkID, week, weekly.Patch{PlannedHours: nullable.Present(8.0), SpentHours: nullable.Present(3.0)}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.Save(ctx, q, tkID, week, weekly.Patch{PlannedHours: nullable.Present(10.0), SpentHours: nullable.Present(5.0)}, now); err != nil {
		t.Fatal(err)
	}
	rows, err := q.ListTaskWeekDevelopers(ctx, db.ListTaskWeekDevelopersParams{TaskID: tkID, WeekStart: "2026-01-05"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("unexpected attribution rows: %#v %v", rows, err)
	}
	if rows[0].PersonID != devID || rows[0].PlannedHours != 8 || rows[0].SpentHours != 3 {
		t.Fatalf("second save rebalanced the attribution: %#v", rows[0])
	}
}

func TestSaveDoesNotResurrectAttributionAfterDelete(t *testing.T) {
	ctx := t.Context()
	q, tkID, devID := newTaskWithDeveloper(t, "2026-01-05", "2026-06-01")
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	week := testkit.MustWeek(t, "2026-01-05")
	if _, err := weekly.Save(ctx, q, tkID, week, weekly.Patch{PlannedHours: nullable.Present(8.0)}, now); err != nil {
		t.Fatal(err)
	}
	if err := weekly.ClearAttribution(ctx, q, tkID, week, devID, false, now); err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.Save(ctx, q, tkID, week, weekly.Patch{PlannedHours: nullable.Present(10.0)}, now); err != nil {
		t.Fatal(err)
	}
	rows, err := q.ListTaskWeekDevelopers(ctx, db.ListTaskWeekDevelopersParams{TaskID: tkID, WeekStart: "2026-01-05"})
	if err != nil || len(rows) != 0 {
		t.Fatalf("attribution resurrected after delete: %#v %v", rows, err)
	}
}

func TestSaveAttributionLockAndErrors(t *testing.T) {
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
	other, err := person.Create(ctx, q, person.Input{Name: "Grace"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID, DeveloperIDs: []int64{dev.ID}})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	locked := testkit.MustWeek(t, "2026-04-27")
	if _, err := weekly.SaveAttribution(ctx, q, tk.ID, locked, dev.ID, weekly.AttributionPatch{PlannedHours: 5, SpentHours: 2}, now); err == nil {
		t.Fatal("expected locked historical attribution to fail")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Status != http.StatusForbidden {
		t.Fatalf("unexpected lock error: %v", err)
	}
	allocation, err := weekly.SaveAttribution(ctx, q, tk.ID, locked, dev.ID, weekly.AttributionPatch{PlannedHours: 5, SpentHours: 2, Unlock: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if allocation.PersonID != dev.ID || allocation.Name != "Ada" || allocation.PlannedHours != 5 || allocation.SpentHours != 2 {
		t.Fatalf("unexpected allocation: %#v", allocation)
	}
	if _, err := weekly.SaveAttribution(ctx, q, tk.ID, locked, 999, weekly.AttributionPatch{PlannedHours: 1, Unlock: true}, now); err == nil {
		t.Fatal("expected unknown person to fail")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Status != http.StatusNotFound || httpErr.Reason != "person-not-found" {
		t.Fatalf("unexpected unknown person error: %v", err)
	}
	if _, err := weekly.SaveAttribution(ctx, q, tk.ID, locked, other.ID, weekly.AttributionPatch{PlannedHours: 1, Unlock: true}, now); err == nil {
		t.Fatal("expected person not on task to fail")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Status != http.StatusBadRequest || httpErr.Reason != "person-not-on-task" {
		t.Fatalf("unexpected person-not-on-task error: %v", err)
	}
	idea, err := task.Create(ctx, q, task.Input{Name: "Idea", DeveloperIDs: []int64{dev.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.SaveAttribution(ctx, q, idea.ID, testkit.MustWeek(t, "2026-05-04"), dev.ID, weekly.AttributionPatch{PlannedHours: 1}, now); err == nil {
		t.Fatal("expected idea task attribution to fail")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Reason != "idea-task-not-assignable" {
		t.Fatalf("unexpected idea error: %v", err)
	}
}

func TestClearAttribution(t *testing.T) {
	ctx := t.Context()
	q, tkID, devID := newTaskWithDeveloper(t, "2026-01-05", "2026-06-01")
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	week := testkit.MustWeek(t, "2026-01-05")
	if _, err := weekly.SaveAttribution(ctx, q, tkID, week, devID, weekly.AttributionPatch{PlannedHours: 5, SpentHours: 2}, now); err != nil {
		t.Fatal(err)
	}
	if err := weekly.ClearAttribution(ctx, q, tkID, week, devID, false, now); err != nil {
		t.Fatal(err)
	}
	rows, err := q.ListTaskWeekDevelopers(ctx, db.ListTaskWeekDevelopersParams{TaskID: tkID, WeekStart: "2026-01-05"})
	if err != nil || len(rows) != 0 {
		t.Fatalf("allocation still present after clear: %#v %v", rows, err)
	}
	if err := weekly.ClearAttribution(ctx, q, tkID, week, devID, false, now); err == nil {
		t.Fatal("expected second clear to fail")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Status != http.StatusNotFound || httpErr.Message != "allocation not found" {
		t.Fatalf("unexpected clear error: %v", err)
	}
}

func TestSaveProgressOnlyDoesNotAutoAttribute(t *testing.T) {
	ctx := t.Context()
	q, tkID, _ := newTaskWithDeveloper(t, "2026-01-05", "2026-06-01")
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	cell, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-01-05"), weekly.Patch{Progress: nullable.Present(40.0)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(cell.Attributions) != 0 {
		t.Fatalf("unexpected attributions: %#v", cell.Attributions)
	}
	rows, err := q.ListTaskWeekDevelopers(ctx, db.ListTaskWeekDevelopersParams{TaskID: tkID, WeekStart: "2026-01-05"})
	if err != nil || len(rows) != 0 {
		t.Fatalf("progress-only save created attribution rows: %#v %v", rows, err)
	}
}

func TestClearAttributionAfterDeveloperRemoved(t *testing.T) {
	ctx := t.Context()
	q, tkID, devID := newTaskWithDeveloper(t, "2026-01-05", "2026-06-01")
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	week := testkit.MustWeek(t, "2026-01-05")
	if _, err := weekly.SaveAttribution(ctx, q, tkID, week, devID, weekly.AttributionPatch{PlannedHours: 5, SpentHours: 2}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := task.Update(ctx, q, tkID, task.Patch{DeveloperIDs: nullable.Present([]int64{})}); err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.SaveAttribution(ctx, q, tkID, week, devID, weekly.AttributionPatch{PlannedHours: 1}, now); err == nil {
		t.Fatal("expected person-not-on-task to fail")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Status != http.StatusBadRequest || httpErr.Reason != "person-not-on-task" {
		t.Fatalf("unexpected person-not-on-task error: %v", err)
	}
	if err := weekly.ClearAttribution(ctx, q, tkID, week, devID, false, now); err != nil {
		t.Fatalf("clear after developer removed failed: %v", err)
	}
	rows, err := q.ListTaskWeekDevelopers(ctx, db.ListTaskWeekDevelopersParams{TaskID: tkID, WeekStart: "2026-01-05"})
	if err != nil || len(rows) != 0 {
		t.Fatalf("allocation still present after clear: %#v %v", rows, err)
	}
}

func TestDeletePersonWithWeekAllocationsRejected(t *testing.T) {
	ctx := t.Context()
	q, tkID, devID := newTaskWithDeveloper(t, "2026-01-05", "2026-06-01")
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if _, err := weekly.Save(ctx, q, tkID, testkit.MustWeek(t, "2026-01-05"), weekly.Patch{PlannedHours: nullable.Present(8.0)}, now); err != nil {
		t.Fatal(err)
	}
	if err := person.Delete(ctx, q, devID); err == nil {
		t.Fatal("expected person delete to be rejected")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Message != "record is still used by other data" {
		t.Fatalf("unexpected delete error: %v", err)
	}
	got, err := person.Get(ctx, q, devID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != devID {
		t.Fatalf("rejected delete removed the person: %#v", got)
	}
}

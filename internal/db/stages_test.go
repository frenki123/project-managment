package db_test

import (
	"context"
	"strings"
	"testing"

	"cad-development/internal/db"
	"cad-development/internal/db/testkit"
)

const pinMessage = "first stage must start at 0% and last stage must end at 100%"

func listStages(t *testing.T, database *db.Database) []db.Stage {
	t.Helper()
	stages, err := database.Q.ListStages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return stages
}

// assertSeededStages proves the stages table still holds exactly the four rows
// seeded by the initial migration, in position order.
func assertSeededStages(t *testing.T, database *db.Database) {
	t.Helper()
	stages := listStages(t, database)
	if len(stages) != 4 {
		t.Fatalf("got %d stages, want 4: %#v", len(stages), stages)
	}
	want := []db.Stage{
		{Name: "Planned", Position: 1, Color: "#607d8b", AutoReachable: 1, ProgressThreshold: 0},
		{Name: "In progress", Position: 2, Color: "#2196f3", AutoReachable: 1, ProgressThreshold: 1},
		{Name: "In review", Position: 3, Color: "#ff9800", AutoReachable: 1, ProgressThreshold: 80},
		{Name: "Done", Position: 4, Color: "#4caf50", AutoReachable: 1, ProgressThreshold: 100},
	}
	for i, w := range want {
		got := stages[i]
		got.ID = 0 // ids are assigned by the seed; compare the rest
		if got != w {
			t.Fatalf("stage %d = %#v, want %#v", i, got, w)
		}
	}
}

func TestStagesSeed(t *testing.T) {
	assertSeededStages(t, testkit.OpenDatabase(t))
}

func TestStagesPinRejectsInvariantViolations(t *testing.T) {
	insert := func(t *testing.T, database *db.Database, name string, position int64, threshold float64) error {
		t.Helper()
		_, err := database.Conn.Exec(
			"INSERT INTO stages (name, position, color, auto_reachable, progress_threshold) VALUES (?, ?, '', 1, ?)",
			name, position, threshold,
		)
		return err
	}

	t.Run("insert before first stage", func(t *testing.T) {
		database := testkit.OpenDatabase(t)
		err := insert(t, database, "Pre", 0, 50)
		if err == nil {
			t.Fatal("expected insert before the first stage with nonzero threshold to be rejected")
		}
		if !strings.Contains(err.Error(), pinMessage) {
			t.Fatalf("error = %v, want it to carry the pin message", err)
		}
		assertSeededStages(t, database)
	})

	t.Run("insert after last stage", func(t *testing.T) {
		database := testkit.OpenDatabase(t)
		err := insert(t, database, "Post", 5, 10)
		if err == nil {
			t.Fatal("expected insert after the last stage with threshold below 100 to be rejected")
		}
		if !strings.Contains(err.Error(), pinMessage) {
			t.Fatalf("error = %v, want it to carry the pin message", err)
		}
		assertSeededStages(t, database)
	})

	t.Run("update last stage threshold", func(t *testing.T) {
		database := testkit.OpenDatabase(t)
		_, err := database.Conn.Exec("UPDATE stages SET progress_threshold = 50 WHERE name = 'Done'")
		if err == nil {
			t.Fatal("expected moving the last stage below 100% to be rejected")
		}
		if !strings.Contains(err.Error(), pinMessage) {
			t.Fatalf("error = %v, want it to carry the pin message", err)
		}
		assertSeededStages(t, database)
	})

	t.Run("delete first stage", func(t *testing.T) {
		database := testkit.OpenDatabase(t)
		_, err := database.Conn.Exec("DELETE FROM stages WHERE name = 'Planned'")
		if err == nil {
			t.Fatal("expected deleting the first stage to be rejected")
		}
		if !strings.Contains(err.Error(), pinMessage) {
			t.Fatalf("error = %v, want it to carry the pin message", err)
		}
		assertSeededStages(t, database)
	})
}

func TestStagesPinAllowsMiddleChanges(t *testing.T) {
	t.Run("manual-only middle stage", func(t *testing.T) {
		database := testkit.OpenDatabase(t)
		// Free a middle position without breaking the pins: 'Done' stays the
		// max stage (position 10, threshold 100), so a stage at position 5 is
		// neither first nor last and may carry any threshold.
		if _, err := database.Conn.Exec("UPDATE stages SET position = 10 WHERE name = 'Done'"); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Conn.Exec(
			"INSERT INTO stages (name, position, color, auto_reachable, progress_threshold) VALUES ('On hold', 5, '#9e9e9e', 0, 90)",
		); err != nil {
			t.Fatalf("expected a manual-only middle stage to be allowed: %v", err)
		}
		stages := listStages(t, database)
		if len(stages) != 5 {
			t.Fatalf("got %d stages, want 5: %#v", len(stages), stages)
		}
		var onHold *db.Stage
		for i := range stages {
			if stages[i].Name == "On hold" {
				onHold = &stages[i]
			}
		}
		if onHold == nil || onHold.Position != 5 || onHold.AutoReachable != 0 || onHold.ProgressThreshold != 90 {
			t.Fatalf("On hold stage not persisted as inserted: %#v", stages)
		}
	})

	t.Run("middle stage threshold update", func(t *testing.T) {
		database := testkit.OpenDatabase(t)
		if _, err := database.Conn.Exec("UPDATE stages SET progress_threshold = 5 WHERE name = 'In progress'"); err != nil {
			t.Fatalf("expected updating a middle stage threshold to be allowed: %v", err)
		}
		stages := listStages(t, database)
		if len(stages) != 4 {
			t.Fatalf("got %d stages, want 4: %#v", len(stages), stages)
		}
		if stages[1].Name != "In progress" || stages[1].ProgressThreshold != 5 {
			t.Fatalf("middle threshold update not persisted: %#v", stages)
		}
	})
}

func TestTaskTotalsStageResolution(t *testing.T) {
	database := testkit.OpenDatabase(t)
	ctx := context.Background()

	if _, err := database.Conn.Exec(
		"INSERT INTO projects (name, purchase_order_name, total_hours, start_date, end_date) VALUES ('Stage project', '', 100, '2026-01-05', '2026-02-01')",
	); err != nil {
		t.Fatal(err)
	}

	insertTask := func(t *testing.T, name string, progress float64) int64 {
		t.Helper()
		if _, err := database.Conn.Exec("INSERT INTO tasks (name, project_id) VALUES (?, 1)", name); err != nil {
			t.Fatal(err)
		}
		var id int64
		if err := database.Conn.QueryRow("SELECT id FROM tasks WHERE name = ?", name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Conn.Exec(
			"INSERT INTO task_weeks (task_id, week_start, planned_hours, spent_hours, progress) VALUES (?, '2026-01-05', 10, 0, ?)",
			id, progress,
		); err != nil {
			t.Fatal(err)
		}
		return id
	}

	for _, tc := range []struct {
		name     string
		progress float64
		stage    string
	}{
		{"Zero", 0, "Planned"},
		{"Boundary in progress", 1, "In progress"},
		{"Mid", 50, "In progress"},
		{"Boundary in review", 80, "In review"},
		{"Review", 90, "In review"},
		{"Full", 100, "Done"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := insertTask(t, "Task "+tc.name, tc.progress)
			row, err := database.Q.GetTaskTotals(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if row.ManualStatus.Valid {
				t.Fatalf("manual_status = %q, want NULL", row.ManualStatus.String)
			}
			if !row.Stage.Valid || row.Stage.String != tc.stage {
				t.Fatalf("stage = %#v, want %q", row.Stage, tc.stage)
			}
		})
	}

	t.Run("manual_status overrides derived stage", func(t *testing.T) {
		id := insertTask(t, "Task Manual", 0)
		if _, err := database.Conn.Exec("UPDATE tasks SET manual_status = 'On hold' WHERE id = ?", id); err != nil {
			t.Fatal(err)
		}
		row, err := database.Q.GetTaskTotals(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if !row.ManualStatus.Valid || row.ManualStatus.String != "On hold" {
			t.Fatalf("manual_status = %#v, want 'On hold'", row.ManualStatus)
		}
		if !row.Stage.Valid || row.Stage.String != "On hold" {
			t.Fatalf("stage = %#v, want manual 'On hold' override", row.Stage)
		}
		// Clearing manual_status back to NULL restores the derived stage.
		if _, err := database.Conn.Exec("UPDATE tasks SET manual_status = NULL WHERE id = ?", id); err != nil {
			t.Fatal(err)
		}
		row, err = database.Q.GetTaskTotals(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if row.ManualStatus.Valid {
			t.Fatalf("manual_status = %#v, want NULL after clearing", row.ManualStatus)
		}
		if !row.Stage.Valid || row.Stage.String != "Planned" {
			t.Fatalf("stage = %#v, want derived 'Planned' after clearing manual_status", row.Stage)
		}
	})
}
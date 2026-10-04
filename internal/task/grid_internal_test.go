package task

import (
	"testing"
	"time"

	"cad-development/internal/weekly"
)

func TestBuildTaskRowsIncludesMissingSeriesWeek(t *testing.T) {
	first, err := weekly.Parse("2026-09-07")
	if err != nil {
		t.Fatal(err)
	}
	second := first.Next()
	firstInfo, err := first.Info()
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := second.Info()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := buildTaskRows(
		[]Task{{ID: 1, Name: "Task"}},
		map[int64][]weekly.Cell{1: {{TaskID: 1, WeekStart: first, PlannedHours: 2, SpentHours: 1, Progress: new(25.0)}}},
		Grid{Weeks: []weekly.WeekInfo{firstInfo, secondInfo}},
		nil,
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(rows[0].Cells) != 2 {
		t.Fatalf("got %d rows and %d cells, want 1 row and 2 cells: %#v", len(rows), len(rows[0].Cells), rows)
	}
	missing := rows[0].Cells[1]
	if missing.WeekStart != second || missing.Planned != 0 || missing.Spent != 0 || missing.Progress != 0 || missing.Stored {
		t.Fatalf("unexpected missing-week cell: %#v", missing)
	}
	if !missing.Locked || missing.SavePath != "/tasks/1/weeks/2026-09-14" {
		t.Fatalf("missing-week metadata not preserved: %#v", missing)
	}
}

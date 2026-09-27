package monthlock

import (
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	assets "cad-development"
	"cad-development/internal/app"
)

func TestLocked(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	april := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	march := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !Locked(april, now, false) || !Locked(march, now, false) {
		t.Fatal("all past months should be locked on 2 May")
	}
	if Locked(now, now, false) {
		t.Fatal("current month should be editable")
	}
	if Locked(now.AddDate(0, 1, 0), now, false) {
		t.Fatal("future month should be editable")
	}
	if Locked(april, now, true) || Locked(march, now, true) {
		t.Fatal("global unlock should open all past months")
	}
	if !Locked(time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), false) {
		t.Fatal("December should be locked in January")
	}
}

func TestWeekLockedUsesMondayMonth(t *testing.T) {
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	if !WeekLocked("2026-04-27", now, false) || !WeekLocked("2026-03-30", now, false) {
		t.Fatal("week starting in April should be locked in May")
	}
	if WeekLocked("2026-04-27", now, true) || WeekLocked("2026-03-30", now, true) {
		t.Fatal("global unlock should open every past week")
	}
	if WeekLocked("2026-05-04", now, false) {
		t.Fatal("week starting in May should be open")
	}
}

func TestGlobalUnlockSurvivesDatabaseReopen(t *testing.T) {
	migrations, err := fs.Sub(assets.FS, "sql/migrations")
	if err != nil {
		t.Fatal(err)
	}
	config := app.DatabaseConfig{DSN: filepath.Join(t.TempDir(), "history.db"), Migrations: migrations, MaxOpenConns: 1}
	database, err := app.OpenDatabase(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := Set(t.Context(), database.Q, true); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	database, err = app.OpenDatabase(config)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	unlocked, err := Unlocked(t.Context(), database.Q)
	if err != nil || !unlocked {
		t.Fatalf("unlock lost after database reopen: %v %v", unlocked, err)
	}
}

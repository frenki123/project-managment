package views

import (
	"testing"

	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestTaskStatus(t *testing.T) {
	cases := []struct {
		progress float64
		want     string
	}{
		{0, "Planned"},
		{1, "Development"},
		{79, "Development"},
		{80, "Internal Testing"},
		{99, "Internal Testing"},
		{100, "Deployment"},
	}
	for _, c := range cases {
		if got := task.Status(c.progress); got != c.want {
			t.Errorf("task.Status(%v) = %q, want %q", c.progress, got, c.want)
		}
	}
}

func TestHours(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{2, "2"},
		{2.5, "2.5"},
		{13.999999, "14"},
		{900, "900"},
		{2.051282051282051, "2.1"},
		{34.66666666666667, "34.7"},
		{2.25, "2.2"},
	}
	for _, c := range cases {
		if got := hours(c.in); got != c.want {
			t.Errorf("hours(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestWeekLabel(t *testing.T) {
	week, err := weekly.Parse("2026-01-05")
	if err != nil {
		t.Fatal(err)
	}
	if got := weekLabel(week); got != "W2 · 05.01" {
		t.Fatalf("weekLabel() = %q, want %q", got, "W2 · 05.01")
	}
	if got := weekLabel(weekly.WeekStart{}); got != "" {
		t.Fatalf("weekLabel(zero) = %q, want empty", got)
	}
}

func TestPercent(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{12, "12"},
		{100, "100"},
		{0.4, "0"},
		{99.6, "99"},
		{82.9, "82"},
		{33.33333333333333, "33"},
		{47.5, "47"},
	}
	for _, c := range cases {
		if got := percent(c.in); got != c.want {
			t.Errorf("percent(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

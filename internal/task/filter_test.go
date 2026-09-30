package task

import (
	"context"
	"strconv"
	"testing"

	"cad-development/internal/db/testkit"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/web"
)

func TestFilterValidatesAndListsWithPrecedence(t *testing.T) {
	q := testkit.Open(t)
	ctx := context.Background()
	first, err := project.Create(ctx, q, project.Input{Name: "First", TotalHours: new(100.0), StartDate: "2026-01-05", EndDate: "2026-01-12"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.Create(ctx, q, project.Input{Name: "Second", TotalHours: new(100.0), StartDate: "2026-01-05", EndDate: "2026-01-12"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: first.ID, Name: "Part", TotalHours: new(10.0)})
	if err != nil {
		t.Fatal(err)
	}

	valid, err := ParseFilter(strconv.FormatInt(first.ID, 10), strconv.FormatInt(sp.ID, 10))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveFilter(ctx, q, valid); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, project, subproject string
		status                    int
	}{
		{"missing project", "999", "", 404},
		{"missing subproject", strconv.FormatInt(first.ID, 10), "999", 404},
		{"cross project", strconv.FormatInt(second.ID, 10), strconv.FormatInt(sp.ID, 10), 400},
		{"malformed project", "bad", "", 400},
		{"malformed subproject", "all", "bad", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := ParseFilter(tc.project, tc.subproject)
			if err == nil {
				_, err = ResolveFilter(ctx, q, f)
			}
			httpErr, ok := web.HTTPErrorFrom(err), true
			if !ok || httpErr.Status != tc.status {
				t.Fatalf("error = %v", err)
			}
		})
	}

	for _, tc := range []struct{ project, subproject, key string }{
		{"", "", "project=all"},
		{"ideas", "", "project=ideas"},
		{strconv.FormatInt(first.ID, 10), strconv.FormatInt(sp.ID, 10), "project=" + strconv.FormatInt(first.ID, 10) + "&subproject=" + strconv.FormatInt(sp.ID, 10)},
	} {
		f, err := ParseFilter(tc.project, tc.subproject)
		if err != nil {
			t.Fatal(err)
		}
		if got := FilterKey(f); got != tc.key {
			t.Fatalf("key = %q, want %q", got, tc.key)
		}
	}
}

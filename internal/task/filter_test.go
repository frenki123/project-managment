package task

import (
	"context"
	"strconv"
	"testing"

	"cad-development/internal/app/testkit"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
)

func TestNormalizeSubprojectFilterFallsBackForMissingOrOtherProject(t *testing.T) {
	q := testkit.Open(t)
	ctx := context.Background()
	first, err := project.Create(ctx, q, project.Input{
		Name: "First", TotalHours: floatPtr(100), StartDate: "2026-01-05", EndDate: "2026-01-12",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.Create(ctx, q, project.Input{
		Name: "Second", TotalHours: floatPtr(100), StartDate: "2026-01-05", EndDate: "2026-01-12",
	})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: first.ID, Name: "Part", TotalHours: floatPtr(10)})
	if err != nil {
		t.Fatal(err)
	}

	valid, err := NormalizeSubprojectFilter(ctx, q, projectKey(first.ID), &sp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if valid == nil {
		t.Fatal("valid subproject filter was discarded")
	}

	missingID := int64(999)
	missing, err := NormalizeSubprojectFilter(ctx, q, projectKey(first.ID), &missingID)
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("missing subproject filter was retained: %v", *missing)
	}

	otherProject, err := NormalizeSubprojectFilter(ctx, q, projectKey(second.ID), &sp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if otherProject != nil {
		t.Fatalf("cross-project subproject filter was retained: %v", *otherProject)
	}
}

func projectKey(id int64) string { return strconv.FormatInt(id, 10) }

func floatPtr(value float64) *float64 { return &value }

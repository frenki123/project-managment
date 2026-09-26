package subproject_test

import (
	"context"
	"math"
	"testing"

	"cad-development/internal/app/testkit"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
)

func TestHoursCap(t *testing.T) {
	ctx := context.Background()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: 10, StartDate: "2026-01-05", EndDate: "2026-02-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "A", TotalHours: 6}); err != nil {
		t.Fatal(err)
	}
	if _, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "B", TotalHours: 5}); err == nil {
		t.Fatal("expected hours cap")
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "B", TotalHours: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := subproject.Update(ctx, q, sp.ID, subproject.Input{ProjectID: p.ID, Name: "B", TotalHours: 5}); err == nil {
		t.Fatal("expected update cap")
	}
	got, err := subproject.Get(ctx, q, sp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalHours != 4 {
		t.Fatalf("rejected update changed hours: %v", got.TotalHours)
	}
	list, err := subproject.ListByProject(ctx, q, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("unexpected subproject count: %d", len(list))
	}
}

func TestHoursRejectNonFiniteValues(t *testing.T) {
	ctx := context.Background()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: 10, StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "bad", TotalHours: value}); err == nil {
			t.Fatalf("expected non-finite value %v to be rejected", value)
		}
	}
}

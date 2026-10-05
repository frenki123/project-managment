package person_test

import (
	"errors"
	"net/http"
	"testing"

	"cad-development/internal/db/testkit"
	"cad-development/internal/nullable"
	"cad-development/internal/person"
	"cad-development/internal/web"
)

func TestCreateDefaultsWeeklyCapacityTo40(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := person.Create(ctx, q, person.Input{Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	if p.WeeklyCapacity != 40 {
		t.Fatalf("default capacity = %v, want 40", p.WeeklyCapacity)
	}
	got, err := person.Get(ctx, q, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ada" || got.WeeklyCapacity != 40 {
		t.Fatalf("unexpected person: %#v", got)
	}
}

func TestCreateStoresCustomWeeklyCapacity(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := person.Create(ctx, q, person.Input{Name: "Grace", WeeklyCapacity: new(32.0)})
	if err != nil {
		t.Fatal(err)
	}
	if p.WeeklyCapacity != 32 {
		t.Fatalf("capacity = %v, want 32", p.WeeklyCapacity)
	}
	got, err := person.Get(ctx, q, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.WeeklyCapacity != 32 {
		t.Fatalf("unexpected person: %#v", got)
	}
}

func TestCreateRejectsBlankName(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	_, err := person.Create(ctx, q, person.Input{Name: "   "})
	var httpErr web.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadRequest || httpErr.Message != "name is required" {
		t.Fatalf("expected name required, got %v", err)
	}
	people, err := person.List(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 0 {
		t.Fatalf("blank name create persisted a person: %#v", people)
	}
}

func TestCreateRejectsNegativeCapacity(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	_, err := person.Create(ctx, q, person.Input{Name: "Ada", WeeklyCapacity: new(-1.0)})
	var httpErr web.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadRequest || httpErr.Message != "hours cannot be negative" {
		t.Fatalf("expected negative hours rejected, got %v", err)
	}
	people, err := person.List(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 0 {
		t.Fatalf("negative capacity create persisted a person: %#v", people)
	}
}

func TestPersonNamesAreCaseInsensitiveUnique(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	if _, err := person.Create(ctx, q, person.Input{Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	second, err := person.Create(ctx, q, person.Input{Name: "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = person.Create(ctx, q, person.Input{Name: "alpha"})
	var httpErr web.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusConflict || httpErr.Message != "person name already exists" {
		t.Fatalf("unexpected duplicate create error: %v", err)
	}
	people, err := person.List(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 2 {
		t.Fatalf("duplicate create persisted a person: %#v", people)
	}
	_, err = person.Update(ctx, q, second.ID, person.Patch{Name: nullable.Present("ALPHA")})
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusConflict || httpErr.Message != "person name already exists" {
		t.Fatalf("unexpected duplicate update error: %v", err)
	}
	got, err := person.Get(ctx, q, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Beta" {
		t.Fatalf("rejected update changed the name: %#v", got)
	}
}

func TestUpdatePartialPatchPreservesCapacity(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := person.Create(ctx, q, person.Input{Name: "Ada", WeeklyCapacity: new(32.0)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := person.Update(ctx, q, p.ID, person.Patch{Name: nullable.Present("Grace")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Grace" || got.WeeklyCapacity != 32 {
		t.Fatalf("partial update changed capacity: %#v", got)
	}
}
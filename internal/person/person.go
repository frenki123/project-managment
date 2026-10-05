package person

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"cad-development/internal/db"
	"cad-development/internal/nullable"
	"cad-development/internal/web"
)

type Person struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	WeeklyCapacity float64 `json:"weekly_capacity"`
}

type Input struct {
	Name           string   `json:"name"`
	WeeklyCapacity *float64 `json:"weekly_capacity"`
}

type Patch struct {
	Name           nullable.Optional[string]  `json:"name,omitzero"`
	WeeklyCapacity nullable.Optional[float64] `json:"weekly_capacity,omitzero"`
}

type PeopleResponse struct {
	People []Person `json:"people"`
}

func FromDB(p db.Person) Person {
	return Person{ID: p.ID, Name: p.Name, WeeklyCapacity: p.WeeklyCapacity}
}

func validate(in Input) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return in, web.Invalid("name is required")
	}
	if in.WeeklyCapacity != nil && !web.NonNegativeFinite(*in.WeeklyCapacity) {
		return in, web.Invalid("hours cannot be negative")
	}
	return in, nil
}

func nameConflictError(err error) error {
	if db.UniqueViolation(err, "people.name") {
		return web.HTTPErrorFromReason(http.StatusConflict, "person-name-taken")
	}
	return err
}

func Create(ctx context.Context, q *db.Queries, in Input) (Person, error) {
	validated, err := validate(in)
	if err != nil {
		return Person{}, err
	}
	row, err := q.CreatePerson(ctx, db.CreatePersonParams{
		Name:           validated.Name,
		WeeklyCapacity: nullable.Float64(validated.WeeklyCapacity),
	})
	if err != nil {
		return Person{}, nameConflictError(err)
	}
	return FromDB(row), nil
}

func Get(ctx context.Context, q *db.Queries, id int64) (Person, error) {
	row, err := q.GetPerson(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Person{}, web.Missing("person not found")
	}
	if err != nil {
		return Person{}, err
	}
	return FromDB(row), nil
}

func List(ctx context.Context, q *db.Queries) ([]Person, error) {
	rows, err := q.ListPeople(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Person, 0, len(rows))
	for _, row := range rows {
		out = append(out, FromDB(row))
	}
	return out, nil
}

func Update(ctx context.Context, q *db.Queries, id int64, patch Patch) (Person, error) {
	if patch.WeeklyCapacity.IsNull() {
		return Person{}, web.Invalid("weekly capacity cannot be null")
	}
	var updated db.Person
	err := q.InTx(ctx, func(txq *db.Queries) error {
		current, err := txq.GetPerson(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return web.Missing("person not found")
		}
		if err != nil {
			return err
		}
		in := Input{
			Name:           patch.Name.Apply(current.Name),
			WeeklyCapacity: new(patch.WeeklyCapacity.Apply(current.WeeklyCapacity)),
		}
		validated, err := validate(in)
		if err != nil {
			return err
		}
		updated, err = txq.UpdatePerson(ctx, db.UpdatePersonParams{
			Name: validated.Name, WeeklyCapacity: *validated.WeeklyCapacity, ID: id,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return web.Missing("person not found")
		}
		return nameConflictError(err)
	})
	if err != nil {
		return Person{}, err
	}
	return FromDB(updated), nil
}

func Delete(ctx context.Context, q *db.Queries, id int64) error {
	err := q.InTx(ctx, func(txq *db.Queries) error {
		rows, err := txq.DeletePerson(ctx, id)
		if err != nil {
			return err
		}
		if rows == 0 {
			return web.Missing("person not found")
		}
		return nil
	})
	return web.ReferencedConflict(err)
}
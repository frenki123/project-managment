package person

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"cad-development/internal/db"
	"cad-development/internal/nullable"
	"cad-development/internal/web"
	"cad-development/internal/weekly"
)

type Person struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	WeeklyCapacity float64 `json:"weekly_capacity"`
}

type Ref struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Input struct {
	Name           string   `json:"name"`
	WeeklyCapacity *float64 `json:"weekly_capacity"`
}

type Patch struct {
	Name           nullable.Optional[string]  `json:"name,omitzero"`
	WeeklyCapacity nullable.Optional[float64] `json:"weekly_capacity,omitzero"`
}

func PatchFromInput(in Input) Patch {
	patch := Patch{Name: nullable.Present(in.Name)}
	if in.WeeklyCapacity != nil {
		patch.WeeklyCapacity = nullable.Present(*in.WeeklyCapacity)
	}
	return patch
}

type PeopleResponse struct {
	People []Person `json:"people"`
}

type Override struct {
	PersonID  int64   `json:"person_id"`
	WeekStart string  `json:"week_start"`
	Capacity  float64 `json:"capacity"`
}

type OverridesResponse struct {
	Overrides []Override `json:"overrides"`
}

func FromDB(p db.Person) Person {
	return Person{ID: p.ID, Name: p.Name, WeeklyCapacity: p.WeeklyCapacity}
}

func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return name, web.Invalid("name is required")
	}
	return name, nil
}

func validate(in Input) (Input, error) {
	name, err := validateName(in.Name)
	if err != nil {
		return in, err
	}
	in.Name = name
	if in.WeeklyCapacity != nil && !web.NonNegativeFinite(*in.WeeklyCapacity) {
		return in, web.Invalid("hours cannot be negative")
	}
	return in, nil
}

func Create(ctx context.Context, q *db.Queries, in Input) (Person, error) {
	name, err := validateName(in.Name)
	if err != nil {
		return Person{}, err
	}
	in.Name = name
	var id int64
	err = q.InTx(ctx, func(txq *db.Queries) error {
		conflict, err := txq.PersonNameConflict(ctx, db.PersonNameConflictParams{Name: in.Name, ExceptID: 0})
		if err != nil {
			return err
		}
		if err := web.HTTPErrorFromReason(int(conflict.Status), conflict.Reason); err != nil {
			return err
		}
		validated, err := validate(in)
		if err != nil {
			return err
		}
		row, err := txq.CreatePerson(ctx, db.CreatePersonParams{
			Name:           validated.Name,
			WeeklyCapacity: nullable.Float64(validated.WeeklyCapacity),
		})
		if err == nil {
			id = row.ID
		}
		return err
	})
	if err != nil {
		if db.UniqueViolation(err, "people.name") {
			return Person{}, web.Conflict("person name already exists")
		}
		return Person{}, err
	}
	return Get(ctx, q, id)
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
		name, err := validateName(in.Name)
		if err != nil {
			return err
		}
		in.Name = name
		conflict, err := txq.PersonNameConflict(ctx, db.PersonNameConflictParams{Name: in.Name, ExceptID: id})
		if err != nil {
			return err
		}
		if err := web.HTTPErrorFromReason(int(conflict.Status), conflict.Reason); err != nil {
			return err
		}
		validated, err := validate(in)
		if err != nil {
			return err
		}
		_, err = txq.UpdatePerson(ctx, db.UpdatePersonParams{ // the returned row cannot replace the post-update Get; only the error is needed
			Name: validated.Name, WeeklyCapacity: *validated.WeeklyCapacity, ID: id,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return web.Missing("person not found")
		}
		if db.UniqueViolation(err, "people.name") {
			return web.Conflict("person name already exists")
		}
		return err
	})
	if err != nil {
		return Person{}, err
	}
	return Get(ctx, q, id)
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

func fromOverrideRow(row db.PersonWeekOverride) Override {
	return Override{PersonID: row.PersonID, WeekStart: row.WeekStart, Capacity: row.Capacity}
}

func SetOverride(ctx context.Context, q *db.Queries, personID int64, weekStart string, capacity float64) (Override, error) {
	if _, err := weekly.Parse(weekStart); err != nil {
		return Override{}, err
	}
	if !web.NonNegativeFinite(capacity) {
		return Override{}, web.Invalid("hours cannot be negative")
	}
	var result Override
	err := q.InTx(ctx, func(txq *db.Queries) error {
		writeCtx, err := txq.PersonOverrideWriteContext(ctx, personID)
		if err != nil {
			return err
		}
		if err := web.HTTPErrorFromReason(int(writeCtx.Status), writeCtx.Reason); err != nil {
			return err
		}
		row, err := txq.SetPersonWeekOverride(ctx, db.SetPersonWeekOverrideParams{
			PersonID: personID, WeekStart: weekStart, Capacity: capacity,
		})
		if err == nil {
			result = fromOverrideRow(row)
		}
		return err
	})
	return result, err
}

func ListOverrides(ctx context.Context, q *db.Queries, personID int64) ([]Override, error) {
	rows, err := q.ListPersonWeekOverrides(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]Override, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromOverrideRow(row))
	}
	return out, nil
}

func ClearOverride(ctx context.Context, q *db.Queries, personID int64, weekStart string) error {
	if _, err := weekly.Parse(weekStart); err != nil {
		return err
	}
	return q.InTx(ctx, func(txq *db.Queries) error {
		writeCtx, err := txq.PersonOverrideWriteContext(ctx, personID)
		if err != nil {
			return err
		}
		if err := web.HTTPErrorFromReason(int(writeCtx.Status), writeCtx.Reason); err != nil {
			return err
		}
		rows, err := txq.DeletePersonWeekOverride(ctx, db.DeletePersonWeekOverrideParams{PersonID: personID, WeekStart: weekStart})
		if err != nil {
			return err
		}
		if rows == 0 {
			return web.Missing("override not found")
		}
		return nil
	})
}

func EffectiveCapacity(ctx context.Context, q *db.Queries, personID int64, weekStart string) (float64, error) {
	capacity, err := q.GetEffectiveCapacity(ctx, db.GetEffectiveCapacityParams{PersonID: personID, WeekStart: weekStart})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, web.Missing("person not found")
	}
	return capacity, err
}
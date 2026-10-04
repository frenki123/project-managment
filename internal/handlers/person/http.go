package personhandler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"cad-development/internal/db"
	"cad-development/internal/handlers/shared"
	"cad-development/internal/person"
	"cad-development/internal/views"
	"cad-development/internal/web"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/people", web.JSONList(q, listPeople))
	mux.HandleFunc("POST /api/v1/people", web.JSONCreate(q, person.Create))
	mux.HandleFunc("GET /api/v1/people/{id}", web.JSONGet(q, person.Get))
	mux.HandleFunc("PUT /api/v1/people/{id}", web.JSONUpdate(q, person.Update))
	mux.HandleFunc("DELETE /api/v1/people/{id}", web.JSONDelete(q, person.Delete))
	mux.HandleFunc("GET /api/v1/people/{id}/overrides", web.JSONList(q, listOverrides))
	mux.HandleFunc("PUT /api/v1/people/{id}/overrides/{weekStart}", setOverrideJSON(q))
	mux.HandleFunc("DELETE /api/v1/people/{id}/overrides/{weekStart}", clearOverrideJSON(q))
	mux.HandleFunc("GET /people", peoplePage(q))
	mux.HandleFunc("GET /people/new", newForm())
	mux.HandleFunc("POST /people", createHTML(q))
	mux.HandleFunc("GET /people/{id}/edit", editForm(q))
	mux.HandleFunc("POST /people/{id}", updateHTML(q))
	mux.HandleFunc("POST /people/{id}/delete", deleteHTML(q))
	mux.HandleFunc("POST /people/{id}/overrides", setOverrideHTML(q))
	mux.HandleFunc("POST /people/{id}/overrides/{weekStart}/delete", clearOverrideHTML(q))
}

func listPeople(ctx context.Context, q *db.Queries, _ *http.Request) (person.PeopleResponse, error) {
	list, err := person.List(ctx, q)
	if err != nil {
		return person.PeopleResponse{}, err
	}
	return person.PeopleResponse{People: list}, nil
}

func listOverrides(ctx context.Context, q *db.Queries, r *http.Request) (person.OverridesResponse, error) {
	id, err := web.PathID(r, "id")
	if err != nil {
		return person.OverridesResponse{}, err
	}
	list, err := person.ListOverrides(ctx, q, id)
	if err != nil {
		return person.OverridesResponse{}, err
	}
	return person.OverridesResponse{Overrides: list}, nil
}

func setOverrideJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := web.PathID(r, "id")
		if err != nil {
			web.WriteError(w, r, err)
			return
		}
		var body struct {
			Capacity *float64 `json:"capacity"`
		}
		if err := web.DecodeJSON(w, r, &body); err != nil {
			web.WriteError(w, r, err)
			return
		}
		if body.Capacity == nil {
			web.WriteError(w, r, web.Invalid("capacity is required"))
			return
		}
		override, err := person.SetOverride(r.Context(), q, id, r.PathValue("weekStart"), *body.Capacity)
		if err != nil {
			web.WriteError(w, r, err)
			return
		}
		web.JSON(w, http.StatusOK, override)
	}
}

func clearOverrideJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := web.PathID(r, "id")
		if err != nil {
			web.WriteError(w, r, err)
			return
		}
		if err := person.ClearOverride(r.Context(), q, id, r.PathValue("weekStart")); err != nil {
			web.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func peoplePage(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := person.List(r.Context(), q)
		if err != nil {
			web.WriteError(w, r, err)
			return
		}
		web.RenderPage(w, r, http.StatusOK, views.PeoplePage(views.PeoplePageData{People: list}))
	}
}

func formInput(r *http.Request) (person.Input, error) {
	vals := submittedFormValues(r)
	in := person.Input{Name: vals.Name}
	capacity, present, err := web.FormFloatValue(r, "weekly_capacity")
	if err != nil {
		return person.Input{}, err
	}
	if present && strings.TrimSpace(r.FormValue("weekly_capacity")) != "" {
		in.WeeklyCapacity = &capacity
	}
	return in, nil
}

func submittedFormValues(r *http.Request) views.PersonFormValues {
	return views.PersonFormValues{Name: r.FormValue("name"), WeeklyCapacity: r.FormValue("weekly_capacity")}
}

func renderPersonForm(w http.ResponseWriter, r *http.Request, vals views.PersonFormValues, action, title string, err error) {
	httpErr := web.HTTPErrorFrom(err)
	data := views.PersonFormData{
		Action: action, Title: title, Context: "Weekly capacity default", Person: vals, Error: httpErr.Message,
	}
	web.RenderFragment(w, r, httpErr.Status, views.PersonForm(data))
}

func renderEditPersonForm(w http.ResponseWriter, r *http.Request, q *db.Queries, id int64, err error) {
	var httpErr web.HTTPError
	if err != nil {
		httpErr = web.HTTPErrorFrom(err)
	} else {
		httpErr.Status = http.StatusOK
	}
	p, getErr := person.Get(r.Context(), q, id)
	if getErr != nil {
		web.WriteFragmentError(w, r, getErr)
		return
	}
	overrides, listErr := person.ListOverrides(r.Context(), q, id)
	if listErr != nil {
		web.WriteFragmentError(w, r, listErr)
		return
	}
	web.RenderFragment(w, r, httpErr.Status, views.PersonForm(personFormData(p, overrides, httpErr.Message)))
}

func personFormData(p person.Person, overrides []person.Override, errMsg string) views.PersonFormData {
	id := strconv.FormatInt(p.ID, 10)
	return views.PersonFormData{
		Action:  "/people/" + id,
		Title:   p.Name,
		Context: "Weekly capacity default",
		PersonID: p.ID,
		Person: views.PersonFormValues{
			Name:           p.Name,
			WeeklyCapacity: strconv.FormatFloat(p.WeeklyCapacity, 'f', -1, 64),
		},
		Overrides:    overrides,
		Error:        errMsg,
		DeleteAction: "/people/" + id + "/delete",
	}
}

func newForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := views.PersonFormData{
			Action:  "/people",
			Title:   "New person",
			Context: "Weekly capacity default",
			Person:  views.PersonFormValues{WeeklyCapacity: "40"},
		}
		web.RenderFragment(w, r, http.StatusOK, views.PersonForm(data))
	}
}

func editForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := shared.PathID(w, r)
		if !ok {
			return
		}
		renderEditPersonForm(w, r, q, id, nil)
	}
}

func createHTML(q *db.Queries) http.HandlerFunc {
	return shared.CreateForm(q, formInput, person.Create, func(w http.ResponseWriter, r *http.Request, err error) {
		renderPersonForm(w, r, submittedFormValues(r), "/people", "New person", err)
	}, func(person.Person) string {
		return "/people"
	})
}

func updateHTML(q *db.Queries) http.HandlerFunc {
	return shared.UpdateForm(q, formInput, func(ctx context.Context, q *db.Queries, id int64, in person.Input) (person.Person, error) {
		return person.Update(ctx, q, id, person.PatchFromInput(in))
	}, func(w http.ResponseWriter, r *http.Request, id int64, err error) {
		renderEditPersonForm(w, r, q, id, err)
	}, func(person.Person) string {
		return "/people"
	})
}

func deleteHTML(q *db.Queries) http.HandlerFunc {
	return shared.DeleteForm(q, person.Get, person.Delete, func(w http.ResponseWriter, r *http.Request, p person.Person, err error) {
		httpErr := web.HTTPErrorFrom(err)
		web.SetToast(w, httpErr.Message)
		renderEditPersonForm(w, r, q, p.ID, err)
	}, func(person.Person) string {
		return "/people"
	})
}

func setOverrideHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := shared.PathID(w, r)
		if !ok {
			return
		}
		capacity, present, err := web.FormFloatValue(r, "capacity")
		if err != nil {
			renderEditPersonForm(w, r, q, id, err)
			return
		}
		if !present || strings.TrimSpace(r.FormValue("capacity")) == "" {
			renderEditPersonForm(w, r, q, id, web.Invalid("capacity is required"))
			return
		}
		if _, err := person.SetOverride(r.Context(), q, id, r.FormValue("week_start"), capacity); err != nil {
			renderEditPersonForm(w, r, q, id, err)
			return
		}
		web.Redirect(w, r, "/people")
	}
}

func clearOverrideHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := shared.PathID(w, r)
		if !ok {
			return
		}
		if err := person.ClearOverride(r.Context(), q, id, r.PathValue("weekStart")); err != nil {
			renderEditPersonForm(w, r, q, id, err)
			return
		}
		web.Redirect(w, r, "/people")
	}
}
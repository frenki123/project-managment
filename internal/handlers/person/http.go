package personhandler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"cad-development/internal/db"
	"cad-development/internal/handlers/shared"
	"cad-development/internal/nullable"
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
	mux.HandleFunc("GET /people", peoplePage(q))
	mux.HandleFunc("GET /people/new", newForm())
	mux.HandleFunc("POST /people", createHTML(q))
	mux.HandleFunc("GET /people/{id}/edit", editForm(q))
	mux.HandleFunc("POST /people/{id}", updateHTML(q))
	mux.HandleFunc("POST /people/{id}/delete", deleteHTML(q))
}

func listPeople(ctx context.Context, q *db.Queries, _ *http.Request) (person.PeopleResponse, error) {
	list, err := person.List(ctx, q)
	if err != nil {
		return person.PeopleResponse{}, err
	}
	return person.PeopleResponse{People: list}, nil
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

func formPatch(r *http.Request) (person.Patch, error) {
	capacity, present, err := web.FormFloatValue(r, "weekly_capacity")
	if err != nil {
		return person.Patch{}, err
	}
	if !present || strings.TrimSpace(r.FormValue("weekly_capacity")) == "" {
		return person.Patch{}, web.Invalid("weekly capacity is required")
	}
	return person.Patch{
		Name:           nullable.Present(r.FormValue("name")),
		WeeklyCapacity: nullable.Present(capacity),
	}, nil
}

func submittedFormValues(r *http.Request) views.PersonFormValues {
	return views.PersonFormValues{Name: r.FormValue("name"), WeeklyCapacity: r.FormValue("weekly_capacity")}
}

func renderPersonForm(w http.ResponseWriter, r *http.Request, vals views.PersonFormValues, action, title, deleteAction string, err error) {
	httpErr := web.HTTPErrorFrom(err)
	data := views.PersonFormData{
		Action: action, Title: title, Person: vals, Error: httpErr.Message, DeleteAction: deleteAction,
	}
	web.RenderFragment(w, r, httpErr.Status, views.PersonForm(data))
}

func personFormData(p person.Person, errMsg string) views.PersonFormData {
	id := strconv.FormatInt(p.ID, 10)
	return views.PersonFormData{
		Action:  "/people/" + id,
		Title:   p.Name,
		Person: views.PersonFormValues{
			Name:           p.Name,
			WeeklyCapacity: strconv.FormatFloat(p.WeeklyCapacity, 'f', -1, 64),
		},
		Error:        errMsg,
		DeleteAction: "/people/" + id + "/delete",
	}
}

func newForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := views.PersonFormData{
			Action:  "/people",
			Title:   "New person",
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
		p, err := person.Get(r.Context(), q, id)
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		web.RenderFragment(w, r, http.StatusOK, views.PersonForm(personFormData(p, "")))
	}
}

func createHTML(q *db.Queries) http.HandlerFunc {
	return shared.CreateForm(q, formInput, person.Create, func(w http.ResponseWriter, r *http.Request, err error) {
		renderPersonForm(w, r, submittedFormValues(r), "/people", "New person", "", err)
	}, func(person.Person) string {
		return "/people"
	})
}

func updateHTML(q *db.Queries) http.HandlerFunc {
	return shared.UpdateForm(q, formPatch, person.Update, func(w http.ResponseWriter, r *http.Request, id int64, err error) {
		idStr := strconv.FormatInt(id, 10)
		renderPersonForm(w, r, submittedFormValues(r), "/people/"+idStr, "Edit person", "/people/"+idStr+"/delete", err)
	}, func(person.Person) string {
		return "/people"
	})
}

func deleteHTML(q *db.Queries) http.HandlerFunc {
	return shared.DeleteForm(q, person.Get, person.Delete, func(w http.ResponseWriter, r *http.Request, p person.Person, err error) {
		httpErr := web.HTTPErrorFrom(err)
		web.SetToast(w, httpErr.Message)
		web.RenderFragment(w, r, httpErr.Status, views.PersonForm(personFormData(p, httpErr.Message)))
	}, func(person.Person) string {
		return "/people"
	})
}
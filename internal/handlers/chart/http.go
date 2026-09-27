package charthandler

import (
	"fmt"
	"net/http"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/db"
	projectdomain "cad-development/internal/project"
	"cad-development/internal/views"
	"cad-development/internal/weekly"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/projects/{id}/s-curve", app.JSONGet(q, projectdomain.LoadSCurve))
	mux.HandleFunc("GET /chart", page(q))
}

func page(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := projectdomain.List(r.Context(), q)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		data := views.ChartPageData{Projects: projects}
		if len(projects) > 0 {
			ids := make([]int64, 0, len(projects))
			for _, p := range projects {
				ids = append(ids, p.ID)
			}
			fallback := app.PreferredProjectID(r, ids)
			if r.URL.Query().Get("project") == "" {
				app.Redirect(w, r, "/chart?project="+strconv.FormatInt(fallback, 10))
				return
			}
			id, err := selectedProjectID(r, fallback)
			if err != nil {
				app.WriteError(w, r, err)
				return
			}
			found := false
			for _, candidate := range ids {
				if candidate == id {
					found = true
					break
				}
			}
			if !found {
				app.Redirect(w, r, "/chart?project="+strconv.FormatInt(fallback, 10))
				return
			}
			app.RememberProject(w, id)
			curve, err := projectdomain.LoadSCurve(r.Context(), q, id)
			if err != nil {
				app.WriteError(w, r, err)
				return
			}
			data.SelectedProject = curve.Project
			data.HasProject = true
			data.Series = series(curve)
		}
		app.RenderPage(w, r, http.StatusOK, views.ChartPage(data))
	}
}

func selectedProjectID(r *http.Request, fallback int64) (int64, error) {
	value := r.URL.Query().Get("project")
	if value == "" {
		return fallback, nil
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return 0, app.Invalid("invalid project")
	}
	return id, nil
}

func series(curve projectdomain.SCurve) views.ChartSeries {
	labels := make([]string, 0, len(curve.Weeks))
	planned := make([]float64, 0, len(curve.Weeks))
	spent := make([]float64, 0, len(curve.Weeks))
	earned := make([]float64, 0, len(curve.Weeks))
	for _, week := range curve.Weeks {
		parsed, err := weekly.ParseWeekStart(weekly.WeekStart(week.WeekStart))
		if err != nil {
			labels = append(labels, week.WeekStart)
		} else {
			year, number := parsed.ISOWeek()
			labels = append(labels, fmt.Sprintf("W%d '%02d", number, year%100))
		}
		planned = append(planned, week.PlannedHours)
		spent = append(spent, week.SpentHours)
		earned = append(earned, week.EarnedHours)
	}
	return views.ChartSeries{
		Labels: labels,
		Datasets: []views.ChartDataset{
			{Label: "Planned (PV)", Data: planned},
			{Label: "Actual (AC)", Data: spent},
			{Label: "Earned (EV)", Data: earned},
		},
	}
}

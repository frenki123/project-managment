package charthandler

import (
	"net/http"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/db"
	projectdomain "cad-development/internal/project"
	"cad-development/internal/views"
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
			id, err := selectedProjectID(r, projects[0].ID)
			if err != nil {
				app.WriteError(w, r, err)
				return
			}
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
		labels = append(labels, week.WeekStart)
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

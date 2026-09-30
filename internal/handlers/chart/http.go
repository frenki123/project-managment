package charthandler

import (
	"fmt"
	"net/http"
	"strconv"

	"cad-development/internal/db"
	"cad-development/internal/project"
	"cad-development/internal/views"
	"cad-development/internal/web"
	"cad-development/internal/weekly"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/projects/{id}/s-curve", web.JSONGet(q, project.LoadSCurve))
	mux.HandleFunc("GET /chart", page(q))
}

func page(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := project.List(r.Context(), q)
		if err != nil {
			web.WriteError(w, r, err)
			return
		}
		data := views.ChartPageData{Projects: projects}
		if len(projects) > 0 {
			ids := make([]int64, 0, len(projects))
			for _, p := range projects {
				ids = append(ids, p.ID)
			}
			fallback := project.PreferredProjectID(r, ids)
			if r.URL.Query().Get("project") == "" {
				web.Redirect(w, r, "/chart?project="+strconv.FormatInt(fallback, 10))
				return
			}
			id, err := selectedProjectID(r)
			if err != nil {
				web.WriteError(w, r, err)
				return
			}
			curve, err := project.LoadSCurve(r.Context(), q, id)
			if err != nil {
				web.WriteError(w, r, err)
				return
			}
			project.RememberProject(w, id)
			data.SelectedProject = curve.Project
			data.HasProject = true
			data.Series = series(curve)
		}
		web.RenderPage(w, r, http.StatusOK, views.ChartPage(data))
	}
}

func selectedProjectID(r *http.Request) (int64, error) {
	value := r.URL.Query().Get("project")
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return 0, web.Invalid("invalid project")
	}
	return id, nil
}

func series(curve project.SCurve) views.ChartSeries {
	labels := make([]string, 0, len(curve.Weeks))
	planned := make([]float64, 0, len(curve.Weeks))
	spent := make([]float64, 0, len(curve.Weeks))
	earned := make([]float64, 0, len(curve.Weeks))
	for _, week := range curve.Weeks {
		parsed := weekly.WeekStart(week.WeekStart).Time()
		if parsed.IsZero() {
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
			{Label: "Planned [h]", Data: planned},
			{Label: "Spent [h]", Data: spent},
			{Label: "Earned [h]", Data: earned},
		},
	}
}

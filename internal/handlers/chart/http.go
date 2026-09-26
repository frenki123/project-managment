package charthandler

import (
	"net/http"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/views"
)

func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /chart", page)
}

func page(w http.ResponseWriter, r *http.Request) {
	app.RenderPage(w, r, http.StatusOK, views.ChartPage(views.ChartPageData{Series: demoSeries()}))
}

func demoSeries() views.ChartSeries {
	labels := make([]string, 0, 8)
	planned := make([]float64, 0, 8)
	actual := make([]float64, 0, 8)
	for week := range 8 {
		labels = append(labels, "W"+strconv.Itoa(week+1))
		planned = append(planned, float64(week)*10)
		actual = append(actual, float64(week)*12)
	}
	return views.ChartSeries{
		Labels: labels,
		Datasets: []views.ChartDataset{
			{Label: "Planned", Data: planned},
			{Label: "Actual", Data: actual},
		},
	}
}

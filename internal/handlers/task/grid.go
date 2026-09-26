package taskhandler

import (
	"net/http"
	"strconv"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	handlererrors "cad-development/internal/handlers/errors"
	taskdomain "cad-development/internal/task"
	"cad-development/internal/views"
)

func ParseFilter(r *http.Request) (projectKey string, subprojectID *int64, err error) {
	projectKey = r.FormValue("project")
	subprojectID, err = app.FormInt64Checked(r, "subproject")
	return projectKey, subprojectID, err
}

func gridPage(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		RenderGrid(w, r, q, "", time.Now())
	}
}

func RenderGrid(w http.ResponseWriter, r *http.Request, q *db.Queries, errMsg string, currentTime time.Time) {
	pk, sid, err := ParseFilter(r)
	if err != nil {
		handlererrors.WriteError(w, r, err)
		return
	}
	grid, err := taskdomain.LoadGrid(r.Context(), q, pk, sid, currentTime)
	if err != nil {
		handlererrors.WriteError(w, r, err)
		return
	}
	data := toViewGrid(grid, errMsg)
	if app.IsHTMX(r) {
		handlererrors.Render(w, r, http.StatusOK, views.Grid(data))
		return
	}
	handlererrors.Render(w, r, http.StatusOK, views.GridPage(data))
}

func toViewGrid(grid taskdomain.Grid, errMsg string) views.GridData {
	data := views.GridData{
		FilterProject:    grid.FilterProject,
		FilterSubproject: grid.FilterSubproject,
		Ideas:            grid.Ideas,
		POName:           grid.POName,
		BudgetHours:      grid.BudgetHours,
		PlannedHours:     grid.PlannedHours,
		SpentHours:       grid.SpentHours,
		ProgressPct:      grid.ProgressPct,
		Overrun:          grid.Overrun,
		LastMonth:        grid.LastMonth,
		LastMonthUnlock:  grid.LastMonthUnlock,
		Error:            errMsg,
		Totals: views.GridTotalsData{
			FilterProject: grid.FilterProject, FilterSubproject: grid.FilterSubproject,
			POName: grid.POName, BudgetHours: grid.BudgetHours, PlannedHours: grid.PlannedHours,
			SpentHours: grid.SpentHours, ProgressPct: grid.ProgressPct, Overrun: grid.Overrun,
		},
	}
	for _, week := range grid.Weeks {
		data.Weeks = append(data.Weeks, views.WeekHeader{
			Start: string(week.Start), Number: week.Number, Date: week.Date,
			Month: week.Month, MonthLabel: week.MonthLabel,
		})
	}
	for _, option := range grid.Projects {
		data.Projects = append(data.Projects, views.Option{Value: option.Value, Label: option.Label, Selected: option.Selected})
	}
	for _, option := range grid.Subprojects {
		data.Subprojects = append(data.Subprojects, views.Option{Value: option.Value, Label: option.Label, Selected: option.Selected})
	}
	for _, month := range grid.PastMonths {
		label := month
		if parsed, err := time.Parse("2006-01", month); err == nil {
			label = parsed.Format("January 2006")
		}
		data.PastMonths = append(data.PastMonths, views.Option{Value: month, Label: label})
	}
	for _, row := range grid.Rows {
		viewRow := views.TaskRow{
			ID:          row.ID,
			Name:        row.Name,
			ProjectName: row.ProjectName,
			Subproject:  row.Subproject,
			TotalHours:  row.TotalHours,
			SpentHours:  row.SpentHours,
			Progress:    row.Progress,
			DetailPath:  "/tasks/" + strconv.FormatInt(row.ID, 10),
		}
		for _, cell := range row.Cells {
			viewRow.Cells = append(viewRow.Cells, views.WeekCell{
				WeekStart: string(cell.WeekStart),
				Planned:   cell.Planned,
				Spent:     cell.Spent,
				Progress:  cell.Progress,
				Stored:    cell.Stored,
				SavePath:  "/tasks/" + strconv.FormatInt(row.ID, 10) + "/weeks/" + string(cell.WeekStart),
				Locked:    cell.Locked,
			})
		}
		data.Rows = append(data.Rows, viewRow)
	}
	if len(data.Weeks) > 0 {
		data.WeekTotals = make([]views.WeekTotal, len(data.Weeks))
		for _, row := range data.Rows {
			for i, cell := range row.Cells {
				data.WeekTotals[i].Planned += cell.Planned
				data.WeekTotals[i].Spent += cell.Spent
				data.WeekTotals[i].Earned += row.TotalHours * cell.Progress / 100
			}
		}
		var planned, spent float64
		for i := range data.WeekTotals {
			planned += data.WeekTotals[i].Planned
			spent += data.WeekTotals[i].Spent
			data.WeekTotals[i].CumulativePlanned = planned
			data.WeekTotals[i].CumulativeSpent = spent
		}
	}
	return data
}

package task

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cad-development/internal/apperr"
	"cad-development/internal/db"
	"cad-development/internal/httpx"
	"cad-development/internal/monthlock"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/views"
	"cad-development/internal/weekly"
)

func ParseFilter(r *http.Request) (projectKey string, subprojectID *int64, err error) {
	projectKey = r.FormValue("project")
	subprojectID, err = httpx.FormInt64Checked(r, "subproject")
	return projectKey, subprojectID, err
}

func LoadGrid(ctx context.Context, q *db.Queries, projectKey string, subprojectID *int64, now time.Time) (views.GridData, error) {
	projects, err := project.List(ctx, q)
	if err != nil {
		return views.GridData{}, err
	}
	if projectKey == "" && len(projects) > 0 {
		projectKey = strconv.FormatInt(projects[0].ID, 10)
	}
	if projectKey == "" {
		projectKey = "ideas"
	}

	data := views.GridData{
		FilterProject:    projectKey,
		FilterSubproject: "",
		Ideas:            projectKey == "ideas",
		Projects:         projectOptions(projects, projectKey),
		LastMonth:        monthlock.PreviousMonth(now),
	}
	unlocked, err := monthlock.UnlockedSet(ctx, q)
	if err != nil {
		return views.GridData{}, err
	}
	data.LastMonthUnlock = unlocked[data.LastMonth]

	if data.Ideas {
		tasks, err := ListIdeas(ctx, q)
		if err != nil {
			return views.GridData{}, err
		}
		for _, t := range tasks {
			data.Rows = append(data.Rows, views.TaskRow{
				ID:          t.ID,
				Name:        t.Name,
				ProjectName: "",
				Subproject:  "",
				DetailPath:  fmt.Sprintf("/tasks/%d", t.ID),
			})
		}
		return data, nil
	}

	pid, err := strconv.ParseInt(projectKey, 10, 64)
	if err != nil || pid < 1 {
		return views.GridData{}, apperr.New(http.StatusBadRequest, "invalid project")
	}
	proj, err := project.Get(ctx, q, pid)
	if err != nil {
		return views.GridData{}, err
	}
	subs, err := subproject.ListByProject(ctx, q, pid)
	if err != nil {
		return views.GridData{}, err
	}
	data.Subprojects = subprojectOptions(subs, subprojectID)
	if subprojectID != nil {
		data.FilterSubproject = strconv.FormatInt(*subprojectID, 10)
	}

	start, err := weekly.ParseDate(proj.StartDate)
	if err != nil {
		return views.GridData{}, err
	}
	end, err := weekly.ParseDate(proj.EndDate)
	if err != nil {
		return views.GridData{}, err
	}
	data.Weeks = weekly.WeekStarts(start, end)
	data.POName = proj.PurchaseOrderName
	data.BudgetHours = proj.TotalHours

	subNames := map[int64]string{}
	for _, s := range subs {
		subNames[s.ID] = s.Name
	}

	var tasks []Task
	if subprojectID != nil {
		sp, err := subproject.Get(ctx, q, *subprojectID)
		if err != nil {
			return views.GridData{}, err
		}
		if sp.ProjectID != pid {
			return views.GridData{}, apperr.New(http.StatusBadRequest, "subproject does not belong to project")
		}
		data.BudgetHours = sp.TotalHours
		data.ProgressPct = nil
		tasks, err = ListBySubproject(ctx, q, *subprojectID)
		if err != nil {
			return views.GridData{}, err
		}
	} else {
		zero := 0.0
		data.ProgressPct = &zero
		tasks, err = ListByProject(ctx, q, pid)
		if err != nil {
			return views.GridData{}, err
		}
	}

	var weeks []db.TaskWeek
	if subprojectID != nil {
		weeks, err = q.ListTaskWeeksBySubproject(ctx, sql.NullInt64{Int64: *subprojectID, Valid: true})
	} else {
		weeks, err = q.ListTaskWeeksByProject(ctx, sql.NullInt64{Int64: pid, Valid: true})
	}
	if err != nil {
		return views.GridData{}, err
	}
	byTask := map[int64][]db.TaskWeek{}
	for _, w := range weeks {
		byTask[w.TaskID] = append(byTask[w.TaskID], w)
	}

	var progressPct float64
	for _, t := range tasks {
		tweeks := byTask[t.ID]
		planned, spent, prog := weekly.Totals(tweeks)
		cellByWeek := map[string]db.TaskWeek{}
		for _, w := range tweeks {
			cellByWeek[w.WeekStart] = w
		}
		row := views.TaskRow{
			ID:          t.ID,
			Name:        t.Name,
			ProjectName: proj.Name,
			TotalHours:  planned,
			SpentHours:  spent,
			Progress:    prog,
			DetailPath:  fmt.Sprintf("/tasks/%d", t.ID),
		}
		if t.SubprojectID != nil {
			row.Subproject = subNames[*t.SubprojectID]
		}
		prev := 0.0
		for _, ws := range data.Weeks {
			cw := cellByWeek[ws]
			stored := weekly.StoredProgress(cw)
			eff := weekly.EffectiveFromPrev(prev, stored)
			if stored != nil {
				prev = *stored
			}
			row.Cells = append(row.Cells, views.WeekCell{
				WeekStart: ws,
				Planned:   cw.PlannedHours,
				Spent:     cw.SpentHours,
				Progress:  eff,
				SavePath:  fmt.Sprintf("/tasks/%d/weeks/%s", t.ID, ws),
				Locked:    monthlock.WeekLocked(ws, now, unlocked),
			})
		}
		data.Rows = append(data.Rows, row)
		data.PlannedHours += planned
		data.SpentHours += spent
		if data.ProgressPct != nil && proj.TotalHours > 0 {
			progressPct += (planned / proj.TotalHours) * prog
		}
	}
	if data.ProgressPct != nil {
		data.ProgressPct = &progressPct
	}
	data.Overrun = data.PlannedHours > data.BudgetHours

	from := start
	for _, p := range projects {
		ps, err := weekly.ParseDate(p.StartDate)
		if err != nil {
			continue
		}
		if ps.Before(from) {
			from = ps
		}
	}
	for _, ym := range monthlock.PastMonths(from, now) {
		data.PastMonths = append(data.PastMonths, views.Option{Value: ym, Label: ym})
	}
	return data, nil
}

func projectOptions(projects []project.Project, selected string) []views.Option {
	out := []views.Option{{Value: "ideas", Label: "Ideas", Selected: selected == "ideas"}}
	for _, p := range projects {
		v := strconv.FormatInt(p.ID, 10)
		out = append(out, views.Option{Value: v, Label: p.Name, Selected: v == selected})
	}
	return out
}

func subprojectOptions(subs []subproject.Subproject, selected *int64) []views.Option {
	out := []views.Option{{Value: "", Label: "All", Selected: selected == nil}}
	for _, s := range subs {
		v := strconv.FormatInt(s.ID, 10)
		sel := selected != nil && *selected == s.ID
		out = append(out, views.Option{Value: v, Label: s.Name, Selected: sel})
	}
	return out
}

package task

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"cad-development/internal/db"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/weekly"
)

type Grid struct {
	Projects                 []Option
	Subprojects              []Option
	FilterProject            string
	FilterSubproject         string
	Ideas                    bool
	SummaryOnly              bool
	ProjectName              string
	SubprojectName           string
	StartDate                string
	EndDate                  string
	Weeks                    []weekly.WeekInfo
	WeekTotals               []GridWeekTotal
	Rows                     []GridRow
	POName                   string
	BudgetHours              float64
	PlannedHours             float64
	SpentHours               float64
	ProgressPct              *float64
	Overrun                  bool
	HistoricalEditingAllowed bool
}

type Option struct {
	Value    string
	Label    string
	Selected bool
}

type GridRow struct {
	ID          int64
	Name        string
	ProjectName string
	Subproject  string
	TotalHours  float64
	SpentHours  float64
	Progress    float64
	Status      string
	DetailPath  string
	Cells       []GridCell
}

type GridCell struct {
	WeekStart weekly.WeekStart
	Planned   float64
	Spent     float64
	Progress  float64
	Stored    bool
	Locked    bool
	SavePath  string
	Error     string
}

type GridWeekTotal struct {
	Planned float64
	Spent   float64
}

func LoadGrid(ctx context.Context, q *db.Queries, filter Filter, now time.Time, allowHistoricalEditing bool) (Grid, error) {
	projects, err := project.List(ctx, q)
	if err != nil {
		return Grid{}, err
	}
	if err := filter.Validate(ctx, q); err != nil {
		return Grid{}, err
	}

	data := Grid{
		FilterProject:    filterProjectKey(filter),
		FilterSubproject: "",
		Ideas:            filter.Ideas,
		ProjectName:      "All tasks",
		Projects:         projectOptions(projects, filterProjectKey(filter)),
	}
	if filter.All && filter.Subproject == nil {
		data.SummaryOnly = true
		tasks, err := List(ctx, q)
		if err != nil {
			return Grid{}, err
		}
		projectNames := make(map[int64]string, len(projects))
		for _, p := range projects {
			projectNames[p.ID] = p.Name
		}
		subs, err := subproject.List(ctx, q)
		if err != nil {
			return Grid{}, err
		}
		subprojectNames := make(map[int64]string, len(subs))
		for _, s := range subs {
			subprojectNames[s.ID] = s.Name
		}
		for _, t := range tasks {
			row := GridRow{
				ID: t.ID, Name: t.Name, Status: t.Status,
				TotalHours: t.TotalHours, SpentHours: t.SpentHours,
				Progress: t.Progress, DetailPath: "/tasks/" + strconv.FormatInt(t.ID, 10),
			}
			if t.ProjectID != nil {
				row.ProjectName = projectNames[*t.ProjectID]
			}
			if t.SubprojectID != nil {
				row.Subproject = subprojectNames[*t.SubprojectID]
			}
			data.Rows = append(data.Rows, row)
		}
		return data, nil
	}
	if filter.Ideas && filter.Subproject == nil {
		tasks, err := ListIdeas(ctx, q)
		if err != nil {
			return Grid{}, err
		}
		for _, t := range tasks {
			data.Rows = append(data.Rows, GridRow{
				ID:          t.ID,
				Name:        t.Name,
				ProjectName: "",
				Subproject:  "",
				DetailPath:  "/tasks/" + strconv.FormatInt(t.ID, 10),
			})
		}
		return data, nil
	}
	data.HistoricalEditingAllowed = allowHistoricalEditing
	pid := filter.ID
	if filter.Subproject != nil && pid == 0 {
		sp, err := subproject.Get(ctx, q, *filter.Subproject)
		if err != nil {
			return Grid{}, err
		}
		pid = sp.ProjectID
	}
	proj, err := project.Get(ctx, q, pid)
	if err != nil {
		return Grid{}, err
	}
	subs, err := subproject.ListByProject(ctx, q, pid)
	if err != nil {
		return Grid{}, err
	}
	data.Subprojects = subprojectOptions(subs, filter.Subproject)
	data.ProjectName = proj.Name
	data.StartDate = proj.StartDate
	data.EndDate = proj.EndDate
	var sp subproject.Subproject
	if filter.Subproject != nil {
		sp, err = subproject.Get(ctx, q, *filter.Subproject)
		if err != nil {
			return Grid{}, err
		}
	}
	if filter.Subproject != nil {
		data.FilterSubproject = strconv.FormatInt(*filter.Subproject, 10)
		data.SubprojectName = sp.Name
	}

	start, err := weekly.ParseDate(proj.StartDate)
	if err != nil {
		return Grid{}, err
	}
	end, err := weekly.ParseDate(proj.EndDate)
	if err != nil {
		return Grid{}, err
	}
	for _, ws := range weekly.WeekStarts(start, end) {
		info, err := weekly.Info(ws)
		if err != nil {
			return Grid{}, err
		}
		data.Weeks = append(data.Weeks, info)
	}
	data.POName = proj.PurchaseOrderName
	data.BudgetHours = proj.TotalHours
	addWeekTotal := func(total GridWeekTotal) { data.WeekTotals = append(data.WeekTotals, total) }

	weeks, err := q.ListTaskWeeksByProject(ctx, sql.NullInt64{Int64: pid, Valid: true})
	if err != nil {
		return Grid{}, err
	}
	byTask := map[int64][]db.TaskWeek{}
	for _, w := range weeks {
		byTask[w.TaskID] = append(byTask[w.TaskID], w)
	}

	subNames := map[int64]string{}
	for _, s := range subs {
		subNames[s.ID] = s.Name
	}

	var tasks []Task
	if filter.Subproject != nil {
		data.BudgetHours = sp.TotalHours
		tasks, err = ListBySubproject(ctx, q, *filter.Subproject)
		if err != nil {
			return Grid{}, err
		}
		summary, err := q.GetSubprojectTotals(ctx, *filter.Subproject)
		if err != nil {
			return Grid{}, err
		}
		data.PlannedHours = summary.PlannedHours
		data.SpentHours = summary.SpentHours
		rows, err := q.ListSubprojectWeekTotals(ctx, *filter.Subproject)
		if err != nil {
			return Grid{}, err
		}
		for _, row := range rows {
			addWeekTotal(GridWeekTotal{
				Planned: row.PlannedHours, Spent: row.SpentHours,
			})
		}
	} else {
		data.ProgressPct = new(0.0)
		tasks, err = ListByProject(ctx, q, pid)
		if err != nil {
			return Grid{}, err
		}
		summary, err := q.GetProjectTotals(ctx, pid)
		if err != nil {
			return Grid{}, err
		}
		data.PlannedHours = summary.PlannedHours
		data.SpentHours = summary.SpentHours
		*data.ProgressPct = summary.Progress
		rows, err := q.ListProjectWeekTotals(ctx, pid)
		if err != nil {
			return Grid{}, err
		}
		for _, row := range rows {
			addWeekTotal(GridWeekTotal{
				Planned: row.PlannedHours, Spent: row.SpentHours,
			})
		}
	}

	for _, t := range tasks {
		tweeks := byTask[t.ID]
		requested := make([]string, 0, len(data.Weeks))
		for _, info := range data.Weeks {
			requested = append(requested, string(info.Start))
		}
		effective := weekly.EffectiveProgress(tweeks, requested)
		cellByWeek := map[string]db.TaskWeek{}
		for _, w := range tweeks {
			cellByWeek[w.WeekStart] = w
		}
		row := GridRow{
			ID:          t.ID,
			Name:        t.Name,
			ProjectName: proj.Name,
			TotalHours:  t.TotalHours,
			SpentHours:  t.SpentHours,
			Progress:    t.Progress,
			Status:      t.Status,
			DetailPath:  "/tasks/" + strconv.FormatInt(t.ID, 10),
		}
		if t.SubprojectID != nil {
			row.Subproject = subNames[*t.SubprojectID]
		}
		for _, info := range data.Weeks {
			ws := info.Start
			cw := cellByWeek[string(ws)]
			row.Cells = append(row.Cells, GridCell{
				WeekStart: ws,
				Planned:   cw.PlannedHours,
				Spent:     cw.SpentHours,
				Progress:  effective[string(ws)],
				Stored:    cw.Progress.Valid,
				Locked:    !allowHistoricalEditing && weekly.IsWeekLocked(string(ws), now),
				SavePath:  "/tasks/" + strconv.FormatInt(t.ID, 10) + "/weeks/" + string(ws),
			})
		}
		data.Rows = append(data.Rows, row)
	}
	data.Overrun = data.PlannedHours > data.BudgetHours

	return data, nil
}

func projectOptions(projects []project.Project, selected string) []Option {
	out := []Option{{Value: "all", Label: "All tasks", Selected: selected == "all"}, {Value: "ideas", Label: "Ideas", Selected: selected == "ideas"}}
	for _, p := range projects {
		v := strconv.FormatInt(p.ID, 10)
		out = append(out, Option{Value: v, Label: p.Name, Selected: v == selected})
	}
	return out
}

func filterProjectKey(filter Filter) string {
	if filter.Ideas {
		return "ideas"
	}
	if filter.All {
		return "all"
	}
	return strconv.FormatInt(filter.ID, 10)
}

func subprojectOptions(subs []subproject.Subproject, selected *int64) []Option {
	out := []Option{{Value: "", Label: "All", Selected: selected == nil}}
	for _, s := range subs {
		v := strconv.FormatInt(s.ID, 10)
		sel := selected != nil && *selected == s.ID
		out = append(out, Option{Value: v, Label: s.Name, Selected: sel})
	}
	return out
}

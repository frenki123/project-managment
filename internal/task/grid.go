package task

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"cad-development/internal/db"
	"cad-development/internal/monthlock"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/weekly"
)

type Grid struct {
	Projects         []GridOption
	Subprojects      []GridOption
	FilterProject    string
	FilterSubproject string
	Ideas            bool
	Weeks            []weekly.WeekStart
	Rows             []GridRow
	POName           string
	BudgetHours      float64
	PlannedHours     float64
	SpentHours       float64
	ProgressPct      *float64
	Overrun          bool
	LastMonth        string
	LastMonthUnlock  bool
	PastMonths       []string
}

type GridOption struct {
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
	Cells       []GridCell
}

type GridCell struct {
	WeekStart weekly.WeekStart
	Planned   float64
	Spent     float64
	Progress  float64
	Locked    bool
}

type taskTotal struct {
	Planned  float64
	Spent    float64
	Progress float64
}

type projectFilter struct {
	Value string
	ID    int64
	Ideas bool
}

func LoadGrid(ctx context.Context, q *db.Queries, projectKey string, subprojectID *int64, now time.Time) (Grid, error) {
	projects, err := project.List(ctx, q)
	if err != nil {
		return Grid{}, err
	}
	filter, err := parseProjectFilter(projectKey, projects)
	if err != nil {
		return Grid{}, err
	}

	data := Grid{
		FilterProject:    filter.Value,
		FilterSubproject: "",
		Ideas:            filter.Ideas,
		Projects:         projectOptions(projects, filter.Value),
		LastMonth:        string(monthlock.PreviousMonth(now)),
	}
	unlocked, err := monthlock.UnlockedSet(ctx, q)
	if err != nil {
		return Grid{}, err
	}
	data.LastMonthUnlock = unlocked.Contains(monthlock.YearMonth(data.LastMonth))

	if filter.Ideas {
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
			})
		}
		return data, nil
	}

	pid := filter.ID
	proj, err := project.Get(ctx, q, pid)
	if err != nil {
		return Grid{}, err
	}
	subs, err := subproject.ListByProject(ctx, q, pid)
	if err != nil {
		return Grid{}, err
	}
	data.Subprojects = subprojectOptions(subs, subprojectID)
	if subprojectID != nil {
		data.FilterSubproject = strconv.FormatInt(*subprojectID, 10)
	}

	start, err := weekly.ParseDate(proj.StartDate)
	if err != nil {
		return Grid{}, err
	}
	end, err := weekly.ParseDate(proj.EndDate)
	if err != nil {
		return Grid{}, err
	}
	data.Weeks = weekly.WeekStarts(start, end)
	data.POName = proj.PurchaseOrderName
	data.BudgetHours = proj.TotalHours

	subNames := map[int64]string{}
	for _, s := range subs {
		subNames[s.ID] = s.Name
	}

	var tasks []Task
	totalsByTask := map[int64]taskTotal{}
	if subprojectID != nil {
		sp, err := subproject.Get(ctx, q, *subprojectID)
		if err != nil {
			return Grid{}, err
		}
		if sp.ProjectID != pid {
			return Grid{}, Invalid("subproject does not belong to project")
		}
		data.BudgetHours = sp.TotalHours
		data.ProgressPct = nil
		tasks, err = ListBySubproject(ctx, q, *subprojectID)
		if err != nil {
			return Grid{}, err
		}
		summary, err := q.GetSubprojectTotals(ctx, *subprojectID)
		if err != nil {
			return Grid{}, err
		}
		data.PlannedHours = summary.PlannedHours
		data.SpentHours = summary.SpentHours
		totalRows, err := q.ListTaskTotalsBySubproject(ctx, *subprojectID)
		if err != nil {
			return Grid{}, err
		}
		for _, total := range totalRows {
			totalsByTask[total.TaskID] = taskTotal{Planned: total.PlannedHours, Spent: total.SpentHours, Progress: total.Progress}
		}
	} else {
		tasks, err = ListByProject(ctx, q, pid)
		if err != nil {
			return Grid{}, err
		}
		data.ProgressPct = new(0.0)
		summary, err := q.GetProjectTotals(ctx, pid)
		if err != nil {
			return Grid{}, err
		}
		data.PlannedHours = summary.PlannedHours
		data.SpentHours = summary.SpentHours
		*data.ProgressPct = summary.Progress
		totalRows, err := q.ListTaskTotalsByProject(ctx, pid)
		if err != nil {
			return Grid{}, err
		}
		for _, total := range totalRows {
			totalsByTask[total.TaskID] = taskTotal{Planned: total.PlannedHours, Spent: total.SpentHours, Progress: total.Progress}
		}
	}

	var weeks []db.TaskWeek
	if subprojectID != nil {
		weeks, err = q.ListTaskWeeksBySubproject(ctx, sql.NullInt64{Int64: *subprojectID, Valid: true})
	} else {
		weeks, err = q.ListTaskWeeksByProject(ctx, sql.NullInt64{Int64: pid, Valid: true})
	}
	if err != nil {
		return Grid{}, err
	}
	byTask := map[int64][]db.TaskWeek{}
	for _, w := range weeks {
		byTask[w.TaskID] = append(byTask[w.TaskID], w)
	}

	for _, t := range tasks {
		tweeks := byTask[t.ID]
		total := totalsByTask[t.ID]
		cellByWeek := map[string]db.TaskWeek{}
		for _, w := range tweeks {
			cellByWeek[w.WeekStart] = w
		}
		row := GridRow{
			ID:          t.ID,
			Name:        t.Name,
			ProjectName: proj.Name,
			TotalHours:  total.Planned,
			SpentHours:  total.Spent,
			Progress:    total.Progress,
		}
		if t.SubprojectID != nil {
			row.Subproject = subNames[*t.SubprojectID]
		}
		prev := 0.0
		for _, ws := range data.Weeks {
			cw := cellByWeek[string(ws)]
			stored := weekly.StoredProgress(cw)
			eff := weekly.EffectiveFromPrev(prev, stored)
			if stored != nil {
				prev = *stored
			}
			row.Cells = append(row.Cells, GridCell{
				WeekStart: ws,
				Planned:   cw.PlannedHours,
				Spent:     cw.SpentHours,
				Progress:  eff,
				Locked:    monthlock.WeekLocked(string(ws), now, unlocked),
			})
		}
		data.Rows = append(data.Rows, row)
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
		data.PastMonths = append(data.PastMonths, string(ym))
	}
	return data, nil
}

func projectOptions(projects []project.Project, selected string) []GridOption {
	out := []GridOption{{Value: "ideas", Label: "Ideas", Selected: selected == "ideas"}}
	for _, p := range projects {
		v := strconv.FormatInt(p.ID, 10)
		out = append(out, GridOption{Value: v, Label: p.Name, Selected: v == selected})
	}
	return out
}

func parseProjectFilter(value string, projects []project.Project) (projectFilter, error) {
	if value == "" && len(projects) > 0 {
		value = strconv.FormatInt(projects[0].ID, 10)
	}
	if value == "" || value == "ideas" {
		return projectFilter{Value: "ideas", Ideas: true}, nil
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return projectFilter{}, Invalid("invalid project")
	}
	return projectFilter{Value: value, ID: id}, nil
}

func subprojectOptions(subs []subproject.Subproject, selected *int64) []GridOption {
	out := []GridOption{{Value: "", Label: "All", Selected: selected == nil}}
	for _, s := range subs {
		v := strconv.FormatInt(s.ID, 10)
		sel := selected != nil && *selected == s.ID
		out = append(out, GridOption{Value: v, Label: s.Name, Selected: sel})
	}
	return out
}

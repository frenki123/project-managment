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
	Kind                     ViewKind
	FilterProject            string
	FilterSubproject         string
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

type ViewKind uint8

const (
	ViewAll ViewKind = iota
	ViewIdeas
	ViewProject
	ViewSubproject
)

func (kind ViewKind) IsWeekly() bool {
	return kind == ViewProject || kind == ViewSubproject
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
	resolved, err := filter.Resolve(ctx, q)
	if err != nil {
		return Grid{}, err
	}
	return LoadResolvedGrid(ctx, q, resolved, now, allowHistoricalEditing)
}

func LoadResolvedGrid(ctx context.Context, q *db.Queries, resolved ResolvedFilter, now time.Time, allowHistoricalEditing bool) (Grid, error) {
	filter := resolved.Filter
	projects, err := project.List(ctx, q)
	if err != nil {
		return Grid{}, err
	}

	kind := ViewProject
	switch {
	case filter.All && filter.Subproject == nil:
		kind = ViewAll
	case filter.Ideas && filter.Subproject == nil:
		kind = ViewIdeas
	case filter.Subproject != nil:
		kind = ViewSubproject
	}
	data := Grid{
		FilterProject:    filterProjectKey(filter),
		FilterSubproject: "",
		Kind:             kind,
		ProjectName:      "All tasks",
		Projects:         projectOptions(projects, filterProjectKey(filter)),
	}
	if kind == ViewAll {
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
	if kind == ViewIdeas {
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
	pid := resolved.Project.ID
	proj := project.FromDB(*resolved.Project)
	subs, err := subproject.ListByProject(ctx, q, pid)
	if err != nil {
		return Grid{}, err
	}
	data.Subprojects = subprojectOptions(subs, filter.Subproject)
	data.ProjectName = proj.Name
	data.StartDate = proj.StartDate
	data.EndDate = proj.EndDate
	if resolved.Subproject != nil {
		data.FilterSubproject = strconv.FormatInt(*filter.Subproject, 10)
		data.SubprojectName = resolved.Subproject.Name
	}

	data.POName = proj.PurchaseOrderName
	data.BudgetHours = proj.TotalHours
	weeks, err := q.ListTaskWeeksByProject(ctx, sql.NullInt64{Int64: pid, Valid: true})
	if err != nil {
		return Grid{}, err
	}
	byTask := map[int64][]db.VTaskWeekEffective{}
	for _, w := range weeks {
		byTask[w.TaskID] = append(byTask[w.TaskID], w)
	}

	subNames := map[int64]string{}
	for _, s := range subs {
		subNames[s.ID] = s.Name
	}

	var tasks []Task
	if resolved.Subproject != nil {
		data.BudgetHours = resolved.Subproject.TotalHours
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
			info, err := weekly.Info(weekly.WeekStart(row.WeekStart))
			if err != nil {
				return Grid{}, err
			}
			data.Weeks = append(data.Weeks, info)
			data.WeekTotals = append(data.WeekTotals, GridWeekTotal{
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
			info, err := weekly.Info(weekly.WeekStart(row.WeekStart))
			if err != nil {
				return Grid{}, err
			}
			data.Weeks = append(data.Weeks, info)
			data.WeekTotals = append(data.WeekTotals, GridWeekTotal{
				Planned: row.PlannedHours, Spent: row.SpentHours,
			})
		}
	}

	for _, t := range tasks {
		tweeks := byTask[t.ID]
		cellByWeek := map[string]db.VTaskWeekEffective{}
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
		progress := 0.0
		for _, info := range data.Weeks {
			ws := info.Start
			cw := cellByWeek[string(ws)]
			if cw.WeekStart != "" {
				progress = cw.EffectiveProgress
			}
			row.Cells = append(row.Cells, GridCell{
				WeekStart: ws,
				Planned:   cw.PlannedHours,
				Spent:     cw.SpentHours,
				Progress:  progress,
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

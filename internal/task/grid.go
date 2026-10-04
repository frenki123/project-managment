package task

import (
	"context"
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

func IsWeekly(kind ViewKind) bool {
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
	resolved, err := ResolveFilter(ctx, q, filter)
	if err != nil {
		return Grid{}, err
	}
	return LoadResolvedGrid(ctx, q, resolved, now, allowHistoricalEditing)
}

func LoadResolvedGrid(ctx context.Context, q *db.Queries, resolved ResolvedFilter, now time.Time, allowHistoricalEditing bool) (Grid, error) {
	return loadGrid(ctx, q, resolved, 0, now, allowHistoricalEditing)
}

// LoadWeekRow renders just the affected task's row plus the scope weeks and totals.
func LoadWeekRow(ctx context.Context, q *db.Queries, resolved ResolvedFilter, taskID int64, now time.Time, allowHistoricalEditing bool) (Grid, error) {
	return loadGrid(ctx, q, resolved, taskID, now, allowHistoricalEditing)
}

type gridContext struct {
	grid     Grid
	projects []project.Project
	subNames map[int64]string
}

func loadGrid(ctx context.Context, q *db.Queries, resolved ResolvedFilter, taskID int64, now time.Time, allowHistoricalEditing bool) (Grid, error) {
	gc, err := buildGridBase(ctx, q, resolved, allowHistoricalEditing)
	if err != nil {
		return Grid{}, err
	}
	switch gc.grid.Kind {
	case ViewAll:
		return loadAllRows(ctx, q, gc)
	case ViewIdeas:
		return loadIdeaRows(ctx, q, gc)
	}
	var tasks []Task
	if resolved.Filter.Subproject != nil {
		tasks, err = ListBySubproject(ctx, q, *resolved.Filter.Subproject)
	} else {
		tasks, err = ListByProject(ctx, q, resolved.Project.ID)
	}
	if err != nil {
		return Grid{}, err
	}
	byTask, err := loadTaskSeries(ctx, q, resolved, taskID)
	if err != nil {
		return Grid{}, err
	}
	if taskID != 0 {
		idx := -1
		for i := range tasks {
			if tasks[i].ID == taskID {
				idx = i
			}
		}
		if idx < 0 {
			return gc.grid, nil
		}
		tasks = tasks[idx : idx+1]
	}
	grid, err := loadScopeSummary(ctx, q, resolved, gc.grid)
	if err != nil {
		return Grid{}, err
	}
	grid.Rows, err = buildTaskRows(tasks, byTask, grid, gc.subNames, now, allowHistoricalEditing)
	if err != nil {
		return Grid{}, err
	}
	grid.Overrun = grid.PlannedHours > grid.BudgetHours
	return grid, nil
}

func loadTaskSeries(ctx context.Context, q *db.Queries, resolved ResolvedFilter, taskID int64) (map[int64][]weekly.Cell, error) {
	if taskID != 0 {
		cells, err := LoadWeeks(ctx, q, taskID)
		if err != nil {
			return nil, err
		}
		return map[int64][]weekly.Cell{taskID: cells}, nil
	}
	rows, err := seriesRows(ctx, q, resolved)
	if err != nil {
		return nil, err
	}
	cells, err := weekly.MapWeekSeries(rows)
	if err != nil {
		return nil, err
	}
	byTask := make(map[int64][]weekly.Cell)
	for _, cell := range cells {
		byTask[cell.TaskID] = append(byTask[cell.TaskID], cell)
	}
	return byTask, nil
}

func seriesRows(ctx context.Context, q *db.Queries, resolved ResolvedFilter) ([]db.VTaskWeekSeries, error) {
	if resolved.Filter.Subproject != nil {
		return q.ListTaskWeekSeriesBySubproject(ctx, *resolved.Filter.Subproject)
	}
	return q.ListTaskWeekSeriesByProject(ctx, resolved.Project.ID)
}

func buildGridBase(ctx context.Context, q *db.Queries, resolved ResolvedFilter, allowHistoricalEditing bool) (gridContext, error) {
	projects, err := project.List(ctx, q)
	if err != nil {
		return gridContext{}, err
	}
	filter := resolved.Filter
	key := filterProjectKey(filter)
	kind := ViewProject
	switch {
	case filter.Subproject != nil:
		kind = ViewSubproject
	case filter.All:
		kind = ViewAll
	case filter.Ideas:
		kind = ViewIdeas
	}
	gc := gridContext{
		projects: projects,
		grid:     Grid{FilterProject: key, Kind: kind, ProjectName: "All tasks", Projects: projectOptions(projects, key)},
	}
	if !IsWeekly(kind) {
		return gc, nil
	}
	proj := resolved.Project
	subs, err := subproject.ListByProject(ctx, q, proj.ID)
	if err != nil {
		return gridContext{}, err
	}
	gc.subNames = make(map[int64]string, len(subs))
	for _, s := range subs {
		gc.subNames[s.ID] = s.Name
	}
	gc.grid.Subprojects = subprojectOptions(subs, filter.Subproject)
	gc.grid.ProjectName = proj.Name
	gc.grid.StartDate = proj.StartDate
	gc.grid.EndDate = proj.EndDate
	gc.grid.POName = proj.PurchaseOrderName
	gc.grid.BudgetHours = proj.TotalHours
	gc.grid.HistoricalEditingAllowed = allowHistoricalEditing
	if resolved.Subproject != nil {
		gc.grid.FilterSubproject = strconv.FormatInt(*filter.Subproject, 10)
		gc.grid.SubprojectName = resolved.Subproject.Name
		gc.grid.BudgetHours = resolved.Subproject.TotalHours
	}
	return gc, nil
}

func loadAllRows(ctx context.Context, q *db.Queries, gc gridContext) (Grid, error) {
	tasks, err := List(ctx, q)
	if err != nil {
		return Grid{}, err
	}
	subs, err := subproject.List(ctx, q)
	if err != nil {
		return Grid{}, err
	}
	projectNames := make(map[int64]string, len(gc.projects))
	subprojectNames := make(map[int64]string, len(subs))
	for _, p := range gc.projects {
		projectNames[p.ID] = p.Name
	}
	for _, s := range subs {
		subprojectNames[s.ID] = s.Name
	}
	grid := gc.grid
	for _, t := range tasks {
		row := GridRow{ID: t.ID, Name: t.Name, Status: t.Status, TotalHours: t.TotalHours, SpentHours: t.SpentHours, Progress: t.Progress, DetailPath: "/tasks/" + strconv.FormatInt(t.ID, 10)}
		if t.ProjectID != nil {
			row.ProjectName = projectNames[*t.ProjectID]
		}
		if t.SubprojectID != nil {
			row.Subproject = subprojectNames[*t.SubprojectID]
		}
		grid.Rows = append(grid.Rows, row)
	}
	return grid, nil
}

func loadIdeaRows(ctx context.Context, q *db.Queries, gc gridContext) (Grid, error) {
	tasks, err := ListIdeas(ctx, q)
	if err != nil {
		return Grid{}, err
	}
	grid := gc.grid
	for _, t := range tasks {
		grid.Rows = append(grid.Rows, GridRow{ID: t.ID, Name: t.Name, DetailPath: "/tasks/" + strconv.FormatInt(t.ID, 10)})
	}
	return grid, nil
}

func loadScopeSummary(ctx context.Context, q *db.Queries, resolved ResolvedFilter, grid Grid) (Grid, error) {
	if resolved.Filter.Subproject != nil {
		summary, err := q.GetSubprojectTotals(ctx, *resolved.Filter.Subproject)
		if err != nil {
			return Grid{}, err
		}
		grid.PlannedHours, grid.SpentHours = summary.PlannedHours, summary.SpentHours
		rows, err := q.ListSubprojectWeekTotals(ctx, *resolved.Filter.Subproject)
		if err != nil {
			return Grid{}, err
		}
		for _, row := range rows {
			if grid, err = addWeekTotal(grid, row.WeekStart, row.PlannedHours, row.SpentHours); err != nil {
				return Grid{}, err
			}
		}
		return grid, nil
	}
	grid.ProgressPct = new(0.0)
	summary, err := q.GetProjectTotals(ctx, resolved.Project.ID)
	if err != nil {
		return Grid{}, err
	}
	grid.PlannedHours, grid.SpentHours = summary.PlannedHours, summary.SpentHours
	*grid.ProgressPct = summary.Progress
	rows, err := q.ListProjectWeekTotals(ctx, resolved.Project.ID)
	if err != nil {
		return Grid{}, err
	}
	for _, row := range rows {
		if grid, err = addWeekTotal(grid, row.WeekStart, row.PlannedHours, row.SpentHours); err != nil {
			return Grid{}, err
		}
	}
	return grid, nil
}

func addWeekTotal(grid Grid, weekStart string, planned, spent float64) (Grid, error) {
	ws, err := weekly.Parse(weekStart)
	if err != nil {
		return grid, err
	}
	info, err := ws.Info()
	if err != nil {
		return grid, err
	}
	grid.Weeks = append(grid.Weeks, info)
	grid.WeekTotals = append(grid.WeekTotals, GridWeekTotal{Planned: planned, Spent: spent})
	return grid, nil
}

func buildTaskRows(tasks []Task, byTask map[int64][]weekly.Cell, grid Grid, subNames map[int64]string, now time.Time, allowHistoricalEditing bool) ([]GridRow, error) {
	rows := make([]GridRow, 0, len(tasks))
	for _, t := range tasks {
		row := GridRow{ID: t.ID, Name: t.Name, ProjectName: grid.ProjectName, TotalHours: t.TotalHours, SpentHours: t.SpentHours, Progress: t.Progress, Status: t.Status, DetailPath: "/tasks/" + strconv.FormatInt(t.ID, 10)}
		if t.SubprojectID != nil {
			row.Subproject = subNames[*t.SubprojectID]
		}
		series := make(map[string]weekly.Cell, len(byTask[t.ID]))
		for _, cell := range byTask[t.ID] {
			series[cell.WeekStart.String()] = cell
		}
		for _, week := range grid.Weeks {
			s, ok := series[week.Start.String()]
			ws := week.Start
			cell := GridCell{
				WeekStart: ws,
				Locked:    !allowHistoricalEditing && ws.IsLocked(now),
				SavePath:  "/tasks/" + strconv.FormatInt(t.ID, 10) + "/weeks/" + ws.String(),
			}
			if ok {
				cell.Planned = s.PlannedHours
				cell.Spent = s.SpentHours
				cell.Progress = *s.Progress
				cell.Stored = s.StoredProgress != nil
			}
			row.Cells = append(row.Cells, cell)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func projectOptions(projects []project.Project, selected string) []Option {
	out := []Option{{Value: filterAll, Label: "All tasks", Selected: selected == filterAll}, {Value: filterIdeas, Label: "Ideas", Selected: selected == filterIdeas}}
	for _, p := range projects {
		v := strconv.FormatInt(p.ID, 10)
		out = append(out, Option{Value: v, Label: p.Name, Selected: v == selected})
	}
	return out
}

func filterProjectKey(filter Filter) string {
	if filter.Ideas {
		return filterIdeas
	}
	if filter.All {
		return filterAll
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

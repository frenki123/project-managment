package task

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/monthlock"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/weekly"
)

type Grid struct {
	Projects         []Option
	Subprojects      []Option
	FilterProject    string
	FilterSubproject string
	Ideas            bool
	ProjectName      string
	SubprojectName   string
	StartDate        string
	EndDate          string
	Weeks            []weekly.WeekInfo
	WeekTotals       []GridWeekTotal
	Rows             []GridRow
	POName           string
	BudgetHours      float64
	PlannedHours     float64
	SpentHours       float64
	ProgressPct      *float64
	Overrun          bool
	HistoryUnlocked  bool
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
	Planned           float64
	Spent             float64
	CumulativeEarned  float64
	CumulativePlanned float64
	CumulativeSpent   float64
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
	filter, err := parseProjectFilter(projectKey)
	if err != nil {
		return Grid{}, err
	}

	data := Grid{
		FilterProject:    filter.Value,
		FilterSubproject: "",
		Ideas:            filter.Ideas,
		ProjectName:      "Ideas",
		Projects:         projectOptions(projects, filter.Value),
	}
	unlocked, err := monthlock.Unlocked(ctx, q)
	if err != nil {
		return Grid{}, err
	}
	data.HistoryUnlocked = unlocked
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
				DetailPath:  "/tasks/" + strconv.FormatInt(t.ID, 10),
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
	data.ProjectName = proj.Name
	data.StartDate = proj.StartDate
	data.EndDate = proj.EndDate
	var sp subproject.Subproject
	if subprojectID != nil {
		sp, err = subproject.Get(ctx, q, *subprojectID)
		if err != nil {
			return Grid{}, err
		}
		if sp.ProjectID != pid {
			subprojectID = nil
			data.Subprojects = subprojectOptions(subs, nil)
		}
	}
	if subprojectID != nil {
		data.FilterSubproject = strconv.FormatInt(*subprojectID, 10)
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
	if subprojectID != nil {
		data.BudgetHours = sp.TotalHours
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
		rows, err := q.ListSubprojectWeekTotals(ctx, *subprojectID)
		if err != nil {
			return Grid{}, err
		}
		for _, row := range rows {
			addWeekTotal(GridWeekTotal{
				Planned: row.PlannedHours, Spent: row.SpentHours,
				CumulativePlanned: row.CumulativePlannedHours, CumulativeSpent: row.CumulativeSpentHours,
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
				Planned: row.PlannedHours, Spent: row.SpentHours, CumulativeEarned: row.EarnedHours,
				CumulativePlanned: row.CumulativePlannedHours, CumulativeSpent: row.CumulativeSpentHours,
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
				Locked:    monthlock.WeekLocked(string(ws), now, unlocked),
				SavePath:  "/tasks/" + strconv.FormatInt(t.ID, 10) + "/weeks/" + string(ws),
			})
		}
		data.Rows = append(data.Rows, row)
	}
	data.Overrun = data.PlannedHours > data.BudgetHours

	return data, nil
}

func projectOptions(projects []project.Project, selected string) []Option {
	out := []Option{{Value: "ideas", Label: "Ideas", Selected: selected == "ideas"}}
	for _, p := range projects {
		v := strconv.FormatInt(p.ID, 10)
		out = append(out, Option{Value: v, Label: p.Name, Selected: v == selected})
	}
	return out
}

func parseProjectFilter(value string) (projectFilter, error) {
	if value == "" || value == "ideas" {
		return projectFilter{Value: "ideas", Ideas: true}, nil
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return projectFilter{}, app.Invalid("invalid project")
	}
	return projectFilter{Value: value, ID: id}, nil
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

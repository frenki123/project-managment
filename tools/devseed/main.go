package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"time"

	"cad-development/internal/db"
	"cad-development/internal/nullable"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/weekly"

	_ "modernc.org/sqlite"
)

const dsnSuffix = "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"

type shape uint8

const (
	shapeEven shape = iota
	shapeFront
	shapeBack
	shapeBell
)

type style uint8

const (
	styleNone style = iota
	styleLinear
	styleStepped
	styleLate
	styleFlat
)

type taskPlan struct {
	subproject int
	planned    float64
	spent      float64
	final      float64
	span       int
	shape      shape
	style      style
}

type subPlan struct {
	totalHours float64
}

type projectPlan struct {
	totalHours  float64
	startOffset int
	endOffset   int
	subprojects []subPlan
	tasks       []taskPlan
}

type week struct {
	weekStart string
	planned   float64
	spent     float64
	progress  *float64
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	target := os.Getenv("DATABASE_URL")
	if target == "" {
		target = "./data/app.db"
	}
	conn, err := sql.Open("sqlite", target+dsnSuffix)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.Ping(); err != nil {
		return err
	}
	return seed(context.Background(), db.New(conn), time.Now())
}

func seed(ctx context.Context, q *db.Queries, anchor time.Time) error {
	anchor = weekly.MondayOnOrBefore(anchor)
	fmt.Printf("%2s %8s %8s %8s %9s %6s %6s\n", "#", "total", "task", "spent", "progress", "weeks", "rows")
	for index, plan := range dataset() {
		if err := seedProject(ctx, q, anchor, index+1, plan); err != nil {
			return err
		}
	}
	return seedIdeas(ctx, q)
}

func seedProject(ctx context.Context, q *db.Queries, anchor time.Time, number int, plan projectPlan) error {
	start := weekly.MondayOnOrBefore(anchor.AddDate(0, 0, 7*plan.startOffset))
	end := weekly.MondayOnOrBefore(anchor.AddDate(0, 0, 7*plan.endOffset))
	weeks := weekly.WeekStarts(start, end)

	created, err := project.Create(ctx, q, project.Input{
		Name:              fmt.Sprintf("Project %d", number),
		PurchaseOrderName: fmt.Sprintf("PO-%d", number),
		TotalHours:        new(plan.totalHours),
		StartDate:         start.Format(time.DateOnly),
		EndDate:           end.Format(time.DateOnly),
	})
	if err != nil {
		return err
	}

	ids := make([]int64, len(plan.subprojects)+1)
	for i, sub := range plan.subprojects {
		result, err := subproject.Create(ctx, q, subproject.Input{
			ProjectID:  created.ID,
			Name:       fmt.Sprintf("Subproject %d", i+1),
			TotalHours: new(sub.totalHours),
		})
		if err != nil {
			return err
		}
		ids[i+1] = result.ID
	}

	rows := 0
	for i, tp := range plan.tasks {
		var subprojectID *int64
		if tp.subproject > 0 {
			subprojectID = &ids[tp.subproject]
		}
		createdTask, err := task.Create(ctx, q, task.Input{
			Name:         fmt.Sprintf("Task %02d", i+1),
			ProjectID:    &created.ID,
			SubprojectID: subprojectID,
		})
		if err != nil {
			return err
		}
		cells, err := expand(tp, weeks)
		if err != nil {
			return fmt.Errorf("project %d task %d: %w", number, i+1, err)
		}
		for _, cell := range cells {
			if _, err := q.UpsertTaskWeek(ctx, db.UpsertTaskWeekParams{
				TaskID:       createdTask.ID,
				WeekStart:    cell.weekStart,
				PlannedHours: cell.planned,
				SpentHours:   cell.spent,
				Progress:     nullable.Float64(cell.progress),
			}); err != nil {
				return err
			}
			rows++
		}
	}

	filled, err := project.Get(ctx, q, created.ID)
	if err != nil {
		return err
	}
	fmt.Printf("%2d %8.1f %8.1f %8.1f %8.1f%% %6d  rows:%d\n",
		number, filled.TotalHours, filled.PlannedHours, filled.SpentHours, filled.ProgressPct,
		len(weeks), rows)
	return nil
}

func seedIdeas(ctx context.Context, q *db.Queries) error {
	for i := range 10 {
		if _, err := task.Create(ctx, q, task.Input{
			Name: fmt.Sprintf("Idea %d", i+1),
		}); err != nil {
			return err
		}
	}
	fmt.Println("ideas: 10 tasks, 0 weekly cells")
	return nil
}

func expand(tp taskPlan, weeks []weekly.WeekStart) ([]week, error) {
	if tp.span < 1 || tp.span > len(weeks) {
		return nil, fmt.Errorf("span %d outside %d project weeks", tp.span, len(weeks))
	}
	planned := distribute(weights(tp.shape, tp.span), tp.planned)
	spent := distribute(planned, tp.spent)
	cells := make([]week, 0, tp.span)
	for i := range tp.span {
		cells = append(cells, week{
			weekStart: string(weeks[i]),
			planned:   planned[i],
			spent:     spent[i],
			progress:  progressAt(tp.style, i, tp.span, tp.final),
		})
	}
	return cells, nil
}

func weights(s shape, span int) []float64 {
	out := make([]float64, span)
	for i := range span {
		switch s {
		case shapeEven:
			out[i] = 1
		case shapeFront:
			out[i] = float64(span - i)
		case shapeBack:
			out[i] = float64(i + 1)
		case shapeBell:
			out[i] = 1 + float64(min(i, span-1-i))
		}
	}
	return out
}

func distribute(w []float64, total float64) []float64 {
	sum := 0.0
	for _, v := range w {
		sum += v
	}
	out := make([]float64, len(w))
	for i, v := range w {
		out[i] = total * v / sum
	}
	return out
}

func progressAt(s style, i, span int, final float64) *float64 {
	var value float64
	switch s {
	case styleLinear:
		value = final * float64(i+1) / float64(span)
	case styleStepped:
		step := max(1, span/4)
		if i < span-1 && (i+1)%step != 0 {
			return nil
		}
		value = final * float64(min((i+1)/step, 4)) / 4
	case styleLate:
		start := span * 7 / 10
		if i < start {
			return nil
		}
		value = final * float64(i-start+1) / float64(span-start)
	case styleFlat:
		if i < span-1 {
			return nil
		}
		value = final
	default:
		return nil
	}
	return new(math.Round(value*10) / 10)
}

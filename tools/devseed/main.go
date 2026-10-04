package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"os"
	"time"

	"cad-development/internal/db"
	"cad-development/internal/nullable"
	"cad-development/internal/person"
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

type personSeed struct {
	name     string
	capacity float64
}

type overrideSeed struct {
	personIndex int
	offset      int
	capacity    float64
}

func main() {
	if err := run(); err != nil {
		slog.Error("seed", "err", err)
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
	people, err := seedPeople(ctx, q)
	if err != nil {
		return err
	}
	overrides, err := seedOverrides(ctx, q, anchor, people)
	if err != nil {
		return err
	}
	fmt.Printf("%2s %8s %8s %8s %9s %6s %6s\n", "#", "total", "task", "spent", "progress", "weeks", "rows")
	tasksWithDevelopers, allocations := 0, 0
	for index, plan := range dataset() {
		twd, alloc, err := seedProject(ctx, q, anchor, index+1, plan, people)
		if err != nil {
			return err
		}
		tasksWithDevelopers += twd
		allocations += alloc
	}
	ideaDevelopers, err := seedIdeas(ctx, q, people)
	if err != nil {
		return err
	}
	fmt.Printf("summary: %d people, %d overrides, %d tasks with developers, %d allocations\n",
		len(people), len(overrides), tasksWithDevelopers+ideaDevelopers, allocations)
	return nil
}

func seedPeople(ctx context.Context, q *db.Queries) ([]person.Person, error) {
	seeds := []personSeed{
		{"Ada Lovelace", 40},
		{"Grace Hopper", 32},
		{"Alan Turing", 24},
		{"Edsger Dijkstra", 40},
		{"Margaret Hamilton", 40},
		{"Linus Torvalds", 16},
	}
	people := make([]person.Person, 0, len(seeds))
	for _, seed := range seeds {
		created, err := person.Create(ctx, q, person.Input{Name: seed.name, WeeklyCapacity: new(seed.capacity)})
		if err != nil {
			return nil, err
		}
		people = append(people, created)
	}
	fmt.Printf("people: seeded %d\n", len(people))
	return people, nil
}

func seedOverrides(ctx context.Context, q *db.Queries, anchor time.Time, people []person.Person) ([]person.Override, error) {
	weekAt := func(offset int) string {
		return weekly.MondayOnOrBefore(anchor.AddDate(0, 0, 7*offset)).Format(time.DateOnly)
	}
	seeds := []overrideSeed{
		{0, -20, 0}, // Ada: holiday week in project 1 history
		{1, -2, 20}, // Grace: part-time week recently
		{2, 2, 30},  // Alan: reduced week
	}
	overrides := make([]person.Override, 0, len(seeds))
	for _, seed := range seeds {
		weekStart := weekAt(seed.offset)
		override, err := person.SetOverride(ctx, q, people[seed.personIndex].ID, weekStart, seed.capacity)
		if err != nil {
			return nil, err
		}
		overrides = append(overrides, override)
		fmt.Printf("override: %s capacity %.0f week %s\n", people[seed.personIndex].Name, seed.capacity, weekStart)
	}
	return overrides, nil
}

func seedProject(ctx context.Context, q *db.Queries, anchor time.Time, number int, plan projectPlan, people []person.Person) (int, int, error) {
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
		return 0, 0, err
	}

	ids := make([]int64, len(plan.subprojects)+1)
	for i, sub := range plan.subprojects {
		result, err := subproject.Create(ctx, q, subproject.Input{
			ProjectID:  created.ID,
			Name:       fmt.Sprintf("Subproject %d", i+1),
			TotalHours: new(sub.totalHours),
		})
		if err != nil {
			return 0, 0, err
		}
		ids[i+1] = result.ID
	}

	rows, tasksWithDevelopers, allocations := 0, 0, 0
	for i, tp := range plan.tasks {
		taskRows, twd, alloc, err := seedTask(ctx, q, created.ID, ids, i, tp, weeks, people)
		if err != nil {
			return 0, 0, fmt.Errorf("project %d task %d: %w", number, i+1, err)
		}
		rows += taskRows
		tasksWithDevelopers += twd
		allocations += alloc
	}

	filled, err := project.Get(ctx, q, created.ID)
	if err != nil {
		return 0, 0, err
	}
	fmt.Printf("%2d %8.1f %8.1f %8.1f %8.1f%% %6d  rows:%d\n",
		number, filled.TotalHours, filled.PlannedHours, filled.SpentHours, filled.ProgressPct,
		len(weeks), rows)
	return tasksWithDevelopers, allocations, nil
}

func seedTask(ctx context.Context, q *db.Queries, projectID int64, ids []int64, i int, tp taskPlan, weeks []weekly.WeekStart, people []person.Person) (int, int, int, error) {
	var subprojectID *int64
	if tp.subproject > 0 {
		subprojectID = &ids[tp.subproject]
	}
	developers := taskDevelopers(i, people)
	createdTask, err := task.Create(ctx, q, task.Input{
		Name:         fmt.Sprintf("Task %02d", i+1),
		ProjectID:    &projectID,
		SubprojectID: subprojectID,
		DeveloperIDs: developers,
	})
	if err != nil {
		return 0, 0, 0, err
	}
	tasksWithDevelopers := 0
	if len(developers) > 0 {
		tasksWithDevelopers++
	}
	cells, err := expand(tp, weeks)
	if err != nil {
		return 0, 0, 0, err
	}
	rows, allocations := 0, 0
	for cellIndex, cell := range cells {
		if _, err := q.UpsertTaskWeek(ctx, db.UpsertTaskWeekParams{
			TaskID:       createdTask.ID,
			WeekStart:    cell.weekStart,
			PlannedHours: cell.planned,
			SpentHours:   cell.spent,
			Progress:     nullable.Float64(cell.progress),
		}); err != nil {
			return 0, 0, 0, err
		}
		rows++
		for _, allocation := range splitCellAllocation(createdTask.ID, cell.weekStart, developers, cell.planned, cell.spent, cellIndex) {
			if err := q.UpsertTaskWeekDeveloper(ctx, allocation); err != nil {
				return 0, 0, 0, err
			}
			allocations++
		}
	}
	return rows, tasksWithDevelopers, allocations, nil
}

func seedIdeas(ctx context.Context, q *db.Queries, people []person.Person) (int, error) {
	tasksWithDevelopers := 0
	for i := range 10 {
		in := task.Input{Name: fmt.Sprintf("Idea %d", i+1)}
		if i%2 == 0 {
			in.DeveloperIDs = taskDevelopers(i, people)
			tasksWithDevelopers++
		}
		if _, err := task.Create(ctx, q, in); err != nil {
			return 0, err
		}
	}
	fmt.Println("ideas: 10 tasks, 0 weekly cells")
	return tasksWithDevelopers, nil
}

// taskDevelopers assigns one or two developers deterministically by task index.
func taskDevelopers(i int, people []person.Person) []int64 {
	primary := people[i%len(people)].ID
	if i%3 == 2 {
		return []int64{primary}
	}
	return []int64{primary, people[(i+2)%len(people)].ID}
}

// splitCellAllocation distributes a cell's hours across the task's developers.
// Single-developer and every third cell give the first developer the full hours;
// other two-developer cells split planned 60/40 and spent 40/60.
func splitCellAllocation(taskID int64, weekStart string, developers []int64, planned, spent float64, cellIndex int) []db.UpsertTaskWeekDeveloperParams {
	if planned == 0 && spent == 0 {
		return nil
	}
	if len(developers) == 0 {
		return nil
	}
	if len(developers) == 1 || cellIndex%3 == 0 {
		return []db.UpsertTaskWeekDeveloperParams{{
			TaskID: taskID, WeekStart: weekStart, PersonID: developers[0],
			PlannedHours: planned, SpentHours: spent,
		}}
	}
	firstPlanned := round1(planned * 0.6)
	secondPlanned := round1(planned - firstPlanned)
	firstSpent := round1(spent * 0.4)
	secondSpent := round1(spent - firstSpent)
	return []db.UpsertTaskWeekDeveloperParams{
		{TaskID: taskID, WeekStart: weekStart, PersonID: developers[0], PlannedHours: firstPlanned, SpentHours: firstSpent},
		{TaskID: taskID, WeekStart: weekStart, PersonID: developers[1], PlannedHours: secondPlanned, SpentHours: secondSpent},
	}
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
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
			weekStart: weeks[i].String(),
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
		if (i+1)%step != 0 || i+1 > 4*step {
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
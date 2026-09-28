package main

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"cad-development/internal/client"
	"github.com/spf13/cobra"
)

type commandState struct {
	apiURL      string
	table       bool
	out, errOut io.Writer
	client      *client.Client
}

// version is set at build time with -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	state := &commandState{apiURL: os.Getenv("CAD_API_URL"), out: os.Stdout, errOut: os.Stderr}
	if state.apiURL == "" {
		state.apiURL = client.DefaultURL
	}
	root := newRoot(state)
	if err := root.Execute(); err != nil {
		writeError(state.errOut, err)
		os.Exit(1)
	}
}

func writeError(w io.Writer, err error) {
	payload := map[string]any{"error": err.Error()}
	if apiErr, ok := errors.AsType[*client.APIError](err); ok {
		payload["status"] = apiErr.Status
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		fmt.Fprintln(w, err)
		return
	}
	fmt.Fprintln(w, string(data))
}

func newRoot(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "pmctl", Short: "Control the project management API", Version: version, SilenceUsage: true, SilenceErrors: true, PersistentPreRunE: func(*cobra.Command, []string) error {
		c, err := client.New(s.apiURL)
		if err != nil {
			return err
		}
		s.client = c
		return nil
	}}
	root.PersistentFlags().StringVar(&s.apiURL, "url", s.apiURL, "REST API base URL")
	root.PersistentFlags().BoolVar(&s.table, "table", false, "print compact human-readable tables")
	root.AddCommand(projectCommands(s), subprojectCommands(s), taskCommands(s), updateWeekCommand(s), curveCommand(s))
	return root
}

func (s *commandState) printer() printer {
	return printer{json: s.out, table: s.out, useTable: s.table}
}
func idArg(args []string) (int64, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("expected one numeric id")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q", args[0])
	}
	return id, nil
}
func twoArgs(args []string) (int64, string, error) {
	if len(args) != 2 {
		return 0, "", fmt.Errorf("expected task id and week start")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid task id %q", args[0])
	}
	return id, args[1], nil
}

func resolveProject(ctx context.Context, c *client.Client, name string) (int64, error) {
	projects, err := c.Projects(ctx)
	if err != nil {
		return 0, err
	}
	want := strings.TrimSpace(name)
	var match *client.Project
	for i := range projects.Projects {
		p := &projects.Projects[i]
		if strings.EqualFold(p.Name, want) {
			if match != nil {
				return 0, fmt.Errorf("project %q is ambiguous", name)
			}
			match = p
		}
	}
	if match != nil {
		return match.ID, nil
	}
	return 0, fmt.Errorf("project %q not found", name)
}

func resolveSubproject(ctx context.Context, c *client.Client, projectID *int64, name string) (int64, error) {
	subprojects, err := c.Subprojects(ctx, projectID)
	if err != nil {
		return 0, err
	}
	want := strings.TrimSpace(name)
	var match *client.Subproject
	for i := range subprojects.Subprojects {
		s := &subprojects.Subprojects[i]
		if strings.EqualFold(s.Name, want) {
			if match != nil {
				if projectID == nil {
					return 0, fmt.Errorf("subproject %q is ambiguous", name)
				}
				return 0, fmt.Errorf("subproject %q is ambiguous in project %d", name, *projectID)
			}
			match = s
		}
	}
	if match != nil {
		return match.ID, nil
	}
	if projectID == nil {
		return 0, fmt.Errorf("subproject %q not found", name)
	}
	return 0, fmt.Errorf("subproject %q not found in project %q", name, strconv.FormatInt(*projectID, 10))
}
func deleteCommand(use, kind string, s *commandState, del func(context.Context, int64) error) *cobra.Command {
	return &cobra.Command{Use: use + " <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := idArg(args)
		if err != nil {
			return err
		}
		if err := del(cmd.Context(), id); err != nil {
			return err
		}
		return s.printer().deleted(kind, id)
	}}
}

func projectCommands(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "projects"}
	root.AddCommand(&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		v, err := s.client.Projects(cmd.Context())
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return projectTable(w, v.Projects) })
	}})
	root.AddCommand(&cobra.Command{Use: "get <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := idArg(args)
		if err != nil {
			return err
		}
		v, err := s.client.Project(cmd.Context(), id)
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return projectTable(w, []client.Project{v}) })
	}})
	root.AddCommand(projectCommand("create", s, false), projectCommand("update <id>", s, true), deleteCommand("delete", "project", s, func(ctx context.Context, id int64) error { return s.client.DeleteProject(ctx, id) }))
	return root
}

type projectFlags struct {
	name, purchaseOrder, startDate, endDate string
	totalHours                              float64
}

func (f *projectFlags) addFlags(c *cobra.Command) {
	c.Flags().StringVar(&f.name, "name", "", "project name")
	c.Flags().StringVar(&f.purchaseOrder, "purchase-order-name", "", "purchase order name")
	c.Flags().Float64Var(&f.totalHours, "total-hours", 0, "project total hours")
	c.Flags().StringVar(&f.startDate, "start-date", "", "project start date")
	c.Flags().StringVar(&f.endDate, "end-date", "", "project end date")
}

func projectCommand(use string, s *commandState, update bool) *cobra.Command {
	var f projectFlags
	c := &cobra.Command{Use: use, Args: func(cmd *cobra.Command, args []string) error {
		if update {
			return cobra.ExactArgs(1)(cmd, args)
		}
		return cobra.NoArgs(cmd, args)
	}, RunE: func(cmd *cobra.Command, args []string) error {
		var totalHours *float64
		if cmd.Flags().Changed("total-hours") {
			totalHours = &f.totalHours
		}
		in := client.ProjectInput{Name: f.name, PurchaseOrderName: f.purchaseOrder, TotalHours: totalHours, StartDate: f.startDate, EndDate: f.endDate}
		var err error
		var v client.Project
		if update {
			id, e := idArg(args)
			if e != nil {
				return e
			}
			v, err = s.client.UpdateProject(cmd.Context(), id, in)
		} else {
			v, err = s.client.CreateProject(cmd.Context(), in)
		}
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return projectTable(w, []client.Project{v}) })
	}}
	f.addFlags(c)
	return c
}

func subprojectCommands(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "subprojects"}
	var projectName string
	var projectID int64
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		var selectedProject *int64
		if projectName != "" {
			resolved, err := resolveProject(cmd.Context(), s.client, projectName)
			if err != nil {
				return err
			}
			selectedProject = &resolved
		} else if cmd.Flags().Changed("project-id") {
			selectedProject = &projectID
		}
		v, err := s.client.Subprojects(cmd.Context(), selectedProject)
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return subprojectTable(w, v.Subprojects) })
	}}
	list.Flags().StringVar(&projectName, "project", "", "filter by project name")
	list.Flags().Int64Var(&projectID, "project-id", 0, "filter by project ID")
	root.AddCommand(list, &cobra.Command{Use: "get <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := idArg(args)
		if err != nil {
			return err
		}
		v, err := s.client.Subproject(cmd.Context(), id)
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return subprojectTable(w, []client.Subproject{v}) })
	}}, subprojectCommand("create", s, false), subprojectCommand("update <id>", s, true), deleteCommand("delete", "subproject", s, func(ctx context.Context, id int64) error { return s.client.DeleteSubproject(ctx, id) }))
	return root
}

type subprojectFlags struct {
	projectID  int64
	name       string
	totalHours float64
}

func (f *subprojectFlags) addFlags(c *cobra.Command) {
	c.Flags().Int64Var(&f.projectID, "project-id", 0, "project ID")
	c.Flags().StringVar(&f.name, "name", "", "subproject name")
	c.Flags().Float64Var(&f.totalHours, "total-hours", 0, "subproject total hours")
}

func subprojectCommand(use string, s *commandState, update bool) *cobra.Command {
	var f subprojectFlags
	c := &cobra.Command{Use: use, Args: func(cmd *cobra.Command, args []string) error {
		if update {
			return cobra.ExactArgs(1)(cmd, args)
		}
		return cobra.NoArgs(cmd, args)
	}, RunE: func(cmd *cobra.Command, args []string) error {
		totalHours := (*float64)(nil)
		if cmd.Flags().Changed("total-hours") {
			totalHours = &f.totalHours
		}
		in := client.SubprojectInput{ProjectID: f.projectID, Name: f.name, TotalHours: totalHours}
		var err error
		var v client.Subproject
		if update {
			id, e := idArg(args)
			if e != nil {
				return e
			}
			v, err = s.client.UpdateSubproject(cmd.Context(), id, in)
		} else {
			v, err = s.client.CreateSubproject(cmd.Context(), in)
		}
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return subprojectTable(w, []client.Subproject{v}) })
	}}
	f.addFlags(c)
	return c
}

func taskCommands(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "tasks"}
	var ideas bool
	var projectName, subprojectName string
	var projectID, subprojectID int64
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		var selectedProject, selectedSubproject *int64
		if projectName != "" {
			resolved, err := resolveProject(cmd.Context(), s.client, projectName)
			if err != nil {
				return err
			}
			selectedProject = &resolved
		} else if cmd.Flags().Changed("project-id") {
			selectedProject = &projectID
		}
		if subprojectName != "" {
			resolved, err := resolveSubproject(cmd.Context(), s.client, selectedProject, subprojectName)
			if err != nil {
				return err
			}
			selectedSubproject = &resolved
		} else if cmd.Flags().Changed("subproject-id") {
			selectedSubproject = &subprojectID
		}
		useIdeas := ideas && selectedProject == nil && selectedSubproject == nil
		v, err := s.client.Tasks(cmd.Context(), useIdeas, selectedProject, selectedSubproject)
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return taskTable(w, v.Tasks) })
	}}
	list.Flags().BoolVar(&ideas, "ideas", false, "list unassigned idea tasks")
	list.Flags().StringVar(&projectName, "project", "", "filter by unique project name")
	list.Flags().Int64Var(&projectID, "project-id", 0, "filter by project ID")
	list.Flags().StringVar(&subprojectName, "subproject", "", "filter by subproject name within the project")
	list.Flags().Int64Var(&subprojectID, "subproject-id", 0, "filter by subproject ID")
	root.AddCommand(list, &cobra.Command{Use: "get <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := idArg(args)
		if err != nil {
			return err
		}
		v, err := s.client.Task(cmd.Context(), id)
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return taskTable(w, []client.Task{v}) })
	}}, taskCommand("create", s, false), taskCommand("update <id>", s, true), deleteCommand("delete", "task", s, func(ctx context.Context, id int64) error { return s.client.DeleteTask(ctx, id) }))
	return root
}

type taskFlags struct {
	name, description, notes, department, developers, priority string
	projectID, subprojectID                                    int64
	ideas                                                      bool
}

func (f *taskFlags) addFlags(c *cobra.Command) {
	c.Flags().StringVar(&f.name, "name", "", "task name")
	c.Flags().StringVar(&f.description, "description", "", "task description")
	c.Flags().StringVar(&f.notes, "implementation-notes", "", "task implementation notes")
	c.Flags().StringVar(&f.department, "department", "", "relevant department")
	c.Flags().StringVar(&f.developers, "developers", "", "developer or developers")
	c.Flags().StringVar(&f.priority, "priority", "", "task priority")
	c.Flags().Int64Var(&f.projectID, "project-id", 0, "project ID")
	c.Flags().Int64Var(&f.subprojectID, "subproject-id", 0, "subproject ID")
	c.Flags().BoolVar(&f.ideas, "ideas", false, "leave the task unassigned")
}

func taskInput(cmd *cobra.Command, f taskFlags, current *client.Task) client.TaskInput {
	in := client.TaskInput{}
	if current != nil {
		in = client.TaskInput{
			Name: current.Name, Description: current.Description, ImplementationNotes: current.ImplementationNotes,
			Department: current.Department, Developers: current.Developers, Priority: current.Priority,
			ProjectID: current.ProjectID, SubprojectID: current.SubprojectID,
		}
	}
	set := func(name string, dst *string, value string) {
		if cmd.Flags().Changed(name) {
			*dst = value
		}
	}
	set("name", &in.Name, f.name)
	set("description", &in.Description, f.description)
	set("implementation-notes", &in.ImplementationNotes, f.notes)
	set("department", &in.Department, f.department)
	set("developers", &in.Developers, f.developers)
	set("priority", &in.Priority, f.priority)
	if cmd.Flags().Changed("project-id") {
		in.ProjectID = &f.projectID
		if current != nil && current.ProjectID != nil && *current.ProjectID != f.projectID && !cmd.Flags().Changed("subproject-id") {
			in.SubprojectID = nil
		}
	}
	if cmd.Flags().Changed("subproject-id") {
		in.SubprojectID = &f.subprojectID
	}
	if f.ideas && !cmd.Flags().Changed("project-id") && !cmd.Flags().Changed("subproject-id") {
		in.ProjectID, in.SubprojectID = nil, nil
	}
	return in
}

func taskCommand(use string, s *commandState, update bool) *cobra.Command {
	var f taskFlags
	c := &cobra.Command{Use: use, Args: func(cmd *cobra.Command, args []string) error {
		if update {
			return cobra.ExactArgs(1)(cmd, args)
		}
		return cobra.NoArgs(cmd, args)
	}, RunE: func(cmd *cobra.Command, args []string) error {
		var id int64
		var current *client.Task
		if update {
			var err error
			id, err = idArg(args)
			if err != nil {
				return err
			}
			value, err := s.client.Task(cmd.Context(), id)
			if err != nil {
				return err
			}
			current = &value
		}
		in := taskInput(cmd, f, current)
		var v client.Task
		var err error
		if update {
			v, err = s.client.UpdateTask(cmd.Context(), id, in)
		} else {
			v, err = s.client.CreateTask(cmd.Context(), in)
		}
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return taskTable(w, []client.Task{v}) })
	}}
	f.addFlags(c)
	return c
}

func updateWeekCommand(s *commandState) *cobra.Command {
	var planned, spent, progress float64
	var clearProgress, unlock bool
	c := &cobra.Command{Use: "update-task-week <task-id> <week-start>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		id, week, err := twoArgs(args)
		if err != nil {
			return err
		}
		in := client.WeekPatch{ClearProgress: clearProgress}
		if cmd.Flags().Changed("planned-hours") {
			in.PlannedHours = &planned
		}
		if cmd.Flags().Changed("spent-hours") {
			in.SpentHours = &spent
		}
		if cmd.Flags().Changed("progress") {
			in.Progress = &progress
		}
		if unlock {
			if _, err := s.client.SetMonthLock(cmd.Context(), true); err != nil {
				return fmt.Errorf("unlock history: %w", err)
			}
		}
		v, updateErr := s.client.UpdateTaskWeek(cmd.Context(), id, week, in)
		var lockErr error
		if unlock {
			lockErr = lockAfter(s.client)
		}
		if updateErr != nil {
			if lockErr != nil {
				return fmt.Errorf("update task week: %w; lock history: %v", updateErr, lockErr)
			}
			return updateErr
		}
		if lockErr != nil {
			return fmt.Errorf("lock history: %w", lockErr)
		}
		return s.printer().print(v, func(w io.Writer) error { return weekTable(w, v) })
	}}
	c.Flags().Float64Var(&planned, "planned-hours", 0, "planned hours")
	c.Flags().Float64Var(&spent, "spent-hours", 0, "spent hours")
	c.Flags().Float64Var(&progress, "progress", 0, "progress percentage")
	c.Flags().BoolVar(&clearProgress, "clear-progress", false, "clear explicitly entered progress")
	c.Flags().BoolVar(&unlock, "unlock", false, "temporarily unlock history for this update")
	return c
}
func lockAfter(c *client.Client) error {
	cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if _, err = c.SetMonthLock(cleanup, false); err == nil {
			return nil
		}
		if apiErr, ok := errors.AsType[*client.APIError](err); ok && apiErr.Status < 500 {
			break
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}
	return err
}

func curveCommand(s *commandState) *cobra.Command {
	var projectName string
	var projectID int64
	c := &cobra.Command{Use: "project-s-curve", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if projectName == "" && !cmd.Flags().Changed("project-id") {
			return errors.New("--project or --project-id is required")
		}
		id := projectID
		var err error
		if projectName != "" {
			id, err = resolveProject(cmd.Context(), s.client, projectName)
		}
		if err != nil {
			return err
		}
		v, err := s.client.SCurve(cmd.Context(), id)
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return curveTable(w, v) })
	}}
	c.Flags().StringVar(&projectName, "project", "", "project name")
	c.Flags().Int64Var(&projectID, "project-id", 0, "project ID")
	return c
}

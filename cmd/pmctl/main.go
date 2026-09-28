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
	inputFile   string
	out, errOut io.Writer
	client      *client.Client
}

// version is set at build time with -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	state := &commandState{apiURL: os.Getenv("CAD_API_URL"), inputFile: "-", out: os.Stdout, errOut: os.Stderr}
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
func idArg(cmd *cobra.Command, args []string) (int64, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("expected one numeric id")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id %q", args[0])
	}
	return id, nil
}
func twoArgs(args []string) (int64, string, error) {
	if len(args) != 2 {
		return 0, "", fmt.Errorf("expected task id and week start")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id < 1 {
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

func resolveSubproject(ctx context.Context, c *client.Client, projectID int64, name string) (int64, error) {
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
				return 0, fmt.Errorf("subproject %q is ambiguous in project %d", name, projectID)
			}
			match = s
		}
	}
	if match != nil {
		return match.ID, nil
	}
	return 0, fmt.Errorf("subproject %q not found in project %q", name, strconv.FormatInt(projectID, 10))
}
func readInput[T any](file string) (T, error) {
	var value T
	var r io.Reader = os.Stdin
	if file != "-" {
		f, err := os.Open(file)
		if err != nil {
			return value, err
		}
		defer f.Close()
		r = f
	}
	if err := json.UnmarshalRead(r, &value, json.RejectUnknownMembers(true), json.MatchCaseInsensitiveNames(true)); err != nil {
		return value, fmt.Errorf("invalid JSON input: %w", err)
	}
	return value, nil
}
func deleteCommand(use, kind string, s *commandState, del func(context.Context, int64) error) *cobra.Command {
	return &cobra.Command{Use: use + " <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := idArg(cmd, args)
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
		id, err := idArg(cmd, args)
		if err != nil {
			return err
		}
		v, err := s.client.Project(cmd.Context(), id)
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return projectTable(w, []client.Project{v}) })
	}})
	root.AddCommand(jsonProjectCommand("create", s, false), jsonProjectCommand("update <id>", s, true), deleteCommand("delete", "project", s, func(ctx context.Context, id int64) error { return s.client.DeleteProject(ctx, id) }))
	return root
}
func jsonProjectCommand(use string, s *commandState, update bool) *cobra.Command {
	c := &cobra.Command{Use: use, Args: func(cmd *cobra.Command, args []string) error {
		if update {
			return cobra.ExactArgs(1)(cmd, args)
		}
		return cobra.NoArgs(cmd, args)
	}, RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput[client.ProjectInput](s.inputFile)
		if err != nil {
			return err
		}
		var v client.Project
		if update {
			id, e := idArg(cmd, args)
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
	c.Flags().StringVar(&s.inputFile, "file", "-", "JSON input file, or - for stdin")
	return c
}

func subprojectCommands(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "subprojects"}
	var projectName string
	var projectID int64
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		var err error
		if projectName != "" && cmd.Flags().Changed("project-id") {
			return errors.New("--project and --project-id cannot be combined")
		}
		if cmd.Flags().Changed("project-id") && projectID < 1 {
			return errors.New("--project-id must be positive")
		}
		if projectName != "" {
			projectID, err = resolveProject(cmd.Context(), s.client, projectName)
		}
		if err != nil {
			return err
		}
		v, err := s.client.Subprojects(cmd.Context(), projectID)
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return subprojectTable(w, v.Subprojects) })
	}}
	list.Flags().StringVar(&projectName, "project", "", "filter by project name")
	list.Flags().Int64Var(&projectID, "project-id", 0, "filter by project ID")
	root.AddCommand(list, &cobra.Command{Use: "get <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := idArg(cmd, args)
		if err != nil {
			return err
		}
		v, err := s.client.Subproject(cmd.Context(), id)
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return subprojectTable(w, []client.Subproject{v}) })
	}}, jsonSubprojectCommand("create", s, false), jsonSubprojectCommand("update <id>", s, true), deleteCommand("delete", "subproject", s, func(ctx context.Context, id int64) error { return s.client.DeleteSubproject(ctx, id) }))
	return root
}
func jsonSubprojectCommand(use string, s *commandState, update bool) *cobra.Command {
	c := &cobra.Command{Use: use, Args: func(cmd *cobra.Command, args []string) error {
		if update {
			return cobra.ExactArgs(1)(cmd, args)
		}
		return cobra.NoArgs(cmd, args)
	}, RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput[client.SubprojectInput](s.inputFile)
		if err != nil {
			return err
		}
		var v client.Subproject
		if update {
			id, e := idArg(cmd, args)
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
	c.Flags().StringVar(&s.inputFile, "file", "-", "JSON input file, or - for stdin")
	return c
}

func taskCommands(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "tasks"}
	var ideas bool
	var projectName, subprojectName string
	var projectID, subprojectID int64
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if ideas && (projectName != "" || subprojectName != "" || cmd.Flags().Changed("project-id") || cmd.Flags().Changed("subproject-id")) {
			return errors.New("--ideas cannot be combined with project or subproject filters")
		}
		if projectName != "" && cmd.Flags().Changed("project-id") {
			return errors.New("--project and --project-id cannot be combined")
		}
		if subprojectName != "" && cmd.Flags().Changed("subproject-id") {
			return errors.New("--subproject and --subproject-id cannot be combined")
		}
		if (subprojectName != "" || cmd.Flags().Changed("subproject-id")) && projectName == "" && !cmd.Flags().Changed("project-id") {
			return errors.New("--subproject requires --project or --project-id")
		}
		if cmd.Flags().Changed("project-id") && projectID < 1 {
			return errors.New("--project-id must be positive")
		}
		if cmd.Flags().Changed("subproject-id") && subprojectID < 1 {
			return errors.New("--subproject-id must be positive")
		}
		var err error
		if projectName != "" {
			projectID, err = resolveProject(cmd.Context(), s.client, projectName)
			if err != nil {
				return err
			}
		}
		if subprojectName != "" {
			subprojectID, err = resolveSubproject(cmd.Context(), s.client, projectID, subprojectName)
			if err != nil {
				return err
			}
		}
		v, err := s.client.Tasks(cmd.Context(), ideas, projectID, subprojectID)
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
		id, err := idArg(cmd, args)
		if err != nil {
			return err
		}
		v, err := s.client.Task(cmd.Context(), id)
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return taskDetailTable(w, v) })
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

func taskInput(cmd *cobra.Command, f taskFlags, current *client.Task) (client.TaskInput, error) {
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
	if f.ideas {
		if cmd.Flags().Changed("project-id") || cmd.Flags().Changed("subproject-id") {
			return client.TaskInput{}, errors.New("--ideas cannot be combined with project assignment flags")
		}
		in.ProjectID, in.SubprojectID = nil, nil
	} else {
		if cmd.Flags().Changed("project-id") {
			if f.projectID < 1 {
				return client.TaskInput{}, errors.New("--project-id must be positive")
			}
			in.ProjectID = &f.projectID
			if current != nil && current.ProjectID != nil && *current.ProjectID != f.projectID && !cmd.Flags().Changed("subproject-id") {
				in.SubprojectID = nil
			}
		}
		if cmd.Flags().Changed("subproject-id") {
			if f.subprojectID < 1 {
				return client.TaskInput{}, errors.New("--subproject-id must be positive")
			}
			if in.ProjectID == nil {
				return client.TaskInput{}, errors.New("--subproject-id requires a project assignment")
			}
			in.SubprojectID = &f.subprojectID
		}
	}
	if in.Name == "" {
		return client.TaskInput{}, errors.New("--name is required")
	}
	return in, nil
}

func taskCommand(use string, s *commandState, update bool) *cobra.Command {
	var f taskFlags
	c := &cobra.Command{Use: use, Args: func(cmd *cobra.Command, args []string) error {
		if update {
			return cobra.ExactArgs(1)(cmd, args)
		}
		return cobra.NoArgs(cmd, args)
	}, RunE: func(cmd *cobra.Command, args []string) error {
		var current *client.Task
		if update {
			id, err := idArg(cmd, args)
			if err != nil {
				return err
			}
			value, err := s.client.Task(cmd.Context(), id)
			if err != nil {
				return err
			}
			current = &value
		}
		in, err := taskInput(cmd, f, current)
		if err != nil {
			return err
		}
		var v client.Task
		if update {
			id, e := idArg(cmd, args)
			if e != nil {
				return e
			}
			v, err = s.client.UpdateTask(cmd.Context(), id, in)
		} else {
			v, err = s.client.CreateTask(cmd.Context(), in)
		}
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return taskDetailTable(w, v) })
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
		if !cmd.Flags().Changed("planned-hours") && !cmd.Flags().Changed("spent-hours") && !cmd.Flags().Changed("progress") && !clearProgress {
			return errors.New("at least one weekly value is required")
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
		if projectName != "" && cmd.Flags().Changed("project-id") {
			return errors.New("--project and --project-id cannot be combined")
		}
		if projectName == "" && !cmd.Flags().Changed("project-id") {
			return errors.New("--project is required")
		}
		if cmd.Flags().Changed("project-id") && projectID < 1 {
			return errors.New("--project-id must be positive")
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

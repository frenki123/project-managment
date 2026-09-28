package main

import (
	"context"
	"io"

	"cad-development/internal/client"
	"github.com/spf13/cobra"
)

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

package main

import (
	"context"
	"io"

	"cad-development/internal/nullable"
	"cad-development/internal/task"
	"github.com/spf13/cobra"
)

func taskCommands(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "tasks"}
	var ideas bool
	var projectName, subprojectName string
	var projectID, subprojectID int64
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		selectedProject, err := resolveProjectFilter(cmd.Context(), s.client, projectName, projectID, cmd.Flags().Changed("project-id"))
		if err != nil {
			return err
		}
		selectedSubproject, err := resolveSubprojectFilter(cmd.Context(), s.client, selectedProject, subprojectName, subprojectID, cmd.Flags().Changed("subproject-id"))
		if err != nil {
			return err
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
		return s.printer().print(v, func(w io.Writer) error { return taskTable(w, []task.Task{v}) })
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

func taskInput(cmd *cobra.Command, f taskFlags) task.Input {
	in := task.Input{
		Name: f.name, Description: f.description, ImplementationNotes: f.notes,
		Department: f.department, Developers: f.developers, Priority: f.priority,
	}
	if cmd.Flags().Changed("project-id") {
		in.ProjectID = &f.projectID
	}
	if cmd.Flags().Changed("subproject-id") {
		in.SubprojectID = &f.subprojectID
	}
	if f.ideas && !cmd.Flags().Changed("project-id") && !cmd.Flags().Changed("subproject-id") {
		in.ProjectID, in.SubprojectID = nil, nil
	}
	return in
}

func taskPatch(cmd *cobra.Command, f taskFlags) task.Patch {
	in := task.Patch{}
	if cmd.Flags().Changed("name") {
		in.Name = textPatch(f.name)
	}
	if cmd.Flags().Changed("description") {
		in.Description = textPatch(f.description)
	}
	if cmd.Flags().Changed("implementation-notes") {
		in.ImplementationNotes = textPatch(f.notes)
	}
	if cmd.Flags().Changed("department") {
		in.Department = textPatch(f.department)
	}
	if cmd.Flags().Changed("developers") {
		in.Developers = textPatch(f.developers)
	}
	if cmd.Flags().Changed("priority") {
		in.Priority = textPatch(f.priority)
	}
	if cmd.Flags().Changed("project-id") {
		id := f.projectID
		in.ProjectID = nullable.Present(&id)
	}
	if cmd.Flags().Changed("subproject-id") {
		id := f.subprojectID
		in.SubprojectID = nullable.Present(&id)
	}
	if f.ideas && !cmd.Flags().Changed("project-id") && !cmd.Flags().Changed("subproject-id") {
		in.ProjectID = nullable.Clear[*int64]()
		in.SubprojectID = nullable.Clear[*int64]()
	}
	return in
}

func taskCommand(use string, s *commandState, update bool) *cobra.Command {
	var f taskFlags
	c := &cobra.Command{Use: use, Args: mutationArgs(update), RunE: func(cmd *cobra.Command, args []string) error {
		var v task.Task
		var err error
		if update {
			in := taskPatch(cmd, f)
			id, e := idArg(args)
			if e != nil {
				return e
			}
			v, err = s.client.UpdateTask(cmd.Context(), id, in)
		} else {
			in := taskInput(cmd, f)
			v, err = s.client.CreateTask(cmd.Context(), in)
		}
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return taskTable(w, []task.Task{v}) })
	}}
	f.addFlags(c)
	return c
}

package main

import (
	"context"

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
		return writeJSON(s.out, v)
	}}
	list.Flags().BoolVar(&ideas, "ideas", false, "list unassigned idea tasks")
	list.Flags().StringVar(&projectName, "project", "", "filter by unique project name")
	list.Flags().Int64Var(&projectID, "project-id", 0, "filter by project ID")
	list.Flags().StringVar(&subprojectName, "subproject", "", "filter by subproject name within the project")
	list.Flags().Int64Var(&subprojectID, "subproject-id", 0, "filter by subproject ID")
	root.AddCommand(list, getCommand(s, s.client.Task), taskCommand("create", s, false), taskCommand("update <id>", s, true), deleteCommand("delete", "task", s, func(ctx context.Context, id int64) error { return s.client.DeleteTask(ctx, id) }))
	return root
}

type taskFlags struct {
	name, description, notes, department, developers, priority, manualEstimate string
	projectID, subprojectID                                                    int64
	ideas                                                                      bool
}

func (f *taskFlags) addFlags(c *cobra.Command) {
	c.Flags().StringVar(&f.name, "name", "", "task name")
	c.Flags().StringVar(&f.description, "description", "", "task description")
	c.Flags().StringVar(&f.notes, "implementation-notes", "", "task implementation notes")
	c.Flags().StringVar(&f.department, "department", "", "relevant department")
	c.Flags().StringVar(&f.developers, "developers", "", "developer or developers")
	c.Flags().StringVar(&f.priority, "priority", "", "task priority")
	c.Flags().StringVar(&f.manualEstimate, "manual-estimate", "", "manual estimate in hours (\"null\" returns to automatic)")
	c.Flags().Int64Var(&f.projectID, "project-id", 0, "project ID")
	c.Flags().Int64Var(&f.subprojectID, "subproject-id", 0, "subproject ID")
	c.Flags().BoolVar(&f.ideas, "ideas", false, "leave the task unassigned")
}

func taskInput(cmd *cobra.Command, f taskFlags) (task.Input, error) {
	in := task.Input{
		Name: f.name, Description: f.description, ImplementationNotes: f.notes,
		Department: f.department, Developers: f.developers, Priority: f.priority,
		ProjectID:    opt(cmd, "project-id", f.projectID).Value,
		SubprojectID: opt(cmd, "subproject-id", f.subprojectID).Value,
	}
	if f.ideas && !cmd.Flags().Changed("project-id") && !cmd.Flags().Changed("subproject-id") {
		in.ProjectID, in.SubprojectID = nil, nil
	}
	manual, err := manualEstimateOpt(cmd, f.manualEstimate)
	if err != nil {
		return in, err
	}
	in.ManualEstimate = manual.ValueOr(nil)
	return in, nil
}

func taskCommand(use string, s *commandState, update bool) *cobra.Command {
	var f taskFlags
	c := &cobra.Command{Use: use, Args: mutationArgs(update), RunE: func(cmd *cobra.Command, args []string) error {
		var v task.Task
		var err error
		if update {
			var in task.Patch
			in, err = taskPatchFromFlags(cmd, f)
			if err != nil {
				return err
			}
			id, e := idArg(args)
			if e != nil {
				return e
			}
			v, err = s.client.UpdateTask(cmd.Context(), id, in)
		} else {
			var in task.Input
			in, err = taskInput(cmd, f)
			if err != nil {
				return err
			}
			v, err = s.client.CreateTask(cmd.Context(), in)
		}
		if err != nil {
			return err
		}
		return writeJSON(s.out, v)
	}}
	f.addFlags(c)
	return c
}

package main

import (
	"context"
	"io"

	"cad-development/internal/client"
	"cad-development/internal/nullable"
	"github.com/spf13/cobra"
)

func subprojectCommands(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "subprojects"}
	var projectName string
	var projectID int64
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		selectedProject, err := resolveProjectFilter(cmd.Context(), s.client, projectName, projectID, cmd.Flags().Changed("project-id"))
		if err != nil {
			return err
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
	c := &cobra.Command{Use: use, Args: mutationArgs(update), RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		var v client.Subproject
		if update {
			in := client.SubprojectPatch{}
			if cmd.Flags().Changed("project-id") {
				in.ProjectID = nullable.Set(f.projectID)
			}
			if cmd.Flags().Changed("name") {
				in.Name = textPatch(f.name)
			}
			if cmd.Flags().Changed("total-hours") {
				in.TotalHours = nullable.Set(f.totalHours)
			}
			id, e := idArg(args)
			if e != nil {
				return e
			}
			v, err = s.client.UpdateSubproject(cmd.Context(), id, in)
		} else {
			totalHours := (*float64)(nil)
			if cmd.Flags().Changed("total-hours") {
				totalHours = &f.totalHours
			}
			in := client.SubprojectInput{ProjectID: f.projectID, Name: f.name, TotalHours: totalHours}
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

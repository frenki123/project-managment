package main

import (
	"context"
	"io"

	"cad-development/internal/project"
	"github.com/spf13/cobra"
)

func projectCommands(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "projects"}
	root.AddCommand(&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		v, err := s.client.Projects(cmd.Context())
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return projectTable(w, v.Projects) })
	}})
	root.AddCommand(getCommand(s, s.client.Project, projectTable))
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
	c := &cobra.Command{Use: use, Args: mutationArgs(update), RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		var v project.Project
		if update {
			id, e := idArg(args)
			if e != nil {
				return e
			}
			v, err = s.client.UpdateProject(cmd.Context(), id, project.Patch{
				Name:              stringOpt(cmd, "name", f.name),
				PurchaseOrderName: stringOpt(cmd, "purchase-order-name", f.purchaseOrder),
				TotalHours:        opt(cmd, "total-hours", f.totalHours),
				StartDate:         stringOpt(cmd, "start-date", f.startDate),
				EndDate:           stringOpt(cmd, "end-date", f.endDate),
			})
		} else {
			in := project.Input{Name: f.name, PurchaseOrderName: f.purchaseOrder, StartDate: f.startDate, EndDate: f.endDate, TotalHours: opt(cmd, "total-hours", f.totalHours).Value}
			v, err = s.client.CreateProject(cmd.Context(), in)
		}
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return projectTable(w, []project.Project{v}) })
	}}
	f.addFlags(c)
	return c
}

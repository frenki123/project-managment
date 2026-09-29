package main

import (
	"context"
	"io"

	"cad-development/internal/client"
	"cad-development/internal/nullable"
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
	c := &cobra.Command{Use: use, Args: mutationArgs(update), RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		var v client.Project
		if update {
			in := client.ProjectPatch{}
			if cmd.Flags().Changed("name") {
				in.Name = textPatch(f.name)
			}
			if cmd.Flags().Changed("purchase-order-name") {
				in.PurchaseOrderName = textPatch(f.purchaseOrder)
			}
			if cmd.Flags().Changed("total-hours") {
				in.TotalHours = nullable.Set(f.totalHours)
			}
			if cmd.Flags().Changed("start-date") {
				in.StartDate = textPatch(f.startDate)
			}
			if cmd.Flags().Changed("end-date") {
				in.EndDate = textPatch(f.endDate)
			}
			id, e := idArg(args)
			if e != nil {
				return e
			}
			v, err = s.client.UpdateProject(cmd.Context(), id, in)
		} else {
			var totalHours *float64
			if cmd.Flags().Changed("total-hours") {
				totalHours = &f.totalHours
			}
			in := client.ProjectInput{Name: f.name, PurchaseOrderName: f.purchaseOrder, TotalHours: totalHours, StartDate: f.startDate, EndDate: f.endDate}
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

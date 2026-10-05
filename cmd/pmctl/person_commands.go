package main

import (
	"context"

	"cad-development/internal/client"
	"cad-development/internal/person"
	"github.com/spf13/cobra"
)

//nolint:dupl // mirrors projectCommands; the client methods and command names differ.
func personCommands(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "people"}
	root.AddCommand(&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		v, err := s.client.People(cmd.Context())
		if err != nil {
			return err
		}
		return writeJSON(s.out, v)
	}})
	root.AddCommand(getCommand(s, (*client.Client).Person))
	root.AddCommand(personCommand("create", s, false), personCommand("update <id>", s, true), deleteCommand("delete", "person", s, func(ctx context.Context, id int64) error { return s.client.DeletePerson(ctx, id) }))
	return root
}

type personFlags struct {
	name           string
	weeklyCapacity float64
}

func (f *personFlags) addFlags(c *cobra.Command) {
	c.Flags().StringVar(&f.name, "name", "", "person name")
	c.Flags().Float64Var(&f.weeklyCapacity, "weekly-capacity", 0, "weekly capacity in hours (default 40)")
}

func personCommand(use string, s *commandState, update bool) *cobra.Command {
	var f personFlags
	c := &cobra.Command{Use: use, Args: mutationArgs(update), RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		var v person.Person
		if update {
			id, e := idArg(args)
			if e != nil {
				return e
			}
			v, err = s.client.UpdatePerson(cmd.Context(), id, person.Patch{
				Name:           stringOpt(cmd, "name", f.name),
				WeeklyCapacity: opt(cmd, "weekly-capacity", f.weeklyCapacity),
			})
		} else {
			in := person.Input{Name: f.name}
			if cmd.Flags().Changed("weekly-capacity") {
				in.WeeklyCapacity = new(f.weeklyCapacity)
			}
			v, err = s.client.CreatePerson(cmd.Context(), in)
		}
		if err != nil {
			return err
		}
		return writeJSON(s.out, v)
	}}
	f.addFlags(c)
	return c
}
package main

import (
	"context"
	"fmt"
	"strconv"

	"cad-development/internal/person"
	"github.com/spf13/cobra"
)

func personCommands(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "people"}
	root.AddCommand(&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		v, err := s.client.People(cmd.Context())
		if err != nil {
			return err
		}
		return writeJSON(s.out, v)
	}})
	root.AddCommand(getCommand(s, s.client.Person))
	root.AddCommand(personCommand("create", s, false), personCommand("update <id>", s, true), deleteCommand("delete", "person", s, func(ctx context.Context, id int64) error { return s.client.DeletePerson(ctx, id) }))
	root.AddCommand(personOverridesCommands(s))
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

func personOverridesCommands(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "overrides"}
	root.AddCommand(&cobra.Command{Use: "list <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := idArg(args)
		if err != nil {
			return err
		}
		v, err := s.client.PersonOverrides(cmd.Context(), id)
		if err != nil {
			return err
		}
		return writeJSON(s.out, v)
	}})
	var capacity float64
	set := &cobra.Command{Use: "set <id> <week-start>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		id, week, err := personWeekArgs(args)
		if err != nil {
			return err
		}
		override, err := s.client.SetPersonOverride(cmd.Context(), id, week, capacity)
		if err != nil {
			return err
		}
		return writeJSON(s.out, override)
	}}
	set.Flags().Float64Var(&capacity, "capacity", 0, "weekly capacity override in hours")
	_ = set.MarkFlagRequired("capacity")
	root.AddCommand(set)
	root.AddCommand(&cobra.Command{Use: "clear <id> <week-start>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		id, week, err := personWeekArgs(args)
		if err != nil {
			return err
		}
		if err := s.client.ClearPersonOverride(cmd.Context(), id, week); err != nil {
			return err
		}
		return writeJSON(s.out, result{Deleted: true, ID: id, Type: "person override"})
	}})
	return root
}

func personWeekArgs(args []string) (int64, string, error) {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid person id %q", args[0])
	}
	return id, args[1], nil
}
package main

import (
	"cad-development/internal/weekly"
	"github.com/spf13/cobra"
)

func updateWeekDeveloperCommand(s *commandState) *cobra.Command {
	var planned, spent float64
	var unlock bool
	c := &cobra.Command{Use: "update-task-week-developer <task-id> <week-start> <person-id>", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		taskID, week, personID, err := threeArgs(args)
		if err != nil {
			return err
		}
		v, err := s.client.UpdateTaskWeekDeveloper(cmd.Context(), taskID, week, personID, weekly.AttributionPatch{PlannedHours: planned, SpentHours: spent, Unlock: unlock})
		if err != nil {
			return err
		}
		return writeJSON(s.out, v)
	}}
	c.Flags().Float64Var(&planned, "planned-hours", 0, "planned hours")
	c.Flags().Float64Var(&spent, "spent-hours", 0, "spent hours")
	c.Flags().BoolVar(&unlock, "unlock", false, "temporarily unlock history for this update")
	return c
}

func clearWeekDeveloperCommand(s *commandState) *cobra.Command {
	var unlock bool
	c := &cobra.Command{Use: "clear-task-week-developer <task-id> <week-start> <person-id>", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		taskID, week, personID, err := threeArgs(args)
		if err != nil {
			return err
		}
		if err := s.client.ClearTaskWeekDeveloper(cmd.Context(), taskID, week, personID, unlock); err != nil {
			return err
		}
		return writeJSON(s.out, result{Deleted: true, ID: personID, Type: "task week developer"})
	}}
	c.Flags().BoolVar(&unlock, "unlock", false, "temporarily unlock history for this update")
	return c
}
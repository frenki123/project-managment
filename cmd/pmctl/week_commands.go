package main

import (
	"github.com/spf13/cobra"
)

func updateWeekCommand(s *commandState) *cobra.Command {
	var planned, spent float64
	var progress string
	var note string
	var unlock bool
	c := &cobra.Command{Use: "update-task-week <task-id> <week-start>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		id, week, err := twoArgs(args)
		if err != nil {
			return err
		}
		in, err := weekPatchFromFlags(cmd, planned, spent, progress, note, unlock)
		if err != nil {
			return err
		}
		v, err := s.client.UpdateTaskWeek(cmd.Context(), id, week, in)
		if err != nil {
			return err
		}
		return writeJSON(s.out, v)
	}}
	c.Flags().Float64Var(&planned, "planned-hours", 0, "planned hours")
	c.Flags().Float64Var(&spent, "spent-hours", 0, "spent hours")
	c.Flags().StringVar(&progress, "progress", "", "progress percentage or \"null\" to clear")
	c.Flags().StringVar(&note, "note", "", "weekly note or \"null\" to clear")
	c.Flags().BoolVar(&unlock, "unlock", false, "temporarily unlock history for this update")
	return c
}

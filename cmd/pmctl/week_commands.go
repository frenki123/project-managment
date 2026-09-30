package main

import (
	"io"

	"cad-development/internal/nullable"
	"cad-development/internal/weekly"
	"github.com/spf13/cobra"
)

func updateWeekCommand(s *commandState) *cobra.Command {
	var planned, spent, progress float64
	var unlock bool
	c := &cobra.Command{Use: "update-task-week <task-id> <week-start>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		id, week, err := twoArgs(args)
		if err != nil {
			return err
		}
		in := weekly.Patch{}
		if cmd.Flags().Changed("planned-hours") {
			in.PlannedHours = nullable.Present(planned)
		}
		if cmd.Flags().Changed("spent-hours") {
			in.SpentHours = nullable.Present(spent)
		}
		if cmd.Flags().Changed("progress") {
			in.Progress = nullable.Present(progress)
		}
		in.Unlock = unlock
		v, err := s.client.UpdateTaskWeek(cmd.Context(), id, week, in)
		if err != nil {
			return err
		}
		return s.printer().print(v, func(w io.Writer) error { return weekTable(w, v) })
	}}
	c.Flags().Float64Var(&planned, "planned-hours", 0, "planned hours")
	c.Flags().Float64Var(&spent, "spent-hours", 0, "spent hours")
	c.Flags().Float64Var(&progress, "progress", 0, "progress percentage")
	c.Flags().BoolVar(&unlock, "unlock", false, "temporarily unlock history for this update")
	return c
}

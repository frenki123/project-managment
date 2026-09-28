package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"cad-development/internal/client"
	"github.com/spf13/cobra"
)

func updateWeekCommand(s *commandState) *cobra.Command {
	var planned, spent, progress float64
	var clearProgress, unlock bool
	c := &cobra.Command{Use: "update-task-week <task-id> <week-start>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		id, week, err := twoArgs(args)
		if err != nil {
			return err
		}
		in := client.WeekPatch{ClearProgress: clearProgress}
		if cmd.Flags().Changed("planned-hours") {
			in.PlannedHours = &planned
		}
		if cmd.Flags().Changed("spent-hours") {
			in.SpentHours = &spent
		}
		if cmd.Flags().Changed("progress") {
			in.Progress = &progress
		}
		if unlock {
			if _, err := s.client.SetMonthLock(cmd.Context(), true); err != nil {
				return fmt.Errorf("unlock history: %w", err)
			}
		}
		v, updateErr := s.client.UpdateTaskWeek(cmd.Context(), id, week, in)
		var lockErr error
		if unlock {
			lockErr = lockAfter(s.client)
		}
		if updateErr != nil {
			if lockErr != nil {
				return fmt.Errorf("update task week: %w; lock history: %v", updateErr, lockErr)
			}
			return updateErr
		}
		if lockErr != nil {
			return fmt.Errorf("lock history: %w", lockErr)
		}
		return s.printer().print(v, func(w io.Writer) error { return weekTable(w, v) })
	}}
	c.Flags().Float64Var(&planned, "planned-hours", 0, "planned hours")
	c.Flags().Float64Var(&spent, "spent-hours", 0, "spent hours")
	c.Flags().Float64Var(&progress, "progress", 0, "progress percentage")
	c.Flags().BoolVar(&clearProgress, "clear-progress", false, "clear explicitly entered progress")
	c.Flags().BoolVar(&unlock, "unlock", false, "temporarily unlock history for this update")
	return c
}

func lockAfter(c *client.Client) error {
	cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if _, err = c.SetMonthLock(cleanup, false); err == nil {
			return nil
		}
		if apiErr, ok := errors.AsType[*client.APIError](err); ok && apiErr.Status < 500 {
			break
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}
	return err
}

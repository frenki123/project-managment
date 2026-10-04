package main

import (
	"fmt"
	"strconv"

	"cad-development/internal/nullable"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
	"github.com/spf13/cobra"
)

func opt[T any](cmd *cobra.Command, name string, value T) nullable.Optional[T] {
	if !cmd.Flags().Changed(name) {
		return nullable.Optional[T]{}
	}
	return nullable.Present(value)
}

func stringOpt(cmd *cobra.Command, name, value string) nullable.Optional[string] {
	if o := opt(cmd, name, value); value != "null" {
		return o
	}
	return nullable.Clear[string]()
}

func taskPatchFromFlags(cmd *cobra.Command, f taskFlags) task.Patch {
	in := task.Patch{
		Name:                stringOpt(cmd, "name", f.name),
		Description:         stringOpt(cmd, "description", f.description),
		ImplementationNotes: stringOpt(cmd, "implementation-notes", f.notes),
		Department:          stringOpt(cmd, "department", f.department),
		DeveloperIDs:        developerIDsOpt(cmd, f),
		Priority:            stringOpt(cmd, "priority", f.priority),
		ProjectID:           opt(cmd, "project-id", &f.projectID),
		SubprojectID:        opt(cmd, "subproject-id", &f.subprojectID),
	}
	if f.ideas && !cmd.Flags().Changed("project-id") && !cmd.Flags().Changed("subproject-id") {
		in.ProjectID, in.SubprojectID = nullable.Clear[*int64](), nullable.Clear[*int64]()
	}
	return in
}

func developerIDsOpt(cmd *cobra.Command, f taskFlags) nullable.Optional[[]int64] {
	if f.clearDevelopers {
		return nullable.Clear[[]int64]()
	}
	if cmd.Flags().Changed("developer-id") {
		return nullable.Present(f.developerIDs)
	}
	return nullable.Optional[[]int64]{}
}

func weekPatchFromFlags(cmd *cobra.Command, planned, spent float64, progress string, unlock bool) (weekly.Patch, error) {
	in := weekly.Patch{
		PlannedHours: opt(cmd, "planned-hours", planned),
		SpentHours:   opt(cmd, "spent-hours", spent),
		Unlock:       unlock,
	}
	if cmd.Flags().Changed("progress") {
		if progress == "null" {
			in.Progress = nullable.Clear[float64]()
		} else if v, err := strconv.ParseFloat(progress, 64); err != nil {
			return weekly.Patch{}, fmt.Errorf("invalid progress %q: expected a number or \"null\"", progress)
		} else {
			in.Progress = nullable.Present(v)
		}
	}
	return in, nil
}

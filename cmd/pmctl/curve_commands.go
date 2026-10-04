package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func curveCommand(s *commandState) *cobra.Command {
	var projectName string
	var projectID int64
	c := &cobra.Command{Use: "project-s-curve", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if projectName == "" && !cmd.Flags().Changed("project-id") {
			return errors.New("--project or --project-id is required")
		}
		id := projectID
		var err error
		if projectName != "" {
			id, err = resolveProject(cmd.Context(), s.client, projectName)
		}
		if err != nil {
			return err
		}
		v, err := s.client.SCurve(cmd.Context(), id)
		if err != nil {
			return err
		}
		return writeJSON(s.out, v)
	}}
	c.Flags().StringVar(&projectName, "project", "", "project name")
	c.Flags().Int64Var(&projectID, "project-id", 0, "project ID")
	return c
}

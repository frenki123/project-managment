package main

import (
	"context"

	"cad-development/internal/client"
	"github.com/spf13/cobra"
)

func resolveProjectFilter(ctx context.Context, c *client.Client, name string, id int64, idChanged bool) (*int64, error) {
	if name != "" {
		resolved, err := resolveProject(ctx, c, name)
		if err != nil {
			return nil, err
		}
		return &resolved, nil
	}
	if idChanged {
		return &id, nil
	}
	return nil, nil
}

func resolveSubprojectFilter(ctx context.Context, c *client.Client, projectID *int64, name string, id int64, idChanged bool) (*int64, error) {
	if name != "" {
		resolved, err := resolveSubproject(ctx, c, projectID, name)
		if err != nil {
			return nil, err
		}
		return &resolved, nil
	}
	if idChanged {
		return &id, nil
	}
	return nil, nil
}

func mutationArgs(update bool) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if update {
			return cobra.ExactArgs(1)(cmd, args)
		}
		return cobra.NoArgs(cmd, args)
	}
}

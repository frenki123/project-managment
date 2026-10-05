package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"cad-development/internal/client"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"github.com/spf13/cobra"
)

type commandState struct {
	apiURL      string
	out, errOut io.Writer
	client      *client.Client
}

// version is set at build time with -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	state := &commandState{apiURL: os.Getenv("CAD_API_URL"), out: os.Stdout, errOut: os.Stderr}
	if state.apiURL == "" {
		state.apiURL = client.DefaultURL
	}
	root := newRoot(state)
	if err := root.Execute(); err != nil {
		writeError(state.errOut, err)
		os.Exit(1)
	}
}

func writeError(w io.Writer, err error) {
	payload := result{Error: err.Error()}
	if apiErr, ok := errors.AsType[*client.APIError](err); ok {
		payload.Status = apiErr.Status
		payload.Reason = apiErr.Reason
	}
	if err := writeJSON(w, payload); err != nil {
		fmt.Fprintln(w, err)
	}
}

func newRoot(s *commandState) *cobra.Command {
	root := &cobra.Command{Use: "pmctl", Short: "Control the project management API", Version: version, SilenceUsage: true, SilenceErrors: true, PersistentPreRunE: func(*cobra.Command, []string) error {
		c, err := client.New(s.apiURL)
		if err != nil {
			return err
		}
		s.client = c
		return nil
	}}
	root.PersistentFlags().StringVar(&s.apiURL, "url", s.apiURL, "REST API base URL")
	root.AddCommand(projectCommands(s), subprojectCommands(s), taskCommands(s), personCommands(s), updateWeekCommand(s), curveCommand(s))
	return root
}

func idArg(args []string) (int64, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("expected one numeric id")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q", args[0])
	}
	return id, nil
}

func twoArgs(args []string) (int64, string, error) {
	if len(args) != 2 {
		return 0, "", fmt.Errorf("expected task id and week start")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid task id %q", args[0])
	}
	return id, args[1], nil
}

func resolveProject(ctx context.Context, c *client.Client, name string) (int64, error) {
	projects, err := c.Projects(ctx)
	if err != nil {
		return 0, err
	}
	want := strings.TrimSpace(name)
	var match *project.Project
	for i := range projects.Projects {
		p := &projects.Projects[i]
		if strings.EqualFold(p.Name, want) {
			if match != nil {
				return 0, fmt.Errorf("project %q is ambiguous", name)
			}
			match = p
		}
	}
	if match != nil {
		return match.ID, nil
	}
	return 0, fmt.Errorf("project %q not found", name)
}

func resolveSubproject(ctx context.Context, c *client.Client, projectID *int64, name string) (int64, error) {
	subprojects, err := c.Subprojects(ctx, projectID)
	if err != nil {
		return 0, err
	}
	want := strings.TrimSpace(name)
	var match *subproject.Subproject
	for i := range subprojects.Subprojects {
		s := &subprojects.Subprojects[i]
		if strings.EqualFold(s.Name, want) {
			if match != nil {
				if projectID == nil {
					return 0, fmt.Errorf("subproject %q is ambiguous", name)
				}
				return 0, fmt.Errorf("subproject %q is ambiguous in project %d", name, *projectID)
			}
			match = s
		}
	}
	if match != nil {
		return match.ID, nil
	}
	if projectID == nil {
		return 0, fmt.Errorf("subproject %q not found", name)
	}
	return 0, fmt.Errorf("subproject %q not found in project %q", name, strconv.FormatInt(*projectID, 10))
}

func getCommand[V any](s *commandState, get func(*client.Client, context.Context, int64) (V, error)) *cobra.Command {
	return &cobra.Command{Use: "get <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := idArg(args)
		if err != nil {
			return err
		}
		v, err := get(s.client, cmd.Context(), id)
		if err != nil {
			return err
		}
		return writeJSON(s.out, v)
	}}
}

func deleteCommand(use, kind string, s *commandState, del func(context.Context, int64) error) *cobra.Command {
	return &cobra.Command{Use: use + " <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := idArg(args)
		if err != nil {
			return err
		}
		if err := del(cmd.Context(), id); err != nil {
			return err
		}
		return writeJSON(s.out, result{Deleted: true, ID: id, Type: kind})
	}}
}

package task

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strconv"

	"cad-development/internal/db"
	"cad-development/internal/web"
)

// Filter selects all tasks, idea tasks, or a project and optional subproject.
// A subproject takes precedence over project, ideas, and all.
type Filter struct {
	All        bool
	Ideas      bool
	ID         int64
	Subproject *int64
}

type ResolvedFilter struct {
	Filter
	Project    *db.Project
	Subproject *db.Subproject
}

func ParseFilter(projectKey, subprojectValue string) (Filter, error) {
	f := Filter{All: projectKey == "" || projectKey == "all"}
	if projectKey == "ideas" {
		f.Ideas = true
	}
	if !f.All && !f.Ideas {
		id, err := strconv.ParseInt(projectKey, 10, 64)
		if err != nil || id < 1 {
			return Filter{}, web.Invalid("invalid project")
		}
		f.ID = id
	}
	if subprojectValue != "" {
		id, err := strconv.ParseInt(subprojectValue, 10, 64)
		if err != nil || id < 1 {
			return Filter{}, web.Invalid("invalid subproject")
		}
		f.Subproject = &id
	}
	return f, nil
}

func FilterKey(f Filter) string {
	project := "all"
	if f.Ideas {
		project = "ideas"
	} else if !f.All {
		project = strconv.FormatInt(f.ID, 10)
	}
	values := url.Values{"project": []string{project}}
	if f.Subproject != nil {
		values.Set("subproject", strconv.FormatInt(*f.Subproject, 10))
	}
	return values.Encode()
}

func ResolveFilter(ctx context.Context, q *db.Queries, f Filter) (ResolvedFilter, error) {
	resolved := ResolvedFilter{Filter: f}
	if !f.All && !f.Ideas {
		project, err := q.GetProject(ctx, f.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return ResolvedFilter{}, web.Missing("project not found")
		} else if err != nil {
			return ResolvedFilter{}, err
		}
		resolved.Project = &project
	}
	if f.Subproject == nil {
		return resolved, nil
	}
	sp, err := q.GetSubproject(ctx, *f.Subproject)
	if errors.Is(err, sql.ErrNoRows) {
		return ResolvedFilter{}, web.Missing("subproject not found")
	}
	if err != nil {
		return ResolvedFilter{}, err
	}
	if !f.All && !f.Ideas && f.ID != sp.ProjectID {
		return ResolvedFilter{}, web.Invalid("subproject does not belong to project")
	}
	resolved.Subproject = &sp
	if resolved.Project == nil {
		project, err := q.GetProject(ctx, sp.ProjectID)
		if errors.Is(err, sql.ErrNoRows) {
			return ResolvedFilter{}, web.Missing("project not found")
		}
		if err != nil {
			return ResolvedFilter{}, err
		}
		resolved.Project = &project
	}
	return resolved, nil
}

func ListByFilter(ctx context.Context, q *db.Queries, f Filter) ([]Task, error) {
	if _, err := ResolveFilter(ctx, q, f); err != nil {
		return nil, err
	}
	switch {
	case f.Subproject != nil:
		return ListBySubproject(ctx, q, *f.Subproject)
	case !f.All && !f.Ideas:
		return ListByProject(ctx, q, f.ID)
	case f.Ideas:
		return ListIdeas(ctx, q)
	default:
		return List(ctx, q)
	}
}

package task

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/db"
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
	ProjectID  int64
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
			return Filter{}, app.Invalid("invalid project")
		}
		f.ID = id
	}
	if subprojectValue != "" {
		id, err := strconv.ParseInt(subprojectValue, 10, 64)
		if err != nil || id < 1 {
			return Filter{}, app.Invalid("invalid subproject")
		}
		f.Subproject = &id
	}
	return f, nil
}

func (f Filter) Key() string {
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

func (f Filter) Validate(ctx context.Context, q *db.Queries) error {
	_, err := f.Resolve(ctx, q)
	return err
}

func (f Filter) Resolve(ctx context.Context, q *db.Queries) (ResolvedFilter, error) {
	resolved := ResolvedFilter{Filter: f, ProjectID: f.ID}
	if !f.All && !f.Ideas {
		if _, err := q.GetProject(ctx, f.ID); errors.Is(err, sql.ErrNoRows) {
			return ResolvedFilter{}, app.Missing("project not found")
		} else if err != nil {
			return ResolvedFilter{}, err
		}
	}
	if f.Subproject == nil {
		return resolved, nil
	}
	sp, err := q.GetSubproject(ctx, *f.Subproject)
	if errors.Is(err, sql.ErrNoRows) {
		return ResolvedFilter{}, app.Missing("subproject not found")
	}
	if err != nil {
		return ResolvedFilter{}, err
	}
	if !f.All && !f.Ideas && f.ID != sp.ProjectID {
		return ResolvedFilter{}, app.Invalid("subproject does not belong to project")
	}
	resolved.ProjectID = sp.ProjectID
	resolved.Subproject = &sp
	return resolved, nil
}

func (f Filter) List(ctx context.Context, q *db.Queries) ([]Task, error) {
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

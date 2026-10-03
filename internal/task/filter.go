package task

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strconv"

	"cad-development/internal/db"
	"cad-development/internal/nullable"
	"cad-development/internal/web"
)

// Filter selects all tasks, idea tasks, or a project and optional subproject.
// Ideas and subprojects are mutually exclusive by schema, so a filter never needs both.
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
	if f.Ideas && subprojectValue != "" {
		return Filter{}, web.Invalid("ideas cannot be filtered by subproject")
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

// ValidateFilter reports whether the filter identifies an existing scope, without loading
// any rows. Missing projects or subprojects are 404; a subproject from another project is 400.
func ValidateFilter(ctx context.Context, q *db.Queries, f Filter) error {
	if (f.All || f.Ideas) && f.Subproject == nil {
		return nil
	}
	var projectID, subprojectID sql.NullInt64
	if !f.All && !f.Ideas {
		projectID = nullable.Int64(&f.ID)
	}
	if f.Subproject != nil {
		subprojectID = nullable.Int64(f.Subproject)
	}
	scope, err := q.TaskFilterScope(ctx, db.TaskFilterScopeParams{ProjectID: projectID, SubprojectID: subprojectID})
	if err != nil {
		return err
	}
	if !f.All && !f.Ideas && scope.ProjectExists == 0 {
		return web.Missing("project not found")
	}
	if f.Subproject != nil && scope.SubprojectExists == 0 {
		return web.Missing("subproject not found")
	}
	if !f.All && !f.Ideas && f.Subproject != nil && scope.SubprojectProjectID != f.ID {
		return web.Invalid("subproject does not belong to project")
	}
	return nil
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
	if f.Ideas {
		return ListIdeas(ctx, q)
	}
	var projectID, subprojectID sql.NullInt64
	if !f.All {
		projectID = nullable.Int64(&f.ID)
	}
	if f.Subproject != nil {
		subprojectID = nullable.Int64(f.Subproject)
	}
	list, err := listScoped(ctx, q, projectID, subprojectID)
	if err != nil {
		return nil, err
	}
	// Rows exist, so the filter is valid: the (project_id, subproject_id) FK guarantees an invalid filter matches nothing.
	if len(list) > 0 {
		return list, nil
	}
	return list, ValidateFilter(ctx, q, f)
}

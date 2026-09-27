package task

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/db"
)

func ParseFilter(projectKey, subprojectValue string) (string, *int64, error) {
	if projectKey == "" {
		projectKey = "ideas"
	}
	if subprojectValue == "" {
		return projectKey, nil, nil
	}
	subprojectID, err := strconv.ParseInt(subprojectValue, 10, 64)
	if err != nil || subprojectID < 1 {
		return "", nil, app.Invalid("invalid subproject")
	}
	if _, err := parseProjectFilter(projectKey); err != nil {
		return "", nil, err
	}
	return projectKey, &subprojectID, nil
}

func ValidateFilter(ctx context.Context, q *db.Queries, projectKey string, subprojectID *int64) error {
	filter, err := parseProjectFilter(projectKey)
	if err != nil {
		return err
	}
	if filter.Ideas {
		return nil
	}
	if _, err := q.GetProject(ctx, filter.ID); errors.Is(err, sql.ErrNoRows) {
		return app.Missing("project not found")
	} else if err != nil {
		return err
	}
	if subprojectID != nil {
		if _, err := q.GetSubproject(ctx, *subprojectID); errors.Is(err, sql.ErrNoRows) {
			return app.Missing("subproject not found")
		} else if err != nil {
			return err
		}
	}
	return nil
}

func NormalizeSubprojectFilter(ctx context.Context, q *db.Queries, projectKey string, subprojectID *int64) (*int64, error) {
	if subprojectID == nil {
		return nil, nil
	}
	filter, err := parseProjectFilter(projectKey)
	if err != nil || filter.Ideas {
		return nil, err
	}
	sp, err := q.GetSubproject(ctx, *subprojectID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && sp.ProjectID != filter.ID) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return subprojectID, nil
}

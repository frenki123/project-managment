-- name: CreateSubproject :one
INSERT INTO subprojects (project_id, name, total_hours)
VALUES (?, ?, ?)
RETURNING id, project_id, name, total_hours;

-- name: GetSubproject :one
SELECT id, project_id, name, total_hours FROM subprojects WHERE id = ?;

-- name: ListSubprojects :many
SELECT id, project_id, name, total_hours
FROM subprojects ORDER BY name COLLATE NOCASE, id;

-- name: ListSubprojectsByProject :many
SELECT id, project_id, name, total_hours
FROM subprojects WHERE project_id = ? ORDER BY name COLLATE NOCASE, id;

-- name: SumSubprojectHoursByProject :one
SELECT CAST(COALESCE(SUM(total_hours), 0) AS REAL) FROM subprojects WHERE project_id = ?;

-- name: SumSubprojectHoursByProjectExcept :one
SELECT CAST(COALESCE(SUM(total_hours), 0) AS REAL) FROM subprojects WHERE project_id = ? AND id != ?;

-- name: UpdateSubproject :one
UPDATE subprojects SET
    project_id = ?,
    name = ?,
    total_hours = ?
WHERE id = ?
RETURNING id, project_id, name, total_hours;

-- name: DeleteSubproject :exec
DELETE FROM subprojects WHERE id = ?;

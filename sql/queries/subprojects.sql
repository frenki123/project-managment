-- name: CreateSubproject :one
INSERT INTO subprojects (project_id, name, total_hours)
VALUES (?, ?, ?)
RETURNING id, project_id, name, total_hours;

-- name: GetSubproject :one
SELECT id, project_id, name, total_hours FROM subprojects WHERE id = ?;

-- name: GetSubprojectTotals :one
SELECT
    CAST(COALESCE(SUM(planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(spent_hours), 0) AS REAL) AS spent_hours
FROM v_task_totals
WHERE subproject_id = CAST(sqlc.arg(subproject_id) AS INTEGER);

-- name: ListSubprojects :many
SELECT id, project_id, name, total_hours
FROM subprojects ORDER BY name COLLATE NOCASE, id;

-- name: ListSubprojectsWithTotals :many
SELECT
    s.id, s.project_id, s.name, s.total_hours,
    CAST(COALESCE(SUM(tt.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tt.spent_hours), 0) AS REAL) AS spent_hours
FROM subprojects s
LEFT JOIN v_task_totals tt ON tt.subproject_id = s.id
GROUP BY s.id
ORDER BY s.name COLLATE NOCASE, s.id;

-- name: ListSubprojectsByProject :many
SELECT id, project_id, name, total_hours
FROM subprojects WHERE project_id = ? ORDER BY name COLLATE NOCASE, id;

-- name: ListSubprojectsByProjectWithTotals :many
SELECT
    s.id, s.project_id, s.name, s.total_hours,
    CAST(COALESCE(SUM(tt.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tt.spent_hours), 0) AS REAL) AS spent_hours
FROM subprojects s
LEFT JOIN v_task_totals tt ON tt.subproject_id = s.id
WHERE s.project_id = ?
GROUP BY s.id
ORDER BY s.name COLLATE NOCASE, s.id;

-- name: SumSubprojectHoursByProject :one
SELECT CAST(COALESCE(SUM(total_hours), 0) AS REAL) FROM subprojects WHERE project_id = ?;

-- name: SumSubprojectHoursByProjectExcept :one
SELECT CAST(COALESCE(SUM(total_hours), 0) AS REAL) FROM subprojects WHERE project_id = ? AND id != ?;

-- name: CountTasksBySubproject :one
SELECT COUNT(*) FROM tasks WHERE subproject_id = CAST(? AS INTEGER);

-- name: UpdateSubproject :one
UPDATE subprojects SET
    project_id = ?,
    name = ?,
    total_hours = ?
WHERE id = ?
RETURNING id, project_id, name, total_hours;

-- name: DeleteSubproject :one
DELETE FROM subprojects
WHERE id = ?
RETURNING id;

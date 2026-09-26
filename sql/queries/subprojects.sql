-- name: CreateSubproject :one
INSERT INTO subprojects (project_id, name, total_hours)
VALUES (?, ?, ?)
RETURNING id, project_id, name, total_hours;

-- name: GetSubproject :one
SELECT id, project_id, name, total_hours FROM subprojects WHERE id = ?;

-- name: GetSubprojectTotals :one
SELECT
    CAST(COALESCE(SUM(tw.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tw.spent_hours), 0) AS REAL) AS spent_hours
FROM tasks t
LEFT JOIN task_weeks tw ON tw.task_id = t.id
WHERE t.subproject_id = CAST(sqlc.arg(subproject_id) AS INTEGER);

-- name: ListSubprojects :many
SELECT id, project_id, name, total_hours
FROM subprojects ORDER BY name COLLATE NOCASE, id;

-- name: ListSubprojectsWithTotals :many
SELECT
    s.id, s.project_id, s.name, s.total_hours,
    CAST(COALESCE(SUM(tw.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tw.spent_hours), 0) AS REAL) AS spent_hours
FROM subprojects s
LEFT JOIN tasks t ON t.subproject_id = s.id
LEFT JOIN task_weeks tw ON tw.task_id = t.id
GROUP BY s.id
ORDER BY s.name COLLATE NOCASE, s.id;

-- name: ListSubprojectsByProject :many
SELECT id, project_id, name, total_hours
FROM subprojects WHERE project_id = ? ORDER BY name COLLATE NOCASE, id;

-- name: ListSubprojectsByProjectWithTotals :many
SELECT
    s.id, s.project_id, s.name, s.total_hours,
    CAST(COALESCE(SUM(tw.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tw.spent_hours), 0) AS REAL) AS spent_hours
FROM subprojects s
LEFT JOIN tasks t ON t.subproject_id = s.id
LEFT JOIN task_weeks tw ON tw.task_id = t.id
WHERE s.project_id = ?
GROUP BY s.id
ORDER BY s.name COLLATE NOCASE, s.id;

-- name: SumSubprojectHoursByProject :one
SELECT CAST(COALESCE(SUM(total_hours), 0) AS REAL) FROM subprojects WHERE project_id = ?;

-- name: SumSubprojectHoursByProjectExcept :one
SELECT CAST(COALESCE(SUM(total_hours), 0) AS REAL) FROM subprojects WHERE project_id = ? AND id != ?;

-- name: CountTasksBySubproject :one
SELECT COUNT(*) FROM tasks WHERE subproject_id = ?;

-- name: CountTaskWeeksBySubproject :one
SELECT COUNT(*)
FROM task_weeks tw
JOIN tasks t ON t.id = tw.task_id
WHERE t.subproject_id = ?;

-- name: UpdateSubproject :one
UPDATE subprojects SET
    project_id = ?,
    name = ?,
    total_hours = ?
WHERE id = ?
RETURNING id, project_id, name, total_hours;

-- name: DeleteSubproject :exec
DELETE FROM subprojects WHERE id = ?;

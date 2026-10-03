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

-- name: ListSubprojectsWithTotals :many
SELECT
    s.id, s.project_id, s.name, s.total_hours,
    CAST(COALESCE(SUM(tt.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tt.spent_hours), 0) AS REAL) AS spent_hours
FROM subprojects s
LEFT JOIN v_task_totals tt ON tt.subproject_id = s.id
WHERE CAST(sqlc.narg(project_id) AS INTEGER) IS NULL OR s.project_id = CAST(sqlc.narg(project_id) AS INTEGER)
GROUP BY s.id
ORDER BY s.name COLLATE NOCASE, s.id;

-- name: ListSubprojects :many
SELECT id, project_id, name, total_hours
FROM subprojects ORDER BY name COLLATE NOCASE, id;

-- name: ListSubprojectsByProject :many
SELECT id, project_id, name, total_hours
FROM subprojects WHERE project_id = ? ORDER BY name COLLATE NOCASE, id;

-- name: SubprojectHoursConflict :one
WITH cap AS (
    SELECT
        (
            SELECT COALESCE(SUM(subprojects.total_hours), 0)
            FROM subprojects
            WHERE subprojects.project_id = sqlc.arg(project_id)
              AND subprojects.id != sqlc.arg(except_id)
        ) + sqlc.arg(new_hours) AS used_plus_new,
        (SELECT total_hours FROM projects WHERE id = sqlc.arg(project_id)) AS budget
)
SELECT
    CASE
        WHEN budget IS NULL THEN 404
        WHEN used_plus_new > budget THEN 409
        ELSE 0
    END AS status,
    CAST(CASE
        WHEN budget IS NULL THEN 'project-not-found'
        WHEN used_plus_new > budget THEN 'subproject-hours-exceed-project'
        ELSE ''
    END AS TEXT) AS reason
FROM cap;

-- name: UpdateSubproject :one
UPDATE subprojects SET
    project_id = ?,
    name = ?,
    total_hours = ?
WHERE id = ?
RETURNING id, project_id, name, total_hours;

-- name: DeleteSubproject :execrows
DELETE FROM subprojects
WHERE id = ?;

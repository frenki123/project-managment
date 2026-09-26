-- name: CreateProject :one
INSERT INTO projects (
    name, purchase_order_name, total_hours, start_date, end_date
) VALUES (?, ?, ?, ?, ?)
RETURNING id, name, purchase_order_name, total_hours, start_date, end_date;

-- name: GetProject :one
SELECT id, name, purchase_order_name, total_hours, start_date, end_date
FROM projects WHERE id = ?;

-- name: GetProjectTotals :one
WITH task_totals AS (
    SELECT project_id, planned_hours, spent_hours, progress
    FROM v_task_totals
    WHERE project_id = CAST(sqlc.arg(project_id) AS INTEGER)
)
SELECT
    CAST(COALESCE(SUM(tt.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tt.spent_hours), 0) AS REAL) AS spent_hours,
    CAST(CASE WHEN p.total_hours > 0
        THEN COALESCE(SUM(tt.planned_hours * tt.progress / p.total_hours), 0)
        ELSE 0 END AS REAL) AS progress
FROM projects p
LEFT JOIN task_totals tt ON tt.project_id = p.id
WHERE p.id = CAST(sqlc.arg(project_id) AS INTEGER)
GROUP BY p.id, p.total_hours;

-- name: ListProjects :many
SELECT id, name, purchase_order_name, total_hours, start_date, end_date
FROM projects ORDER BY name COLLATE NOCASE, id;

-- name: ListProjectsWithTotals :many
WITH task_totals AS (
    SELECT project_id, planned_hours, spent_hours, progress
    FROM v_task_totals
    WHERE project_id IS NOT NULL
)
SELECT
    p.id, p.name, p.purchase_order_name, p.total_hours, p.start_date, p.end_date,
    CAST(COALESCE(SUM(tt.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tt.spent_hours), 0) AS REAL) AS spent_hours,
    CAST(CASE WHEN p.total_hours > 0
        THEN COALESCE(SUM(tt.planned_hours * tt.progress / p.total_hours), 0)
        ELSE 0 END AS REAL) AS progress
FROM projects p
LEFT JOIN task_totals tt ON tt.project_id = p.id
GROUP BY p.id
ORDER BY p.name COLLATE NOCASE, p.id;

-- name: UpdateProject :one
UPDATE projects SET
    name = ?,
    purchase_order_name = ?,
    total_hours = ?,
    start_date = ?,
    end_date = ?
WHERE id = ?
RETURNING id, name, purchase_order_name, total_hours, start_date, end_date;

-- name: DeleteProject :one
DELETE FROM projects
WHERE projects.id = ?
  AND NOT EXISTS (SELECT 1 FROM tasks WHERE project_id = projects.id)
  AND NOT EXISTS (SELECT 1 FROM subprojects WHERE project_id = projects.id)
RETURNING projects.id;

-- name: CountTasksByProject :one
SELECT COUNT(*) FROM tasks WHERE project_id = CAST(? AS INTEGER);

-- name: CountSubprojectsByProject :one
SELECT COUNT(*) FROM subprojects WHERE project_id = CAST(? AS INTEGER);

-- name: CountTaskWeeksOutsideRange :one
SELECT COUNT(*) FROM task_weeks tw
JOIN tasks t ON t.id = tw.task_id
WHERE t.project_id = CAST(sqlc.arg(project_id) AS INTEGER)
  AND (tw.week_start < sqlc.arg(first_week) OR tw.week_start > sqlc.arg(last_week));

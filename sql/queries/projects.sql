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
    SELECT
        t.id,
        COALESCE(SUM(tw.planned_hours), 0) AS planned_hours,
        COALESCE(SUM(tw.spent_hours), 0) AS spent_hours,
        COALESCE(MAX(tw.progress), 0) AS progress
    FROM tasks t
    LEFT JOIN task_weeks tw ON tw.task_id = t.id
    WHERE t.project_id = CAST(sqlc.arg(project_id) AS INTEGER)
    GROUP BY t.id
)
SELECT
    CAST(COALESCE(SUM(tt.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tt.spent_hours), 0) AS REAL) AS spent_hours,
    CAST(CASE WHEN p.total_hours > 0
        THEN COALESCE(SUM(tt.planned_hours * tt.progress / p.total_hours), 0)
        ELSE 0 END AS REAL) AS progress
FROM projects p
LEFT JOIN task_totals tt ON TRUE
WHERE p.id = CAST(sqlc.arg(project_id) AS INTEGER)
GROUP BY p.total_hours;

-- name: ListProjects :many
SELECT id, name, purchase_order_name, total_hours, start_date, end_date
FROM projects ORDER BY name COLLATE NOCASE, id;

-- name: ListProjectsWithTotals :many
WITH task_totals AS (
    SELECT
        t.project_id,
        t.id,
        COALESCE(SUM(tw.planned_hours), 0) AS planned_hours,
        COALESCE(SUM(tw.spent_hours), 0) AS spent_hours,
        COALESCE(MAX(tw.progress), 0) AS progress
    FROM tasks t
    LEFT JOIN task_weeks tw ON tw.task_id = t.id
    WHERE t.project_id IS NOT NULL
    GROUP BY t.project_id, t.id
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

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = ?;

-- name: CountTaskWeeksByProject :one
SELECT COUNT(*)
FROM task_weeks tw
JOIN tasks t ON t.id = tw.task_id
WHERE t.project_id = ?;

-- name: CountTaskWeeksOutsideRange :one
SELECT COUNT(*) FROM task_weeks tw
JOIN tasks t ON t.id = tw.task_id
WHERE t.project_id = CAST(sqlc.arg(project_id) AS INTEGER)
  AND (tw.week_start < sqlc.arg(first_week) OR tw.week_start > sqlc.arg(last_week));

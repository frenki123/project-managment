-- name: CreateProject :one
INSERT INTO projects (
    name, purchase_order_name, total_hours, start_date, end_date
) VALUES (?, ?, ?, ?, ?)
RETURNING id, name, purchase_order_name, total_hours, start_date, end_date;

-- name: GetProject :one
SELECT id, name, purchase_order_name, total_hours, start_date, end_date
FROM projects WHERE id = ?;

-- name: ListProjects :many
SELECT id, name, purchase_order_name, total_hours, start_date, end_date
FROM projects ORDER BY name COLLATE NOCASE, id;

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

-- name: ClearTaskWeeksByProject :exec
DELETE FROM task_weeks
WHERE task_id IN (SELECT id FROM tasks WHERE project_id = CAST(? AS INTEGER));

-- name: CountTaskWeeksOutsideRange :one
SELECT COUNT(*) FROM task_weeks tw
JOIN tasks t ON t.id = tw.task_id
WHERE t.project_id = CAST(sqlc.arg(project_id) AS INTEGER)
  AND (tw.week_start < sqlc.arg(first_week) OR tw.week_start > sqlc.arg(last_week));

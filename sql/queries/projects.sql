-- name: CreateProject :one
INSERT INTO projects (
    name, purchase_order_name, total_hours, start_date, end_date
) VALUES (?, ?, ?, ?, ?)
RETURNING id, name, purchase_order_name, total_hours, start_date, end_date;

-- name: GetProject :one
SELECT id, name, purchase_order_name, total_hours, start_date, end_date
FROM projects WHERE id = ?;

-- name: GetProjectTotals :one
SELECT
planned_hours, spent_hours, progress
FROM v_project_totals
WHERE id = CAST(sqlc.arg(project_id) AS INTEGER);

-- name: ListProjects :many
SELECT id, name, purchase_order_name, total_hours, start_date, end_date
FROM projects ORDER BY name COLLATE NOCASE, id;

-- name: ListProjectsWithTotals :many
SELECT
id, name, purchase_order_name, total_hours, start_date, end_date,
planned_hours, spent_hours, progress
FROM v_project_totals
ORDER BY name COLLATE NOCASE, id;

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
WHERE id = ?
RETURNING id;

-- name: ProjectUpdateConflict :one
WITH bounds AS (
    SELECT
        date(sqlc.arg(start_date), '-' || ((strftime('%w', sqlc.arg(start_date)) + 6) % 7) || ' days') AS first_week,
        date(sqlc.arg(end_date), '-' || ((strftime('%w', sqlc.arg(end_date)) + 6) % 7) || ' days') AS last_week
), checks AS (
    SELECT
        sqlc.arg(total_hours) < (
            SELECT COALESCE(SUM(total_hours), 0)
            FROM subprojects
            WHERE subprojects.project_id = sqlc.arg(project_id)
        ) AS hours_below,
        EXISTS (
            SELECT 1
            FROM task_weeks tw
            JOIN tasks t ON t.id = tw.task_id
            WHERE t.project_id = sqlc.arg(project_id)
              AND (tw.week_start < (SELECT first_week FROM bounds)
                   OR tw.week_start > (SELECT last_week FROM bounds))
        ) AS weeks_outside
)
SELECT
    CASE WHEN hours_below OR weeks_outside THEN 409 ELSE 0 END AS status,
    CAST(CASE
        WHEN hours_below THEN 'project hours cannot be less than subproject hours'
        WHEN weeks_outside THEN 'project dates cannot exclude existing weekly data'
        ELSE ''
    END AS TEXT) AS reason
FROM checks;

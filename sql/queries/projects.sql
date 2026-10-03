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
planned_hours, spent_hours, progress, CAST(earned_hours AS REAL) AS earned_hours
FROM v_project_totals
WHERE id = CAST(sqlc.arg(project_id) AS INTEGER);

-- name: ListProjects :many
SELECT id, name, purchase_order_name, total_hours, start_date, end_date
FROM projects ORDER BY name COLLATE NOCASE, id;

-- name: ListProjectsWithTotals :many
SELECT
id, name, purchase_order_name, total_hours, start_date, end_date,
planned_hours, spent_hours, progress, CAST(earned_hours AS REAL) AS earned_hours
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

-- name: DeleteProject :execrows
DELETE FROM projects
WHERE id = ?;

-- name: ProjectNameConflict :one
-- 409 when another project already has this name (case-insensitive)
SELECT
    CASE WHEN COUNT(*) > 0 THEN 409 ELSE 0 END AS status,
    CAST(CASE WHEN COUNT(*) > 0 THEN 'project-name-taken' ELSE '' END AS TEXT) AS reason
FROM projects
WHERE name = CAST(sqlc.arg(name) AS TEXT) COLLATE NOCASE
  AND id != CAST(sqlc.arg(except_id) AS INTEGER);

-- name: ProjectUpdateConflict :one
WITH bounds AS (
    SELECT
        date(CAST(sqlc.arg(start_date) AS TEXT), '-' || ((strftime('%w', CAST(sqlc.arg(start_date) AS TEXT)) + 6) % 7) || ' days') AS first_week,
        date(CAST(sqlc.arg(end_date) AS TEXT), '-' || ((strftime('%w', CAST(sqlc.arg(end_date) AS TEXT)) + 6) % 7) || ' days') AS last_week
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
        WHEN hours_below THEN 'project-hours-below-subprojects'
        WHEN weeks_outside THEN 'project-dates-exclude-weekly-data'
        ELSE ''
    END AS TEXT) AS reason
FROM checks;

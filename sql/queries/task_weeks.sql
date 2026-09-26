-- name: GetTaskWeek :one
SELECT task_id, week_start, planned_hours, spent_hours, progress
FROM task_weeks WHERE task_id = ? AND week_start = ?;

-- name: ListTaskWeeksByTask :many
SELECT task_id, week_start, planned_hours, spent_hours, progress
FROM task_weeks WHERE task_id = ? ORDER BY week_start;

-- name: GetTaskTotals :one
SELECT
    CAST(COALESCE(SUM(planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(spent_hours), 0) AS REAL) AS spent_hours,
    CAST(COALESCE(MAX(progress), 0) AS REAL) AS progress
FROM task_weeks
WHERE task_id = ?;

-- name: CountTaskWeeksByTask :one
SELECT COUNT(*) FROM task_weeks WHERE task_id = ?;

-- name: ListTaskTotalsByProject :many
SELECT
    t.id AS task_id,
    CAST(COALESCE(SUM(tw.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tw.spent_hours), 0) AS REAL) AS spent_hours,
    CAST(COALESCE(MAX(tw.progress), 0) AS REAL) AS progress
FROM tasks t
LEFT JOIN task_weeks tw ON tw.task_id = t.id
WHERE t.project_id = CAST(sqlc.arg(project_id) AS INTEGER)
GROUP BY t.id;

-- name: ListTaskTotalsBySubproject :many
SELECT
    t.id AS task_id,
    CAST(COALESCE(SUM(tw.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tw.spent_hours), 0) AS REAL) AS spent_hours,
    CAST(COALESCE(MAX(tw.progress), 0) AS REAL) AS progress
FROM tasks t
LEFT JOIN task_weeks tw ON tw.task_id = t.id
WHERE t.subproject_id = CAST(sqlc.arg(subproject_id) AS INTEGER)
GROUP BY t.id;

-- name: ListTaskWeeksByProject :many
SELECT tw.*
FROM task_weeks tw
JOIN tasks t ON t.id = tw.task_id
WHERE t.project_id = ?
ORDER BY tw.task_id, tw.week_start;

-- name: ListTaskWeeksBySubproject :many
SELECT tw.*
FROM task_weeks tw
JOIN tasks t ON t.id = tw.task_id
WHERE t.subproject_id = ?
ORDER BY tw.task_id, tw.week_start;

-- name: GetLastProgressBefore :one
SELECT progress FROM task_weeks
WHERE task_id = ? AND week_start < ? AND progress IS NOT NULL
ORDER BY week_start DESC
LIMIT 1;

-- name: ListTaskWeeksAfter :many
SELECT task_id, week_start, planned_hours, spent_hours, progress FROM task_weeks
WHERE task_id = ? AND week_start > ?
ORDER BY week_start;

-- name: UpsertTaskWeek :one
INSERT INTO task_weeks (task_id, week_start, planned_hours, spent_hours, progress)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (task_id, week_start) DO UPDATE SET
    planned_hours = excluded.planned_hours,
    spent_hours = excluded.spent_hours,
    progress = excluded.progress
RETURNING task_id, week_start, planned_hours, spent_hours, progress;

-- name: UpdateTaskWeekProgress :exec
UPDATE task_weeks SET progress = ?
WHERE task_id = ? AND week_start = ?;

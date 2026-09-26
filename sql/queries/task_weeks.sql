-- name: GetTaskWeek :one
SELECT task_id, week_start, planned_hours, spent_hours, progress
FROM task_weeks WHERE task_id = ? AND week_start = ?;

-- name: ListTaskWeeksByTask :many
SELECT task_id, week_start, planned_hours, spent_hours, progress
FROM task_weeks WHERE task_id = ? ORDER BY week_start;

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

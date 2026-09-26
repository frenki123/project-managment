-- name: GetTaskWeek :one
SELECT task_id, week_start, planned_hours, spent_hours, progress
FROM task_weeks WHERE task_id = ? AND week_start = ?;

-- name: ListTaskWeeksByTask :many
SELECT task_id, week_start, planned_hours, spent_hours, progress
FROM task_weeks WHERE task_id = ? ORDER BY week_start;

-- name: GetTaskTotals :one
SELECT CAST(planned_hours AS REAL) AS planned_hours,
       CAST(spent_hours AS REAL) AS spent_hours,
       CAST(progress AS REAL) AS progress
FROM v_task_totals
WHERE task_id = ?;

-- name: CountTaskWeeksByTask :one
SELECT COUNT(*) FROM task_weeks WHERE task_id = ?;

-- name: ListTaskTotalsByProject :many
SELECT
    t.id AS task_id,
    CAST(tt.planned_hours AS REAL) AS planned_hours,
    CAST(tt.spent_hours AS REAL) AS spent_hours,
    CAST(tt.progress AS REAL) AS progress
FROM v_task_totals tt
JOIN tasks t ON t.id = tt.task_id
WHERE t.project_id = CAST(sqlc.arg(project_id) AS INTEGER)
;

-- name: ListTaskWeeksByProject :many
SELECT tw.task_id, tw.week_start, tw.planned_hours, tw.spent_hours, tw.progress
FROM task_weeks tw
JOIN tasks t ON t.id = tw.task_id
WHERE t.project_id = ?
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

-- name: UpdateTaskWeeksProgressAfter :exec
UPDATE task_weeks
SET progress = ?
WHERE task_id = ?
  AND week_start > ?
  AND progress IS NOT NULL
  AND progress < ?;

-- name: ListProjectWeekTotals :many
WITH RECURSIVE bounds AS (
    SELECT start_date, end_date
    FROM projects
    WHERE id = CAST(sqlc.arg(project_id) AS INTEGER)
), weeks AS (
    SELECT
        date(start_date, '-' || ((CAST(strftime('%w', start_date) AS INTEGER) + 6) % 7) || ' days') AS week_start,
        date(end_date, '-' || ((CAST(strftime('%w', end_date) AS INTEGER) + 6) % 7) || ' days') AS last_week
    FROM bounds
    UNION ALL
    SELECT date(week_start, '+7 days'), last_week
    FROM weeks
    WHERE week_start < last_week
), scope AS (
    SELECT t.id, tt.planned_hours AS complexity
    FROM tasks t
    JOIN v_task_totals tt ON tt.task_id = t.id
    WHERE t.project_id = CAST(sqlc.arg(project_id) AS INTEGER)
), weekly AS (
    SELECT
        w.week_start,
        COALESCE(SUM(tw.planned_hours), 0) AS planned_hours,
        COALESCE(SUM(tw.spent_hours), 0) AS spent_hours,
        COALESCE(SUM(scope.complexity * COALESCE((
            SELECT MAX(p.progress)
            FROM task_weeks p
            WHERE p.task_id = scope.id AND p.week_start <= w.week_start
        ), 0) / 100.0), 0) AS earned_hours
    FROM weeks w
    LEFT JOIN scope ON TRUE
    LEFT JOIN task_weeks tw ON tw.task_id = scope.id AND tw.week_start = w.week_start
    GROUP BY 1
)
SELECT
    CAST(week_start AS TEXT) AS week_start,
    CAST(planned_hours AS REAL) AS planned_hours,
    CAST(spent_hours AS REAL) AS spent_hours,
    CAST(earned_hours AS REAL) AS earned_hours,
    CAST(SUM(planned_hours) OVER (ORDER BY week_start ROWS UNBOUNDED PRECEDING) AS REAL) AS cumulative_planned_hours,
    CAST(SUM(spent_hours) OVER (ORDER BY week_start ROWS UNBOUNDED PRECEDING) AS REAL) AS cumulative_spent_hours
FROM weekly
ORDER BY week_start;

-- name: ListSubprojectWeekTotals :many
WITH RECURSIVE bounds AS (
    SELECT p.start_date, p.end_date
    FROM projects p
    JOIN subprojects s ON s.project_id = p.id
    WHERE s.id = CAST(sqlc.arg(subproject_id) AS INTEGER)
), weeks AS (
    SELECT
        date(start_date, '-' || ((CAST(strftime('%w', start_date) AS INTEGER) + 6) % 7) || ' days') AS week_start,
        date(end_date, '-' || ((CAST(strftime('%w', end_date) AS INTEGER) + 6) % 7) || ' days') AS last_week
    FROM bounds
    UNION ALL
    SELECT date(week_start, '+7 days'), last_week
    FROM weeks
    WHERE week_start < last_week
), scope AS (
    SELECT t.id, tt.planned_hours AS complexity
    FROM tasks t
    JOIN v_task_totals tt ON tt.task_id = t.id
    WHERE t.subproject_id = CAST(sqlc.arg(subproject_id) AS INTEGER)
), weekly AS (
    SELECT
        w.week_start,
        COALESCE(SUM(tw.planned_hours), 0) AS planned_hours,
        COALESCE(SUM(tw.spent_hours), 0) AS spent_hours,
        COALESCE(SUM(scope.complexity * COALESCE((
            SELECT MAX(p.progress)
            FROM task_weeks p
            WHERE p.task_id = scope.id AND p.week_start <= w.week_start
        ), 0) / 100.0), 0) AS earned_hours
    FROM weeks w
    LEFT JOIN scope ON TRUE
    LEFT JOIN task_weeks tw ON tw.task_id = scope.id AND tw.week_start = w.week_start
    GROUP BY 1
)
SELECT
    CAST(week_start AS TEXT) AS week_start,
    CAST(planned_hours AS REAL) AS planned_hours,
    CAST(spent_hours AS REAL) AS spent_hours,
    CAST(earned_hours AS REAL) AS earned_hours,
    CAST(SUM(planned_hours) OVER (ORDER BY week_start ROWS UNBOUNDED PRECEDING) AS REAL) AS cumulative_planned_hours,
    CAST(SUM(spent_hours) OVER (ORDER BY week_start ROWS UNBOUNDED PRECEDING) AS REAL) AS cumulative_spent_hours
FROM weekly
ORDER BY week_start;

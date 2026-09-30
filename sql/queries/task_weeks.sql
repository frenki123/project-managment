-- name: GetTaskWeek :one
SELECT task_id, week_start, planned_hours, spent_hours, progress
FROM task_weeks WHERE task_id = ? AND week_start = ?;

-- name: ListTaskWeeksByTask :many
SELECT task_id, week_start, planned_hours, spent_hours, progress,
       CAST(effective_progress AS REAL) AS effective_progress
FROM v_task_week_effective WHERE task_id = ? ORDER BY week_start;

-- name: GetTaskTotals :one
SELECT CAST(planned_hours AS REAL) AS planned_hours,
       CAST(spent_hours AS REAL) AS spent_hours,
       CAST(progress AS REAL) AS progress
FROM v_task_totals
WHERE task_id = ?;

-- name: HasTaskWeeks :one
SELECT EXISTS (SELECT 1 FROM task_weeks WHERE task_id = ?);

-- name: ListTaskTotals :many
SELECT t.id AS task_id,
       CAST(COALESCE(SUM(tw.planned_hours), 0) AS REAL) AS planned_hours,
       CAST(COALESCE(SUM(tw.spent_hours), 0) AS REAL) AS spent_hours,
       CAST(COALESCE(MAX(tw.progress), 0) AS REAL) AS progress
FROM tasks t
LEFT JOIN task_weeks tw ON tw.task_id = t.id
WHERE CAST(sqlc.arg(scope) AS TEXT) = 'all'
   OR (CAST(sqlc.arg(scope) AS TEXT) = 'ideas' AND t.project_id IS NULL)
   OR (CAST(sqlc.arg(scope) AS TEXT) = 'project' AND t.project_id = CAST(sqlc.arg(owner_id) AS INTEGER))
   OR (CAST(sqlc.arg(scope) AS TEXT) = 'subproject' AND t.subproject_id = CAST(sqlc.arg(owner_id) AS INTEGER))
GROUP BY t.id;

-- name: ListTaskWeeksByProject :many
SELECT tw.*
FROM v_task_week_effective tw
JOIN tasks t ON t.id = tw.task_id
WHERE t.project_id = ?
ORDER BY tw.task_id, tw.week_start;

-- name: TaskAssignmentConflict :one
SELECT
    CASE
        WHEN sqlc.narg(subproject_id) IS NOT NULL
             AND NOT EXISTS (SELECT 1 FROM subprojects WHERE id = sqlc.narg(subproject_id)) THEN 404
        WHEN sqlc.narg(project_id) IS NOT NULL
             AND NOT EXISTS (SELECT 1 FROM projects WHERE id = sqlc.narg(project_id)) THEN 404
        WHEN sqlc.narg(subproject_id) IS NOT NULL
             AND sqlc.narg(project_id) IS NOT NULL
             AND (SELECT s.project_id FROM subprojects s WHERE s.id = sqlc.narg(subproject_id))
                 IS NOT sqlc.narg(project_id) THEN 400
        ELSE 0
    END AS status,
    CAST(CASE
        WHEN sqlc.narg(subproject_id) IS NOT NULL
             AND NOT EXISTS (SELECT 1 FROM subprojects WHERE id = sqlc.narg(subproject_id))
            THEN 'subproject not found'
        WHEN sqlc.narg(project_id) IS NOT NULL
             AND NOT EXISTS (SELECT 1 FROM projects WHERE id = sqlc.narg(project_id))
            THEN 'project not found'
        WHEN sqlc.narg(subproject_id) IS NOT NULL
             AND sqlc.narg(project_id) IS NOT NULL
             AND (SELECT s.project_id FROM subprojects s WHERE s.id = sqlc.narg(subproject_id))
                 IS NOT sqlc.narg(project_id)
            THEN 'subproject does not belong to project'
        ELSE ''
    END AS TEXT) AS reason;

-- name: WeekWriteContext :one
WITH input AS (
    SELECT sqlc.arg(week_start) AS week_start, sqlc.arg(task_id) AS task_id
)
SELECT
    t.project_id IS NULL AS is_idea,
    CASE WHEN input.week_start BETWEEN p.first_week AND p.last_week THEN 1 ELSE 0 END AS in_range,
    CAST(COALESCE((
        SELECT MAX(w2.progress)
        FROM task_weeks w2
        WHERE w2.task_id = t.id
          AND w2.week_start < input.week_start
    ), 0) AS REAL) AS previous_progress,
    COALESCE(tw.planned_hours, 0) AS planned_hours,
    COALESCE(tw.spent_hours, 0) AS spent_hours,
    tw.progress
FROM input
JOIN tasks t ON t.id = input.task_id
LEFT JOIN v_project_bounds p ON p.id = t.project_id
LEFT JOIN task_weeks tw
    ON tw.task_id = t.id
   AND tw.week_start = input.week_start
WHERE t.id = input.task_id;

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
    SELECT first_week, last_week
    FROM v_project_bounds
    WHERE id = CAST(sqlc.arg(project_id) AS INTEGER)
), weeks AS (
    SELECT
        first_week AS week_start,
        last_week
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
                SELECT MAX(p.effective_progress)
                FROM v_task_week_effective p
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
    SELECT p.first_week, p.last_week
    FROM v_project_bounds p
    JOIN subprojects s ON s.project_id = p.id
    WHERE s.id = CAST(sqlc.arg(subproject_id) AS INTEGER)
), weeks AS (
    SELECT
        first_week AS week_start,
        last_week
    FROM bounds
    UNION ALL
    SELECT date(week_start, '+7 days'), last_week
    FROM weeks
    WHERE week_start < last_week
), scope AS (
    SELECT t.id
    FROM tasks t
    WHERE t.subproject_id = CAST(sqlc.arg(subproject_id) AS INTEGER)
), weekly AS (
    SELECT
        w.week_start,
        COALESCE(SUM(tw.planned_hours), 0) AS planned_hours,
        COALESCE(SUM(tw.spent_hours), 0) AS spent_hours
    FROM weeks w
    LEFT JOIN scope ON TRUE
    LEFT JOIN task_weeks tw ON tw.task_id = scope.id AND tw.week_start = w.week_start
    GROUP BY 1
)
SELECT
    CAST(week_start AS TEXT) AS week_start,
    CAST(planned_hours AS REAL) AS planned_hours,
    CAST(spent_hours AS REAL) AS spent_hours
FROM weekly
ORDER BY week_start;

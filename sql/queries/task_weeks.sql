-- name: GetTaskWeek :one
SELECT task_id, week_start, planned_hours, spent_hours, progress
FROM task_weeks WHERE task_id = ? AND week_start = ?;

-- name: GetTaskTotals :one
SELECT CAST(planned_hours AS REAL) AS planned_hours,
       CAST(spent_hours AS REAL) AS spent_hours,
       CAST(progress AS REAL) AS progress
FROM v_task_totals
WHERE task_id = ?;

-- name: TaskReassignConflict :one
SELECT
    CASE WHEN EXISTS (SELECT 1 FROM task_weeks WHERE task_id = CAST(sqlc.arg(task_id) AS INTEGER)) THEN 409 ELSE 0 END AS status,
    CAST(CASE WHEN EXISTS (SELECT 1 FROM task_weeks WHERE task_id = CAST(sqlc.arg(task_id) AS INTEGER)) THEN 'task-has-weekly-data' ELSE '' END AS TEXT) AS reason;

-- name: ListTaskTotalsIdeas :many
SELECT tt.* FROM v_task_totals tt WHERE tt.project_id IS NULL;

-- name: ListTaskTotalsScoped :many
SELECT tt.* FROM v_task_totals tt
WHERE (CAST(sqlc.narg(project_id) AS INTEGER) IS NULL OR tt.project_id = CAST(sqlc.narg(project_id) AS INTEGER))
  AND (CAST(sqlc.narg(subproject_id) AS INTEGER) IS NULL OR tt.subproject_id = CAST(sqlc.narg(subproject_id) AS INTEGER));

-- name: ListTaskWeekSeriesByProject :many
SELECT ts.* FROM v_task_week_series ts WHERE ts.task_id IN (SELECT id FROM tasks WHERE project_id = CAST(sqlc.arg(project_id) AS INTEGER)) ORDER BY ts.task_id, ts.week_start;

-- name: ListTaskWeekSeriesBySubproject :many
SELECT ts.* FROM v_task_week_series ts WHERE ts.task_id IN (SELECT id FROM tasks WHERE subproject_id = CAST(sqlc.arg(subproject_id) AS INTEGER)) ORDER BY ts.task_id, ts.week_start;

-- name: ListTaskWeekSeriesByTask :many
SELECT ts.* FROM v_task_week_series ts WHERE ts.task_id = ? ORDER BY ts.week_start;

-- name: TaskAssignmentConflict :one
WITH refs AS (
    SELECT
        CAST(sqlc.narg(subproject_id) AS INTEGER) AS subproject_id,
        CAST(sqlc.narg(project_id) AS INTEGER) AS project_id,
        EXISTS (SELECT 1 FROM subprojects WHERE id = CAST(sqlc.narg(subproject_id) AS INTEGER)) AS subproject_exists,
        EXISTS (SELECT 1 FROM projects WHERE id = CAST(sqlc.narg(project_id) AS INTEGER)) AS project_exists,
        (SELECT s.project_id FROM subprojects s WHERE s.id = CAST(sqlc.narg(subproject_id) AS INTEGER)) AS subproject_project_id
)
SELECT
    CASE
        WHEN refs.subproject_id IS NOT NULL AND NOT refs.subproject_exists THEN 404
        WHEN refs.project_id IS NOT NULL AND NOT refs.project_exists THEN 404
        WHEN refs.subproject_id IS NOT NULL AND refs.project_id IS NOT NULL
             AND refs.subproject_project_id IS NOT refs.project_id THEN 400
        ELSE 0
    END AS status,
    CAST(CASE
        WHEN refs.subproject_id IS NOT NULL AND NOT refs.subproject_exists THEN 'subproject-not-found'
        WHEN refs.project_id IS NOT NULL AND NOT refs.project_exists THEN 'project-not-found'
        WHEN refs.subproject_id IS NOT NULL AND refs.project_id IS NOT NULL
             AND refs.subproject_project_id IS NOT refs.project_id THEN 'subproject-project-mismatch'
        ELSE ''
    END AS TEXT) AS reason
FROM refs;

-- name: WeekWriteContext :one
WITH input AS (
    SELECT sqlc.arg(week_start) AS week_start, sqlc.arg(task_id) AS task_id
), context AS (
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
WHERE t.id = input.task_id
), result AS (
SELECT
    CASE
        WHEN is_idea THEN 400
        WHEN in_range = 0 THEN 400
        ELSE 0
    END AS status,
    CASE
        WHEN is_idea THEN 'idea-task-not-assignable'
        WHEN in_range = 0 THEN 'week-outside-project-bounds'
        ELSE ''
    END AS reason,
    planned_hours, spent_hours, progress, previous_progress
FROM context
)
SELECT status, CAST(reason AS TEXT) AS reason, planned_hours, spent_hours, progress, previous_progress
FROM result;

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
SET progress = NULL
WHERE task_id = sqlc.arg(task_id)
  AND week_start > sqlc.arg(week_start)
  AND progress IS NOT NULL
  AND progress < sqlc.arg(progress);

-- name: ListProjectWeekTotals :many
WITH weekly AS (
    SELECT ts.week_start,
           SUM(ts.planned_hours) AS planned_hours,
           SUM(ts.spent_hours) AS spent_hours,
           SUM(tt.planned_hours * ts.effective_progress / 100.0) AS earned_hours
    FROM v_task_week_series ts
    JOIN v_task_totals tt ON tt.task_id = ts.task_id
    WHERE ts.task_id IN (SELECT id FROM tasks WHERE project_id = CAST(sqlc.arg(project_id) AS INTEGER))
    GROUP BY ts.week_start
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
WITH weekly AS (
    SELECT ts.week_start,
           SUM(ts.planned_hours) AS planned_hours,
           SUM(ts.spent_hours) AS spent_hours
    FROM v_task_week_series ts
    WHERE ts.task_id IN (SELECT id FROM tasks WHERE subproject_id = CAST(sqlc.arg(subproject_id) AS INTEGER))
    GROUP BY ts.week_start
)
SELECT
    CAST(week_start AS TEXT) AS week_start,
    CAST(planned_hours AS REAL) AS planned_hours,
    CAST(spent_hours AS REAL) AS spent_hours
FROM weekly
ORDER BY week_start;

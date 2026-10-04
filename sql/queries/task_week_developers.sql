-- name: ListTaskWeekDevelopers :many
SELECT twd.task_id, twd.week_start, twd.person_id, p.name AS person_name, twd.planned_hours, twd.spent_hours
FROM task_week_developers twd JOIN people p ON p.id = twd.person_id
WHERE twd.task_id = ? AND twd.week_start = ?
ORDER BY twd.person_id;

-- name: ListTaskWeekDevelopersByTask :many
SELECT twd.task_id, twd.week_start, twd.person_id, p.name AS person_name, twd.planned_hours, twd.spent_hours
FROM task_week_developers twd JOIN people p ON p.id = twd.person_id
WHERE twd.task_id = ?
ORDER BY twd.week_start, twd.person_id;

-- name: UpsertTaskWeekDeveloper :exec
INSERT INTO task_week_developers (task_id, week_start, person_id, planned_hours, spent_hours)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (task_id, week_start, person_id) DO UPDATE SET
    planned_hours = excluded.planned_hours,
    spent_hours = excluded.spent_hours;

-- name: DeleteTaskWeekDeveloper :execrows
DELETE FROM task_week_developers
WHERE task_id = ? AND week_start = ? AND person_id = ?;

-- name: FirstTaskDeveloper :one
SELECT person_id FROM task_developers WHERE task_id = ? ORDER BY person_id LIMIT 1;

-- name: PersonOnTask :one
SELECT
    CASE
        WHEN NOT EXISTS (SELECT 1 FROM people WHERE id = CAST(sqlc.arg(person_id) AS INTEGER)) THEN 404
        WHEN NOT EXISTS (SELECT 1 FROM task_developers WHERE task_id = CAST(sqlc.arg(task_id) AS INTEGER) AND person_id = CAST(sqlc.arg(person_id) AS INTEGER)) THEN 400
        ELSE 0
    END AS status,
    CAST(CASE
        WHEN NOT EXISTS (SELECT 1 FROM people WHERE id = CAST(sqlc.arg(person_id) AS INTEGER)) THEN 'person-not-found'
        WHEN NOT EXISTS (SELECT 1 FROM task_developers WHERE task_id = CAST(sqlc.arg(task_id) AS INTEGER) AND person_id = CAST(sqlc.arg(person_id) AS INTEGER)) THEN 'person-not-on-task'
        ELSE ''
    END AS TEXT) AS reason;
-- name: ListTaskDevelopers :many
SELECT td.person_id AS id, p.name
FROM task_developers td
JOIN people p ON p.id = td.person_id
WHERE td.task_id = ?
ORDER BY p.name COLLATE NOCASE, td.person_id;

-- name: ListAllTaskDevelopers :many
SELECT td.task_id, td.person_id AS id, p.name
FROM task_developers td
JOIN people p ON p.id = td.person_id
ORDER BY td.task_id, p.name COLLATE NOCASE;

-- name: ReplaceTaskDevelopers :exec
DELETE FROM task_developers WHERE task_id = ?;

-- name: DeleteTaskDevelopers :exec
DELETE FROM task_developers WHERE task_id = ?;

-- name: AddTaskDeveloper :execrows
INSERT INTO task_developers (task_id, person_id) VALUES (?, ?);

-- name: PersonExists :one
SELECT CAST(COUNT(*) > 0 AS INTEGER) AS present FROM people WHERE id = ?;
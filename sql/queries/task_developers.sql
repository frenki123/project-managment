-- name: ReplaceTaskDevelopers :exec
DELETE FROM task_developers WHERE task_id = ?;

-- name: AddTaskDeveloper :execrows
INSERT INTO task_developers (task_id, person_id) VALUES (?, ?);
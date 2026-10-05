-- name: ClearTaskDevelopers :exec
DELETE FROM task_developers WHERE task_id = ?;

-- name: AddTaskDeveloper :exec
INSERT INTO task_developers (task_id, person_id) VALUES (?, ?);

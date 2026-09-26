-- name: CreateTask :one
INSERT INTO tasks (
    name,
    description,
    implementation_notes,
    department,
    developers,
    priority,
    project_id,
    subproject_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id;

-- name: GetTask :one
SELECT id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id
FROM tasks WHERE id = ?;

-- name: ListTasks :many
SELECT id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id
FROM tasks ORDER BY name COLLATE NOCASE, id;

-- name: ListIdeaTasks :many
SELECT id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id
FROM tasks WHERE project_id IS NULL ORDER BY name COLLATE NOCASE, id;

-- name: ListTasksByProject :many
SELECT id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id
FROM tasks WHERE project_id = ? ORDER BY name COLLATE NOCASE, id;

-- name: ListTasksBySubproject :many
SELECT id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id
FROM tasks WHERE subproject_id = ? ORDER BY name COLLATE NOCASE, id;

-- name: UpdateTask :one
UPDATE tasks SET
    name = ?,
    description = ?,
    implementation_notes = ?,
    department = ?,
    developers = ?,
    priority = ?,
    project_id = ?,
    subproject_id = ?
WHERE id = ?
RETURNING id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE id = ?;

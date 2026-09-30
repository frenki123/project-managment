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
) VALUES (?, ?, ?, ?, ?, ?,
    COALESCE(sqlc.narg(project_id), (
        SELECT project_id FROM subprojects WHERE id = sqlc.narg(subproject_id)
    )),
    sqlc.narg(subproject_id))
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
    project_id = COALESCE(sqlc.narg(project_id), (
        SELECT project_id FROM subprojects WHERE id = sqlc.narg(subproject_id)
    )),
    subproject_id = sqlc.narg(subproject_id)
WHERE tasks.id = sqlc.arg(id)
RETURNING id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id;

-- name: DeleteTask :one
DELETE FROM tasks
WHERE id = ?
RETURNING id;

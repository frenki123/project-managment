-- name: CreateTask :one
INSERT INTO tasks (
    name,
    description,
    implementation_notes,
    department,
    developers,
    priority,
    project_id,
    subproject_id,
    manual_status
) VALUES (?, ?, ?, ?, ?, ?,
    COALESCE(CAST(sqlc.narg(project_id) AS INTEGER), (
        SELECT project_id FROM subprojects WHERE id = CAST(sqlc.narg(subproject_id) AS INTEGER)
    )),
    CAST(sqlc.narg(subproject_id) AS INTEGER),
    CAST(sqlc.narg(manual_status) AS TEXT))
RETURNING id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id, manual_status;

-- name: GetTask :one
SELECT id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id, manual_status
FROM tasks WHERE id = ?;

-- name: ListIdeaTasks :many
SELECT id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id, manual_status
FROM tasks WHERE project_id IS NULL ORDER BY name COLLATE NOCASE, id;

-- name: ListTasksScoped :many
SELECT id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id, manual_status
FROM tasks
WHERE (CAST(sqlc.narg(project_id) AS INTEGER) IS NULL OR project_id = CAST(sqlc.narg(project_id) AS INTEGER))
  AND (CAST(sqlc.narg(subproject_id) AS INTEGER) IS NULL OR subproject_id = CAST(sqlc.narg(subproject_id) AS INTEGER))
ORDER BY name COLLATE NOCASE, id;

-- name: TaskFilterScope :one
SELECT
    CAST(COALESCE((SELECT 1 FROM projects WHERE id = CAST(sqlc.narg(project_id) AS INTEGER)), 0) AS INTEGER) AS project_exists,
    CAST(COALESCE((SELECT 1 FROM subprojects WHERE id = CAST(sqlc.narg(subproject_id) AS INTEGER)), 0) AS INTEGER) AS subproject_exists,
    CAST(COALESCE((SELECT project_id FROM subprojects WHERE id = CAST(sqlc.narg(subproject_id) AS INTEGER)), 0) AS INTEGER) AS subproject_project_id;

-- name: UpdateTask :one
UPDATE tasks SET
    name = ?,
    description = ?,
    implementation_notes = ?,
    department = ?,
    developers = ?,
    priority = ?,
    manual_status = CAST(sqlc.narg(manual_status) AS TEXT),
    project_id = COALESCE(CAST(sqlc.narg(project_id) AS INTEGER), (
        SELECT project_id FROM subprojects WHERE id = CAST(sqlc.narg(subproject_id) AS INTEGER)
    )),
    subproject_id = CAST(sqlc.narg(subproject_id) AS INTEGER)
WHERE tasks.id = sqlc.arg(id)
RETURNING id, name, description, implementation_notes, department, developers, priority, project_id, subproject_id, manual_status;

-- name: DeleteTask :execrows
DELETE FROM tasks
WHERE id = ?;

-- name: ListStages :many
SELECT id, name, position, color, auto_reachable, progress_threshold
FROM stages ORDER BY position, id;

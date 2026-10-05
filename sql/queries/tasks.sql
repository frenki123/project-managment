-- name: CreateTask :one
INSERT INTO tasks (
    name,
    description,
    implementation_notes,
    department,
    priority,
    project_id,
    subproject_id
) VALUES (?, ?, ?, ?, ?,
    COALESCE(CAST(sqlc.narg(project_id) AS INTEGER), (
        SELECT project_id FROM subprojects WHERE id = CAST(sqlc.narg(subproject_id) AS INTEGER)
    )),
    CAST(sqlc.narg(subproject_id) AS INTEGER))
RETURNING id, name, description, implementation_notes, department, priority, project_id, subproject_id;

-- name: GetTask :one
SELECT
    t.id, t.name, t.description, t.implementation_notes, t.department, t.priority, t.project_id, t.subproject_id,
    CAST((SELECT json_group_array(json_object('id', p.id, 'name', p.name, 'weekly_capacity', p.weekly_capacity))
          FROM task_developers td JOIN people p ON p.id = td.person_id
          WHERE td.task_id = t.id) AS TEXT) AS developers
FROM tasks t WHERE t.id = ?;

-- name: ListIdeaTasks :many
SELECT id, name, description, implementation_notes, department, priority, project_id, subproject_id
FROM tasks WHERE project_id IS NULL ORDER BY name COLLATE NOCASE, id;

-- name: ListTasksScoped :many
SELECT id, name, description, implementation_notes, department, priority, project_id, subproject_id
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
    priority = ?,
    project_id = COALESCE(CAST(sqlc.narg(project_id) AS INTEGER), (
        SELECT project_id FROM subprojects WHERE id = CAST(sqlc.narg(subproject_id) AS INTEGER)
    )),
    subproject_id = CAST(sqlc.narg(subproject_id) AS INTEGER)
WHERE tasks.id = sqlc.arg(id)
RETURNING id, name, description, implementation_notes, department, priority, project_id, subproject_id;

-- name: DeleteTask :execrows
DELETE FROM tasks
WHERE id = ?;

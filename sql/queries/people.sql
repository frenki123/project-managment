-- name: CreatePerson :one
INSERT INTO people (name, weekly_capacity)
VALUES (?, COALESCE(CAST(sqlc.narg(weekly_capacity) AS REAL), 40))
RETURNING id, name, weekly_capacity;

-- name: GetPerson :one
SELECT id, name, weekly_capacity FROM people WHERE id = ?;

-- name: ListPeople :many
SELECT id, name, weekly_capacity
FROM people ORDER BY name COLLATE NOCASE, id;

-- name: UpdatePerson :one
UPDATE people SET
    name = ?,
    weekly_capacity = ?
WHERE id = ?
RETURNING id, name, weekly_capacity;

-- name: DeletePerson :execrows
DELETE FROM people
WHERE id = ?;

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

-- name: PersonNameConflict :one
-- 409 when another person already has this name (case-insensitive)
SELECT
    CASE WHEN COUNT(*) > 0 THEN 409 ELSE 0 END AS status,
    CAST(CASE WHEN COUNT(*) > 0 THEN 'person-name-taken' ELSE '' END AS TEXT) AS reason
FROM people
WHERE name = CAST(sqlc.arg(name) AS TEXT) COLLATE NOCASE
  AND id != CAST(sqlc.arg(except_id) AS INTEGER);

-- name: PersonOverrideWriteContext :one
-- 404 when the person does not exist
SELECT
    CASE WHEN COUNT(*) = 0 THEN 404 ELSE 0 END AS status,
    CAST(CASE WHEN COUNT(*) = 0 THEN 'person-not-found' ELSE '' END AS TEXT) AS reason
FROM people
WHERE id = CAST(sqlc.arg(person_id) AS INTEGER);

-- name: SetPersonWeekOverride :one
INSERT INTO person_week_overrides (person_id, week_start, capacity)
VALUES (?, ?, ?)
ON CONFLICT (person_id, week_start) DO UPDATE SET capacity = excluded.capacity
RETURNING person_id, week_start, capacity;

-- name: ListPersonWeekOverrides :many
SELECT person_id, week_start, capacity
FROM person_week_overrides WHERE person_id = ? ORDER BY week_start;

-- name: DeletePersonWeekOverride :execrows
DELETE FROM person_week_overrides
WHERE person_id = ? AND week_start = ?;

-- name: GetEffectiveCapacity :one
SELECT CAST(COALESCE(
    (SELECT capacity FROM person_week_overrides
     WHERE person_id = CAST(sqlc.arg(person_id) AS INTEGER)
       AND week_start = sqlc.arg(week_start)),
    weekly_capacity
) AS REAL) AS capacity
FROM people
WHERE id = CAST(sqlc.arg(person_id) AS INTEGER);
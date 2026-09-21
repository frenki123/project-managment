-- name: GetMeta :one
SELECT value FROM meta WHERE key = ?;

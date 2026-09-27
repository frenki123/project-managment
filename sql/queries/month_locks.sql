-- name: GetGlobalUnlock :one
SELECT unlocked FROM edit_lock WHERE id = 1;

-- name: SetGlobalUnlock :exec
UPDATE edit_lock SET unlocked = ? WHERE id = 1;

-- name: ListMonthLocks :many
SELECT year_month, unlocked FROM month_locks ORDER BY year_month;

-- name: ListUnlockedMonths :many
SELECT year_month FROM month_locks WHERE unlocked = 1 ORDER BY year_month;

-- name: UpsertMonthLock :one
INSERT INTO month_locks (year_month, unlocked)
VALUES (?, ?)
ON CONFLICT (year_month) DO UPDATE SET unlocked = excluded.unlocked
RETURNING *;

-- name: ListObjectivesByWeek :many
SELECT id, week_start, title, description, status, position, created_at, updated_at
FROM weekly_objectives
WHERE week_start = ?
ORDER BY position;

-- name: GetObjective :one
SELECT id, week_start, title, description, status, position, created_at, updated_at
FROM weekly_objectives WHERE id = ?;

-- name: CreateObjective :one
INSERT INTO weekly_objectives (id, week_start, title, description, status, position)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, week_start, title, description, status, position, created_at, updated_at;

-- name: UpdateObjective :one
UPDATE weekly_objectives SET
    title       = ?,
    description = ?,
    status      = ?,
    position    = ?,
    updated_at  = datetime('now')
WHERE id = ?
RETURNING id, week_start, title, description, status, position, created_at, updated_at;

-- name: DeleteObjective :exec
DELETE FROM weekly_objectives WHERE id = ?;

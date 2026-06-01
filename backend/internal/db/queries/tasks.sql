-- name: GetTask :one
SELECT id, title, description, planned_date, week_start, status, position,
       time_estimate_minutes, time_actual_minutes, parent_task_id, weekly_objective_id,
       source, source_id, source_url, source_metadata,
       completed_at, archived_at, created_at, updated_at
FROM tasks WHERE id = ?;

-- name: ListTasksByDate :many
SELECT id, title, description, planned_date, week_start, status, position,
       time_estimate_minutes, time_actual_minutes, parent_task_id, weekly_objective_id,
       source, source_id, source_url, source_metadata,
       completed_at, archived_at, created_at, updated_at
FROM tasks WHERE planned_date = ? ORDER BY status, position;

-- name: ListTasksByWeek :many
SELECT id, title, description, planned_date, week_start, status, position,
       time_estimate_minutes, time_actual_minutes, parent_task_id, weekly_objective_id,
       source, source_id, source_url, source_metadata,
       completed_at, archived_at, created_at, updated_at
FROM tasks WHERE week_start = ? ORDER BY planned_date, status, position;

-- name: ListBacklog :many
SELECT id, title, description, planned_date, week_start, status, position,
       time_estimate_minutes, time_actual_minutes, parent_task_id, weekly_objective_id,
       source, source_id, source_url, source_metadata,
       completed_at, archived_at, created_at, updated_at
FROM tasks
WHERE planned_date IS NULL AND status NOT IN ('done', 'cancelled')
ORDER BY position;

-- name: CreateTask :one
INSERT INTO tasks (
    id, title, description, planned_date, week_start, status, position,
    time_estimate_minutes, parent_task_id, weekly_objective_id,
    source, source_id, source_url, source_metadata
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, title, description, planned_date, week_start, status, position,
          time_estimate_minutes, time_actual_minutes, parent_task_id, weekly_objective_id,
          source, source_id, source_url, source_metadata,
          completed_at, archived_at, created_at, updated_at;

-- name: UpdateTask :one
UPDATE tasks SET
    title                 = ?,
    description           = ?,
    status                = ?,
    position              = ?,
    planned_date          = ?,
    week_start            = ?,
    time_estimate_minutes = ?,
    time_actual_minutes   = ?,
    weekly_objective_id   = ?,
    completed_at          = ?,
    updated_at            = datetime('now')
WHERE id = ?
RETURNING id, title, description, planned_date, week_start, status, position,
          time_estimate_minutes, time_actual_minutes, parent_task_id, weekly_objective_id,
          source, source_id, source_url, source_metadata,
          completed_at, archived_at, created_at, updated_at;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE id = ?;

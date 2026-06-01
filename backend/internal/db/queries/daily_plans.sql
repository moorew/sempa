-- name: GetDailyPlan :one
SELECT id, plan_date, status, intention, reflection, wins, shutdown_at, created_at, updated_at
FROM daily_plans WHERE plan_date = ?;

-- name: UpsertDailyPlan :one
INSERT INTO daily_plans (id, plan_date, status, intention, reflection, wins, shutdown_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(plan_date) DO UPDATE SET
    status      = excluded.status,
    intention   = excluded.intention,
    reflection  = excluded.reflection,
    wins        = excluded.wins,
    shutdown_at = excluded.shutdown_at,
    updated_at  = datetime('now')
RETURNING id, plan_date, status, intention, reflection, wins, shutdown_at, created_at, updated_at;

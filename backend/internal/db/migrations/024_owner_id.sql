-- Phase 1B: per-row data ownership. Every user-owned entity gets an owner_id
-- pointing at users.id. Empty string '' means "unowned" — the state of every
-- row created before multi-user; a startup backfill (db.BackfillOwnership)
-- assigns those to the primary (oldest) user so the existing single user keeps
-- all their data. Reads/writes are NOT scoped yet (that's the next step); adding
-- these columns is behaviour-neutral.
--
-- Scoped here: the household-shareable + personal entities. Tags stay global;
-- attachments inherit ownership via their parent task/objective.
ALTER TABLE tasks              ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE lists              ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE list_items         ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE weekly_objectives  ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE daily_plans        ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE week_reviews       ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE pomodoro_sessions  ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';

-- Speed up the per-user scoping queries that land next.
CREATE INDEX IF NOT EXISTS idx_tasks_owner             ON tasks(owner_id);
CREATE INDEX IF NOT EXISTS idx_lists_owner             ON lists(owner_id);
CREATE INDEX IF NOT EXISTS idx_list_items_owner        ON list_items(owner_id);
CREATE INDEX IF NOT EXISTS idx_weekly_objectives_owner ON weekly_objectives(owner_id);
CREATE INDEX IF NOT EXISTS idx_daily_plans_owner       ON daily_plans(owner_id);
CREATE INDEX IF NOT EXISTS idx_week_reviews_owner       ON week_reviews(owner_id);
CREATE INDEX IF NOT EXISTS idx_pomodoro_sessions_owner ON pomodoro_sessions(owner_id);

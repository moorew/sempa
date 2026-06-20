-- Phase 2: sharing. The household-shareable entities get a `shared` flag.
-- Visibility rule everywhere: a row is visible to user U when
--   owner_id = U  OR  shared = 1
-- shared = 0 (the default) means private to the owner.
--
-- list_items carry their own `shared` (denormalised from the parent list) and
-- subtasks are rows in `tasks` (so they already have it) — both are kept in
-- lockstep with their parent on every share-toggle, so scoping queries stay
-- uniform (owner_id = ? OR shared = 1) with no joins.
--
-- daily_plans, week_reviews, pomodoro_sessions are personal: NO shared column,
-- always owner-scoped.
ALTER TABLE tasks             ADD COLUMN shared INTEGER NOT NULL DEFAULT 0;
ALTER TABLE lists             ADD COLUMN shared INTEGER NOT NULL DEFAULT 0;
ALTER TABLE list_items        ADD COLUMN shared INTEGER NOT NULL DEFAULT 0;
ALTER TABLE weekly_objectives ADD COLUMN shared INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_tasks_shared             ON tasks(shared);
CREATE INDEX IF NOT EXISTS idx_lists_shared             ON lists(shared);
CREATE INDEX IF NOT EXISTS idx_list_items_shared        ON list_items(shared);
CREATE INDEX IF NOT EXISTS idx_weekly_objectives_shared ON weekly_objectives(shared);

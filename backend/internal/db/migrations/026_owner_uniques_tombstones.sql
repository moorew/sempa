-- Phase 1B-2 plumbing for per-user scoping.
--
-- 1) daily_plans & week_reviews are keyed one-per-date. With multiple users that
--    must become one-per-(user,date), so the single-column UNIQUE(date) has to
--    become UNIQUE(owner_id, date). SQLite can't drop an inline UNIQUE, so we
--    rebuild each table (data is tiny — single-user history).
-- 2) sync_tombstones learns who owned the deleted row + whether it was shared +
--    a kind, so the sync pull can send each deletion only to the users who could
--    actually see it (and handle un-share "revocations" — peers drop it, the
--    owner keeps it).

-- ── daily_plans rebuild ──────────────────────────────────────────────────────
CREATE TABLE daily_plans_new (
    id          TEXT PRIMARY KEY,
    plan_date   TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK(status IN ('pending', 'planning', 'active', 'shutdown_complete')),
    intention   TEXT,
    reflection  TEXT,
    wins        TEXT,
    shutdown_at TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    owner_id    TEXT NOT NULL DEFAULT '',
    UNIQUE(owner_id, plan_date)
);
INSERT INTO daily_plans_new
    (id, plan_date, status, intention, reflection, wins, shutdown_at, created_at, updated_at, owner_id)
    SELECT id, plan_date, status, intention, reflection, wins, shutdown_at, created_at, updated_at, owner_id
    FROM daily_plans;
DROP TABLE daily_plans;
ALTER TABLE daily_plans_new RENAME TO daily_plans;
CREATE INDEX IF NOT EXISTS idx_daily_plans_owner ON daily_plans(owner_id);

-- ── week_reviews rebuild ─────────────────────────────────────────────────────
CREATE TABLE week_reviews_new (
    id          TEXT PRIMARY KEY,
    week_start  TEXT NOT NULL,
    wins        TEXT,
    challenges  TEXT,
    next_focus  TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    owner_id    TEXT NOT NULL DEFAULT '',
    UNIQUE(owner_id, week_start)
);
INSERT INTO week_reviews_new
    (id, week_start, wins, challenges, next_focus, created_at, updated_at, owner_id)
    SELECT id, week_start, wins, challenges, next_focus, created_at, updated_at, owner_id
    FROM week_reviews;
DROP TABLE week_reviews;
ALTER TABLE week_reviews_new RENAME TO week_reviews;
CREATE INDEX IF NOT EXISTS idx_week_reviews_owner ON week_reviews(owner_id);

-- ── sync_tombstones: per-user deletion routing ───────────────────────────────
-- owner_id = '' means a global entity (e.g. tags) → delivered to everyone.
-- kind 'delete' goes to viewers (owner, or anyone if it was shared/global);
-- kind 'revoke' (an un-share) goes to everyone EXCEPT the owner.
ALTER TABLE sync_tombstones ADD COLUMN owner_id   TEXT    NOT NULL DEFAULT '';
ALTER TABLE sync_tombstones ADD COLUMN was_shared INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sync_tombstones ADD COLUMN kind       TEXT    NOT NULL DEFAULT 'delete';

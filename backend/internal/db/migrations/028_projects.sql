-- Projects: long-lived, multi-month containers (like a Jira epic) that group
-- tasks, weekly objectives and lists. A project is the long arc; a weekly
-- objective is that week's slice of it; tasks are the daily work.
--
-- Each task/objective/list belongs to at most one project (project_id). Subtasks
-- and recurring instances inherit their parent's/template's project.

-- ── Project statuses ─────────────────────────────────────────────────────────
-- User-customisable (Settings → Projects), global like tag_definitions. Every
-- status maps to one fixed `category`, which is what app logic keys off (e.g.
-- weekly planning lists `active` projects; `done`/`archived` hide by default),
-- so users can rename, recolour, reorder and add statuses freely.
CREATE TABLE IF NOT EXISTS project_statuses (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    color      TEXT NOT NULL DEFAULT '#8a8178',
    category   TEXT NOT NULL DEFAULT 'active'
                   CHECK(category IN ('planning', 'active', 'on_hold', 'done', 'archived')),
    position   REAL NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Seeded with fixed ids so every install (and every client) agrees on them.
INSERT OR IGNORE INTO project_statuses (id, name, color, category, position) VALUES
    ('planning', 'Planning', '#8a8178', 'planning', 0),
    ('active',   'Active',   '#b3592e', 'active',   1),
    ('on_hold',  'On hold',  '#c99a2e', 'on_hold',  2),
    ('done',     'Done',     '#4f8a5b', 'done',     3),
    ('archived', 'Archived', '#9a948d', 'archived', 4);

-- ── Projects ─────────────────────────────────────────────────────────────────
-- status_id deliberately has no FK: deleting a status reassigns its projects in
-- the handler (bumping updated_at so the change syncs), and local-first clients
-- must never fail an upsert on a status that hasn't arrived yet.
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL DEFAULT '',
    description TEXT,
    status_id   TEXT NOT NULL DEFAULT 'active',
    color       TEXT NOT NULL DEFAULT '#b3592e',
    start_date  TEXT,
    target_date TEXT,
    position    REAL NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    owner_id    TEXT NOT NULL DEFAULT '',
    shared      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_projects_owner  ON projects(owner_id);
CREATE INDEX IF NOT EXISTS idx_projects_shared ON projects(shared);

-- ── Links ────────────────────────────────────────────────────────────────────
-- ON DELETE SET NULL is a backstop only: it doesn't touch updated_at, so the
-- delete handler clears these explicitly (with a bump) so clients see it.
ALTER TABLE tasks             ADD COLUMN project_id TEXT REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE weekly_objectives ADD COLUMN project_id TEXT REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE lists             ADD COLUMN project_id TEXT REFERENCES projects(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_project      ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_objectives_project ON weekly_objectives(project_id);
CREATE INDEX IF NOT EXISTS idx_lists_project      ON lists(project_id);

-- ── Attachments on projects ──────────────────────────────────────────────────
-- SQLite can't alter a CHECK constraint, so rebuild to allow owner_type 'project'.
-- Nothing references attachments, so the swap is FK-safe.
CREATE TABLE attachments_new (
    id          TEXT PRIMARY KEY,
    owner_type  TEXT NOT NULL CHECK(owner_type IN ('task', 'objective', 'project')),
    owner_id    TEXT NOT NULL,
    filename    TEXT NOT NULL,
    mime_type   TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO attachments_new (id, owner_type, owner_id, filename, mime_type, size_bytes, created_at)
    SELECT id, owner_type, owner_id, filename, mime_type, size_bytes, created_at FROM attachments;
DROP TABLE attachments;
ALTER TABLE attachments_new RENAME TO attachments;
CREATE INDEX IF NOT EXISTS idx_attachments_owner ON attachments(owner_type, owner_id);

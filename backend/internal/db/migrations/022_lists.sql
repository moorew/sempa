-- Lists: standalone, undated checklists (e.g. an ongoing groceries list) that can
-- optionally be linked to a task. Items get a simple done flag (greyed/struck in
-- place — no separate archive). Lists persist independently and are only archived
-- manually, or automatically on task completion when the user opts in per list.
CREATE TABLE IF NOT EXISTS lists (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL DEFAULT '',
    task_id             TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    position            REAL NOT NULL DEFAULT 0,
    archived_at         TEXT,
    archive_on_complete INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS list_items (
    id         TEXT PRIMARY KEY,
    list_id    TEXT NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    text       TEXT NOT NULL DEFAULT '',
    position   REAL NOT NULL DEFAULT 0,
    done       INTEGER NOT NULL DEFAULT 0,
    category   TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_lists_task ON lists(task_id);
CREATE INDEX IF NOT EXISTS idx_list_items_list ON list_items(list_id);

-- Tags stored as JSON array on each task, e.g. '["work","personal"]'
ALTER TABLE tasks ADD COLUMN tags TEXT NOT NULL DEFAULT '[]';

-- Recurring task support
ALTER TABLE tasks ADD COLUMN recurrence_rule       TEXT;    -- 'daily','weekdays','weekly:1','monthly:15'
ALTER TABLE tasks ADD COLUMN recurrence_origin_id  TEXT REFERENCES tasks(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN is_customized         INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_tasks_recurrence_origin ON tasks(recurrence_origin_id);

-- Tag definitions: name -> hex colour
CREATE TABLE IF NOT EXISTS tag_definitions (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  color      TEXT NOT NULL DEFAULT '#6366f1',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_definitions_name ON tag_definitions(lower(name));

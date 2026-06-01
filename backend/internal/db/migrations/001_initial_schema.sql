CREATE TABLE IF NOT EXISTS weekly_objectives (
    id          TEXT PRIMARY KEY,
    week_start  TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'active'
                    CHECK(status IN ('active', 'completed', 'cancelled')),
    position    REAL NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_objectives_week ON weekly_objectives(week_start);

CREATE TABLE IF NOT EXISTS tasks (
    id                   TEXT PRIMARY KEY,
    title                TEXT NOT NULL,
    description          TEXT,
    planned_date         TEXT,
    week_start           TEXT,
    status               TEXT NOT NULL DEFAULT 'backlog'
                             CHECK(status IN ('backlog', 'planned', 'in_progress', 'done', 'cancelled')),
    position             REAL NOT NULL DEFAULT 0,
    time_estimate_minutes INTEGER,
    time_actual_minutes  INTEGER,
    parent_task_id       TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    weekly_objective_id  TEXT REFERENCES weekly_objectives(id) ON DELETE SET NULL,
    source               TEXT CHECK(source IN ('manual', 'gmail', 'fastmail', 'jira')),
    source_id            TEXT,
    source_url           TEXT,
    source_metadata      TEXT,
    completed_at         TEXT,
    archived_at          TEXT,
    created_at           TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at           TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(source, source_id)
);

CREATE INDEX IF NOT EXISTS idx_tasks_planned_date     ON tasks(planned_date);
CREATE INDEX IF NOT EXISTS idx_tasks_week_start       ON tasks(week_start);
CREATE INDEX IF NOT EXISTS idx_tasks_status           ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_weekly_objective ON tasks(weekly_objective_id);

CREATE TABLE IF NOT EXISTS daily_plans (
    id          TEXT PRIMARY KEY,
    plan_date   TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK(status IN ('pending', 'planning', 'active', 'shutdown_complete')),
    intention   TEXT,
    reflection  TEXT,
    wins        TEXT,
    shutdown_at TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS pomodoro_sessions (
    id               TEXT PRIMARY KEY,
    task_id          TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    duration_minutes INTEGER NOT NULL DEFAULT 25,
    started_at       TEXT NOT NULL,
    completed_at     TEXT,
    was_completed    INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_pomodoro_task ON pomodoro_sessions(task_id);

CREATE TABLE IF NOT EXISTS integration_configs (
    id             TEXT PRIMARY KEY,
    type           TEXT NOT NULL UNIQUE
                       CHECK(type IN ('gmail', 'fastmail', 'jira')),
    enabled        INTEGER NOT NULL DEFAULT 1,
    config         TEXT NOT NULL DEFAULT '{}',
    last_synced_at TEXT,
    created_at     TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

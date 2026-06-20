-- Multi-user identities. The single env SEMPA_USERNAME/SEMPA_PASSWORD stays as a
-- bootstrap admin that always works (can't be locked out); this table holds the
-- real accounts: Google sign-ins and password users. Phase 1B hangs per-row
-- data ownership off users.id.
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,   -- lowercased; the identity key
    name          TEXT NOT NULL DEFAULT '',
    password_hash TEXT,                    -- bcrypt; NULL for Google-only accounts
    is_admin      INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Link a session to its user. email is already on the session; user_id is the
-- stable FK data ownership will use.
ALTER TABLE sessions ADD COLUMN user_id TEXT NOT NULL DEFAULT '';

-- Sessions carry a scope so paired devices (the Sempa Dock) get a restricted
-- token instead of full account access. Existing rows default to 'full' (normal
-- logins are unchanged). 'device' tokens are gated by an allowlist in requireAuth.
ALTER TABLE sessions ADD COLUMN scope TEXT NOT NULL DEFAULT 'full';

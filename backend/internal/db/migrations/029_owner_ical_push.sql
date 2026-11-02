-- Per-user ownership for calendar subscriptions and push targets.
--
-- These tables were instance-wide: every member could list (and delete) every
-- iCal subscription, including private feed URLs, and every task reminder was
-- pushed to every registered browser and phone. Rows created before this
-- migration are owned by the primary user via db.BackfillOwnership (they were
-- all made when the instance had one user, or by its owner).
--
-- Server-only tables: none of them is synced to local-first clients, so this
-- needs no mirror in schema.ts / db.rs / sync.svelte.ts.
ALTER TABLE ical_subscriptions ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE push_subscriptions ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE device_tokens      ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_ical_subscriptions_owner ON ical_subscriptions(owner_id);
CREATE INDEX IF NOT EXISTS idx_push_subscriptions_owner ON push_subscriptions(owner_id);
CREATE INDEX IF NOT EXISTS idx_device_tokens_owner      ON device_tokens(owner_id);

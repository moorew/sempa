-- Dock device pairing: a short code pairs a Sempa Dock (or other device) to the
-- account without ever putting a password on the device. Approving the code mints
-- a scoped, revocable session token (stored in `token`) which the device then
-- claims via the status poll. Revoking deletes the session + flags the row.
CREATE TABLE IF NOT EXISTS paired_devices (
    id            TEXT PRIMARY KEY,
    code          TEXT NOT NULL UNIQUE,   -- short human-entered pairing code
    device_name   TEXT NOT NULL DEFAULT '',
    platform      TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'pending',  -- pending | approved | revoked
    token         TEXT,                   -- minted session id (Bearer); null until approved
    token_claimed INTEGER NOT NULL DEFAULT 0,  -- token handed to the device exactly once
    expires_at    TEXT NOT NULL,          -- pairing-code expiry (RFC3339), pre-approval
    created_at    TEXT NOT NULL,
    approved_at   TEXT,
    revoked_at    TEXT
);

CREATE INDEX IF NOT EXISTS idx_paired_devices_code  ON paired_devices(code);
CREATE INDEX IF NOT EXISTS idx_paired_devices_token ON paired_devices(token);

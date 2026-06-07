-- "Roughly at" time-of-day hint for recurring tasks.
--
-- Stored as a free "HH:MM" string (24h). It dictates ONLY the visual ordering of
-- tasks in the daily list — it is NOT a calendar time block and has no duration.
-- It lives on the recurring template and is copied onto each generated instance so
-- the daily view can sort without joining back to the template.
ALTER TABLE tasks ADD COLUMN roughly_at TEXT;

-- Pristine vs. modified tracking already exists via tasks.is_customized
-- (migration 002): 0 = pristine (safe to auto-replace on rollover), 1 = modified
-- (carry forward). No new column needed for the smart-rollover logic.

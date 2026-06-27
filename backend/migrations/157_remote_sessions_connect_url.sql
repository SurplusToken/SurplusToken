-- remote_sessions.connect_url: store the Kasm auto-login connect URL on the row so a
-- reconnect can REATTACH to the same still-alive container (returning the stored URL)
-- instead of destroying + recreating the session (which loses the user's open tab and
-- in-progress conversation). Empty string when unknown (e.g. legacy rows).
-- Idempotent; safe to re-run.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE remote_sessions ADD COLUMN IF NOT EXISTS connect_url TEXT NOT NULL DEFAULT '';

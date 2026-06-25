-- remote_sessions: tracks "远程连接" (remote browser) sessions. Each row maps a
-- SurplusAI user's request to remote-connect into a contributed account's logged-in
-- ChatGPT (a Firefox session on a separate Kasm Workspaces server). One Kasm user per
-- SurplusAI user (kasm_user_id); the login identity is per-account via the Kasm seed
-- profile (KASM_SEED_ACCOUNT=<account_id>). status is one of starting/running/queued/
-- ended; a background reconciler reconciles live rows against Kasm get_kasms and marks
-- vanished sessions as ended (frees per-account ≤5 + global ≤5 concurrency slots).
-- No foreign keys (consistent with the ops/audit table design philosophy in this codebase).
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS remote_sessions (
    id              BIGSERIAL    PRIMARY KEY,
    account_id      BIGINT       NOT NULL,
    surplus_user_id BIGINT       NOT NULL,
    kasm_id         TEXT         NOT NULL DEFAULT '',
    kasm_user_id    TEXT         NOT NULL DEFAULT '',
    mode            TEXT         NOT NULL DEFAULT '',   -- "setup" or "use"
    status          TEXT         NOT NULL DEFAULT '',   -- starting | running | queued | ended
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_seen_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    ended_at        TIMESTAMPTZ                          -- nullable; set when the session is torn down/reconciled away
);

-- Per-account live-session counting (status in starting/running).
CREATE INDEX IF NOT EXISTS remotesession_account_id ON remote_sessions (account_id);
-- Per-user lookups (a user only ever sees their own sessions).
CREATE INDEX IF NOT EXISTS remotesession_surplus_user_id ON remote_sessions (surplus_user_id);
-- Global live-session counting + reconciler scans over non-ended rows.
CREATE INDEX IF NOT EXISTS remotesession_status ON remote_sessions (status);

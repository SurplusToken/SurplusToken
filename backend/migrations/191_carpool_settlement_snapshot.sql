-- Carpool settlement freeze (结算落库).
--
-- migration 187 added carpool_members.settled_at but nothing ever wrote it:
-- the settlement was recomputed live on every read. That is fine for a preview
-- and wrong for a financial record -- actual_usage_usd keeps moving (it reads
-- user_subscriptions.monthly_usage_usd), so the statement the owner collected
-- money against silently differs from the one anyone reads a day later.
--
-- Settling freezes the numbers. Every figure needed to render the statement is
-- stored, so a settled statement never depends on live usage or on the carpool
-- parameters staying untouched:
--
--   settled_floor_usage_usd     0.8 x declared x period_weeks (80% floor)
--   settled_actual_usage_usd    usage snapshot at settle time
--   settled_billable_usage_usd  max(actual, floor)
--   settled_usage_share_cny     usage pool share
--   settled_seat_fee_cny        seat fee share at launch headcount
--   settled_total_delta_cny     refund (+) / top-up (-)
--
-- floor_triggered is derived (actual <= floor), not stored.
ALTER TABLE carpool_members
    ADD COLUMN IF NOT EXISTS settled_by_user_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS settled_floor_usage_usd DECIMAL(20, 10) NULL,
    ADD COLUMN IF NOT EXISTS settled_actual_usage_usd DECIMAL(20, 10) NULL,
    ADD COLUMN IF NOT EXISTS settled_billable_usage_usd DECIMAL(20, 10) NULL,
    ADD COLUMN IF NOT EXISTS settled_usage_share_cny DECIMAL(20, 2) NULL,
    ADD COLUMN IF NOT EXISTS settled_seat_fee_cny DECIMAL(20, 2) NULL,
    ADD COLUMN IF NOT EXISTS settled_total_delta_cny DECIMAL(20, 2) NULL;

-- Car-level marker. settled_at IS NULL is the idempotency guard for the settle
-- transaction (UPDATE ... WHERE settled_at IS NULL), so a double submit cannot
-- overwrite an already-frozen statement.
ALTER TABLE carpools
    ADD COLUMN IF NOT EXISTS settled_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS settled_by_user_id BIGINT NULL;

CREATE INDEX IF NOT EXISTS idx_carpools_settled_at
    ON carpools (settled_at)
    WHERE settled_at IS NOT NULL;

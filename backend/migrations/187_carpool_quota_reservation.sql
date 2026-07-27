-- Carpool quota reservation (额度预约制): replaces per-seat capacity with a
-- weekly quota pool. Members declare a weekly quota when joining, prepay by
-- declaration, launch is triggered manually once total declarations enter the
-- [launch_min_ratio, launch_max_ratio] band, and each member subscription
-- gets a per-subscription weekly limit override (reserve + shared pool).

-- carpools: quota pool & pricing parameters (defaults match design doc §3).
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS weekly_limit_usd DECIMAL(20, 10) NOT NULL DEFAULT 2400;
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS seat_fee_cny DECIMAL(20, 2) NOT NULL DEFAULT 400;
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS usage_pool_cny DECIMAL(20, 2) NOT NULL DEFAULT 1000;
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS reserve_ratio DECIMAL(3, 2) NOT NULL DEFAULT 0.80;
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS launch_min_ratio DECIMAL(4, 3) NOT NULL DEFAULT 0.950;
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS launch_max_ratio DECIMAL(4, 3) NOT NULL DEFAULT 1.050;

-- capacity/car_type/base_fee_cny/usage_pool_cny_per_account are deprecated:
-- kept for backward compatibility but no longer written or enforced.
ALTER TABLE carpools ALTER COLUMN capacity DROP NOT NULL;
ALTER TABLE carpools DROP CONSTRAINT IF EXISTS carpools_capacity_positive;

-- carpool_members: declaration, first-month prepayment ledger, settle marker.
-- declared_weekly_quota_usd carries DEFAULT 0 only so pre-existing staging rows
-- survive the migration; new joins always write an explicit positive value.
ALTER TABLE carpool_members ADD COLUMN IF NOT EXISTS declared_weekly_quota_usd DECIMAL(20, 10) NOT NULL DEFAULT 0;
ALTER TABLE carpool_members ADD COLUMN IF NOT EXISTS prepaid_amount_cny DECIMAL(20, 2) NULL;
ALTER TABLE carpool_members ADD COLUMN IF NOT EXISTS settled_at TIMESTAMPTZ NULL;

-- user_subscriptions: per-subscription weekly limit override. NULL means
-- "fall back to the group-level limit" (non-carpool subscriptions unchanged).
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS weekly_limit_usd DECIMAL(20, 10) NULL;

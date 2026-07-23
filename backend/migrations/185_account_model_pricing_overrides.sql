-- Account-specific route pricing. A rule applies only after scheduling selected
-- the configured account inside the configured group.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS account_model_pricing_overrides (
    id                BIGSERIAL      PRIMARY KEY,
    group_id          BIGINT         NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    account_id        BIGINT         NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    platform          VARCHAR(50)    NOT NULL,
    models            JSONB          NOT NULL DEFAULT '[]',
    billing_mode      VARCHAR(20)    NOT NULL DEFAULT 'token',
    input_price       NUMERIC(20,12),
    output_price      NUMERIC(20,12),
    cache_write_price NUMERIC(20,12),
    cache_read_price  NUMERIC(20,12),
    image_input_price NUMERIC(20,12),
    image_output_price NUMERIC(20,12),
    per_request_price NUMERIC(20,12),
    created_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    CONSTRAINT account_model_pricing_override_mode_check
        CHECK (billing_mode IN ('token', 'per_request', 'image'))
);

CREATE INDEX IF NOT EXISTS idx_account_model_pricing_overrides_lookup
    ON account_model_pricing_overrides (group_id, account_id, platform);

CREATE TABLE IF NOT EXISTS account_model_pricing_override_intervals (
    id                BIGSERIAL      PRIMARY KEY,
    pricing_id        BIGINT         NOT NULL REFERENCES account_model_pricing_overrides(id) ON DELETE CASCADE,
    min_tokens        INT            NOT NULL DEFAULT 0,
    max_tokens        INT,
    tier_label        VARCHAR(50),
    input_price       NUMERIC(20,12),
    output_price      NUMERIC(20,12),
    cache_write_price NUMERIC(20,12),
    cache_read_price  NUMERIC(20,12),
    per_request_price NUMERIC(20,12),
    sort_order        INT            NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    CONSTRAINT account_model_pricing_override_interval_bounds_check
        CHECK (min_tokens >= 0 AND (max_tokens IS NULL OR max_tokens > min_tokens))
);

CREATE INDEX IF NOT EXISTS idx_account_model_pricing_override_intervals_pricing
    ON account_model_pricing_override_intervals (pricing_id, sort_order, id);

COMMENT ON TABLE account_model_pricing_overrides IS
    'Final billing price for a model when a group request is served by a specific account';
COMMENT ON TABLE account_model_pricing_override_intervals IS
    'Context/token tiers for account-specific route pricing; intervals are left-open and right-closed';

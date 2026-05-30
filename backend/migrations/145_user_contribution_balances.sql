CREATE TABLE IF NOT EXISTS user_contributions (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    contribution_quota DECIMAL(20,8) NOT NULL DEFAULT 0,
    contribution_frozen_quota DECIMAL(20,8) NOT NULL DEFAULT 0,
    contribution_history_quota DECIMAL(20,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_contribution_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action VARCHAR(20) NOT NULL,
    amount DECIMAL(20,8) NOT NULL,
    usage_billing_request_id VARCHAR(128) NULL,
    api_key_id BIGINT NULL REFERENCES api_keys(id) ON DELETE SET NULL,
    account_id BIGINT NULL REFERENCES accounts(id) ON DELETE SET NULL,
    consumer_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    model VARCHAR(100) NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    image_count INTEGER NOT NULL DEFAULT 0,
    total_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    account_stats_cost DECIMAL(20,10) NULL,
    account_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1,
    reward_rate DECIMAL(10,4) NOT NULL DEFAULT 0,
    balance_after DECIMAL(20,8) NULL,
    contribution_quota_after DECIMAL(20,8) NULL,
    contribution_frozen_quota_after DECIMAL(20,8) NULL,
    contribution_history_quota_after DECIMAL(20,8) NULL,
    frozen_until TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_contribution_ledger_action_check
        CHECK (action IN ('accrue', 'transfer', 'withdraw_hold', 'withdraw_complete', 'withdraw_cancel'))
);

CREATE INDEX IF NOT EXISTS idx_user_contributions_quota
    ON user_contributions(contribution_quota);

CREATE INDEX IF NOT EXISTS idx_user_contribution_ledger_user_created
    ON user_contribution_ledger(user_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_contribution_ledger_accrue_request
    ON user_contribution_ledger(usage_billing_request_id, api_key_id, user_id)
    WHERE action = 'accrue' AND usage_billing_request_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_contribution_ledger_account_created
    ON user_contribution_ledger(account_id, created_at DESC)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_contribution_ledger_consumer_created
    ON user_contribution_ledger(consumer_user_id, created_at DESC)
    WHERE consumer_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_contribution_ledger_frozen_thaw
    ON user_contribution_ledger(user_id, frozen_until)
    WHERE frozen_until IS NOT NULL;

COMMENT ON TABLE user_contributions IS '用户贡献账号收益余额';
COMMENT ON COLUMN user_contributions.contribution_quota IS '当前可划转/可提现的贡献余额';
COMMENT ON COLUMN user_contributions.contribution_frozen_quota IS '冻结中的贡献余额';
COMMENT ON COLUMN user_contributions.contribution_history_quota IS '历史累计贡献收益';
COMMENT ON TABLE user_contribution_ledger IS '用户贡献账号收益流水';
COMMENT ON COLUMN user_contribution_ledger.reward_rate IS '贡献收益结算比例，百分比，0-100';

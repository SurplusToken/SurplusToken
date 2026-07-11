-- Shared account rate marketplace: an independent pricing axis for contributed
-- (owner_user_id IS NOT NULL) accounts, separate from accounts.rate_multiplier
-- (which remains account quota/stat scoped only). Owners set a sharing price on
-- their contributed accounts; consumers set an accepted closed-interval price
-- range. System accounts and owner/co-owner self-use are always priced at 1.0
-- and never filtered by the range.

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS sharing_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS sharing_rate_updated_at TIMESTAMPTZ NULL;

ALTER TABLE accounts
    DROP CONSTRAINT IF EXISTS accounts_sharing_rate_multiplier_check;
ALTER TABLE accounts
    ADD CONSTRAINT accounts_sharing_rate_multiplier_check
    CHECK (sharing_rate_multiplier >= 0 AND sharing_rate_multiplier <= 5) NOT VALID;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS sharing_rate_min DECIMAL(10,4) NULL,
    ADD COLUMN IF NOT EXISTS sharing_rate_max DECIMAL(10,4) NULL;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_sharing_rate_range_check;
ALTER TABLE users
    ADD CONSTRAINT users_sharing_rate_range_check
    CHECK (
        (sharing_rate_min IS NULL OR (sharing_rate_min >= 0 AND sharing_rate_min <= 5))
        AND (sharing_rate_max IS NULL OR (sharing_rate_max >= 0 AND sharing_rate_max <= 5))
        AND (sharing_rate_min IS NULL OR sharing_rate_max IS NULL OR sharing_rate_min <= sharing_rate_max)
    ) NOT VALID;

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS sharing_rate_multiplier DECIMAL(10,4) NULL;

COMMENT ON COLUMN accounts.sharing_rate_multiplier IS '贡献账号共享市场报价倍率（独立于 rate_multiplier），默认 1.0，硬范围 [0,5]';
COMMENT ON COLUMN accounts.sharing_rate_updated_at IS '共享报价最近一次变更时间，用于 owner 改价冷却判定';
COMMENT ON COLUMN users.sharing_rate_min IS '用户愿意接受的贡献账号共享报价区间下限（NULL=不限）';
COMMENT ON COLUMN users.sharing_rate_max IS '用户愿意接受的贡献账号共享报价区间上限（NULL=不限）';
COMMENT ON COLUMN usage_logs.sharing_rate_multiplier IS '计费时使用的账号共享报价倍率快照（NULL 表示按 1.0 处理）';

-- 共享市场功能开关与硬边界，默认全部关闭/维持现状；后续批次读取这些配置接入
-- 调度/计费/前端。floor/cap 是报价可设置的软边界（当前与硬 CHECK 范围一致），
-- cooldown 是 owner 相邻两次改价之间的最小间隔（分钟）。
INSERT INTO settings (key, value)
VALUES
    ('sharing_pool_display_enabled', 'false'),
    ('sharing_range_filter_enabled', 'false'),
    ('sharing_pool_billing_enabled', 'false'),
    ('sharing_rate_floor', '0'),
    ('sharing_rate_cap', '5'),
    ('sharing_rate_cooldown_minutes', '10')
ON CONFLICT (key) DO NOTHING;

-- Record and fail-close legacy ownership inconsistencies left possible by the
-- historical owner/co-owner tables having no foreign keys. Invalid primary
-- ownership is never silently reassigned because a held pool may exist; those
-- accounts are made unschedulable for explicit operator resolution. Invalid
-- co-owner rows have no principal that can receive funds and are safely removed.
CREATE TABLE IF NOT EXISTS shared_account_marketplace_migration_issues (
    issue_key  VARCHAR(160) PRIMARY KEY,
    issue_type VARCHAR(60)  NOT NULL,
    account_id BIGINT       NULL,
    user_id    BIGINT       NULL,
    details    JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO shared_account_marketplace_migration_issues
    (issue_key, issue_type, account_id, user_id, details)
SELECT
    'orphan-primary:' || accounts.id,
    'orphan_primary_owner',
    accounts.id,
    accounts.owner_user_id,
    jsonb_build_object('account_type', accounts.type)
FROM accounts
WHERE accounts.deleted_at IS NULL
  AND accounts.owner_user_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM users
      WHERE users.id = accounts.owner_user_id AND users.deleted_at IS NULL
  )
ON CONFLICT (issue_key) DO NOTHING;

INSERT INTO shared_account_marketplace_migration_issues
    (issue_key, issue_type, account_id, user_id, details)
SELECT
    'non-oauth-owner:' || accounts.id,
    'owner_account_non_oauth',
    accounts.id,
    accounts.owner_user_id,
    jsonb_build_object('account_type', accounts.type)
FROM accounts
WHERE accounts.deleted_at IS NULL
  AND accounts.owner_user_id IS NOT NULL
  AND accounts.type <> 'oauth'
ON CONFLICT (issue_key) DO NOTHING;

INSERT INTO shared_account_marketplace_migration_issues
    (issue_key, issue_type, account_id, user_id, details)
SELECT
    'invalid-coowner:' || co_owners.id,
    'invalid_coowner_reference',
    co_owners.account_id,
    co_owners.user_id,
    '{}'::jsonb
FROM account_co_owners AS co_owners
WHERE NOT EXISTS (
        SELECT 1 FROM accounts
        WHERE accounts.id = co_owners.account_id AND accounts.deleted_at IS NULL
    )
   OR NOT EXISTS (
        SELECT 1 FROM users
        WHERE users.id = co_owners.user_id AND users.deleted_at IS NULL
    )
ON CONFLICT (issue_key) DO NOTHING;

UPDATE accounts
SET schedulable = FALSE, updated_at = NOW()
WHERE deleted_at IS NULL
  AND id IN (
      SELECT account_id
      FROM shared_account_marketplace_migration_issues
      WHERE issue_type IN ('orphan_primary_owner', 'owner_account_non_oauth')
        AND account_id IS NOT NULL
  );

DELETE FROM account_co_owners AS co_owners
WHERE NOT EXISTS (
        SELECT 1 FROM accounts
        WHERE accounts.id = co_owners.account_id AND accounts.deleted_at IS NULL
    )
   OR NOT EXISTS (
        SELECT 1 FROM users
        WHERE users.id = co_owners.user_id AND users.deleted_at IS NULL
    );

-- account_contribution_pool_ledger: per-transaction, auditable accrue/distribute/
-- reversal entries for account_contribution_pools.pool_amount. Unlike the bare
-- UPSERT counter in account_contribution_pools, every change to a pool's balance
-- is recorded here so pool_amount can be reconciled as
-- SUM(opening_balance) + SUM(accrue) - SUM(distribute) - SUM(reversal) at any
-- time (all stored amounts are positive; direction determines the sign).
-- No foreign keys (consistent with account_contribution_pools' ops-table design
-- philosophy) so history survives account deletion for future audit/clawback.
CREATE TABLE IF NOT EXISTS account_contribution_pool_ledger (
    id                        BIGSERIAL PRIMARY KEY,
    account_id                BIGINT         NOT NULL,
    direction                 VARCHAR(20)    NOT NULL,
    amount                    DECIMAL(20,10) NOT NULL,
    usage_billing_request_id  VARCHAR(128)   NULL,
    api_key_id                BIGINT         NULL,
    consumer_user_id          BIGINT         NULL,
    sharing_rate_multiplier   DECIMAL(10,4)  NULL,
    reward_rate               DECIMAL(10,4)  NULL,
    actual_cost                DECIMAL(20,10) NULL,
    distribution_id           BIGINT         NULL,
    created_at                TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    CONSTRAINT account_contribution_pool_ledger_direction_check
        CHECK (direction IN ('opening_balance', 'accrue', 'distribute', 'reversal'))
);

-- Also upgrades a table created by an interrupted/preview run of this migration.
ALTER TABLE account_contribution_pool_ledger
    DROP CONSTRAINT IF EXISTS account_contribution_pool_ledger_direction_check;
ALTER TABLE account_contribution_pool_ledger
    ADD CONSTRAINT account_contribution_pool_ledger_direction_check
    CHECK (direction IN ('opening_balance', 'accrue', 'distribute', 'reversal')) NOT VALID;

CREATE INDEX IF NOT EXISTS idx_account_contribution_pool_ledger_account_created
    ON account_contribution_pool_ledger(account_id, created_at DESC);

-- Idempotency for accrual: at most one 'accrue' row per (request, api key, account).
CREATE UNIQUE INDEX IF NOT EXISTS idx_account_contribution_pool_ledger_accrue_request
    ON account_contribution_pool_ledger(usage_billing_request_id, api_key_id, account_id)
    WHERE direction = 'accrue' AND usage_billing_request_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_account_contribution_pool_ledger_distribution
    ON account_contribution_pool_ledger(distribution_id)
    WHERE distribution_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_contribution_pool_ledger_opening
    ON account_contribution_pool_ledger(account_id)
    WHERE direction = 'opening_balance';

-- Migration 156 predates the per-transaction ledger, so preserve existing held
-- balances as an explicit opening entry. Subtract any already-tracked net amount
-- to make this safe for an interrupted migration retry or a partially upgraded DB.
WITH tracked AS (
    SELECT
        account_id,
        SUM(CASE
            WHEN direction IN ('opening_balance', 'accrue') THEN amount
            WHEN direction IN ('distribute', 'reversal') THEN -amount
            ELSE 0
        END) AS net_amount
    FROM account_contribution_pool_ledger
    GROUP BY account_id
)
INSERT INTO account_contribution_pool_ledger (account_id, direction, amount, created_at)
SELECT
    pools.account_id,
    'opening_balance',
    pools.pool_amount - COALESCE(tracked.net_amount, 0),
    pools.updated_at
FROM account_contribution_pools AS pools
LEFT JOIN tracked ON tracked.account_id = pools.account_id
WHERE pools.pool_amount - COALESCE(tracked.net_amount, 0) > 0
  AND NOT EXISTS (
      SELECT 1
      FROM account_contribution_pool_ledger AS existing
      WHERE existing.account_id = pools.account_id
        AND existing.direction = 'opening_balance'
  );

-- account_contribution_pool_distributions: idempotency + strict mode enum for the
-- pool distribution API. A client-supplied idempotency_key deduplicates retries /
-- concurrent double-clicks for the same account; mode is restricted to a known enum.
CREATE TABLE IF NOT EXISTS account_contribution_pool_distributions (
    id                  BIGSERIAL PRIMARY KEY,
    account_id          BIGINT         NOT NULL,
    idempotency_key     VARCHAR(128)   NOT NULL,
    request_fingerprint VARCHAR(64)    NOT NULL,
    mode                VARCHAR(20)    NOT NULL,
    total_amount        DECIMAL(20,10) NOT NULL,
    allocations         JSONB          NOT NULL DEFAULT '[]'::jsonb,
    created_by          BIGINT         NOT NULL,
    created_at          TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    CONSTRAINT account_contribution_pool_distributions_mode_check
        CHECK (mode IN ('even', 'custom'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_contribution_pool_distributions_idem
    ON account_contribution_pool_distributions(account_id, idempotency_key);

COMMENT ON TABLE account_contribution_pool_ledger IS '账号贡献奖励池逐笔流水（opening_balance/accrue/distribute/reversal），用于对账 pool_amount';
COMMENT ON TABLE account_contribution_pool_distributions IS '账号贡献奖励池分发请求的幂等记录（account_id + idempotency_key 唯一）';

ALTER TABLE account_contribution_pool_ledger
    DROP CONSTRAINT IF EXISTS account_contribution_pool_ledger_amount_positive_check;
ALTER TABLE account_contribution_pool_ledger
    ADD CONSTRAINT account_contribution_pool_ledger_amount_positive_check
    CHECK (amount > 0) NOT VALID;

ALTER TABLE account_contribution_pools
    DROP CONSTRAINT IF EXISTS account_contribution_pools_amount_nonnegative_check;
ALTER TABLE account_contribution_pools
    ADD CONSTRAINT account_contribution_pools_amount_nonnegative_check
    CHECK (pool_amount >= 0) NOT VALID;

ALTER TABLE account_contribution_pool_distributions
    DROP CONSTRAINT IF EXISTS account_contribution_pool_distributions_amount_positive_check;
ALTER TABLE account_contribution_pool_distributions
    ADD CONSTRAINT account_contribution_pool_distributions_amount_positive_check
    CHECK (total_amount > 0) NOT VALID;

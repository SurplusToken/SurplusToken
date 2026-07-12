ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS dynamic_sharing_pool BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN groups.dynamic_sharing_pool IS
    '动态共享池分组：仅调度用户贡献账号，并按账号共享报价执行区间过滤、排序、计费和奖励';

ALTER TABLE groups
    DROP CONSTRAINT IF EXISTS groups_dynamic_sharing_pool_config_check;
ALTER TABLE groups
    ADD CONSTRAINT groups_dynamic_sharing_pool_config_check
    CHECK (
        NOT dynamic_sharing_pool
        OR (subscription_type = 'standard' AND rate_multiplier = 1.0)
    ) NOT VALID;

ALTER TABLE groups VALIDATE CONSTRAINT groups_dynamic_sharing_pool_config_check;

ALTER TABLE user_group_rate_multipliers
    ADD COLUMN IF NOT EXISTS sharing_rate_min DECIMAL(10,4) NULL,
    ADD COLUMN IF NOT EXISTS sharing_rate_max DECIMAL(10,4) NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'user_group_sharing_rate_range_valid'
    ) THEN
        ALTER TABLE user_group_rate_multipliers
            ADD CONSTRAINT user_group_sharing_rate_range_valid CHECK (
                (sharing_rate_min IS NULL OR (sharing_rate_min >= 0 AND sharing_rate_min <= 5))
                AND (sharing_rate_max IS NULL OR (sharing_rate_max >= 0 AND sharing_rate_max <= 5))
                AND (sharing_rate_min IS NULL OR sharing_rate_max IS NULL OR sharing_rate_min <= sharing_rate_max)
            );
    END IF;
END $$;

-- Preserve existing preferences while moving ownership from the user profile
-- to each dynamic group. Subsequent edits are isolated by (user_id, group_id).
INSERT INTO user_group_rate_multipliers (
    user_id,
    group_id,
    sharing_rate_min,
    sharing_rate_max,
    created_at,
    updated_at
)
SELECT
    users.id,
    groups.id,
    users.sharing_rate_min,
    users.sharing_rate_max,
    NOW(),
    NOW()
FROM users
CROSS JOIN groups
WHERE groups.dynamic_sharing_pool = TRUE
  AND (users.sharing_rate_min IS NOT NULL OR users.sharing_rate_max IS NOT NULL)
ON CONFLICT (user_id, group_id) DO UPDATE SET
    sharing_rate_min = COALESCE(user_group_rate_multipliers.sharing_rate_min, EXCLUDED.sharing_rate_min),
    sharing_rate_max = COALESCE(user_group_rate_multipliers.sharing_rate_max, EXCLUDED.sharing_rate_max),
    updated_at = NOW();

COMMENT ON COLUMN user_group_rate_multipliers.sharing_rate_min IS
    '用户在该动态分组接受的共享报价下限；NULL 表示下限不限';
COMMENT ON COLUMN user_group_rate_multipliers.sharing_rate_max IS
    '用户在该动态分组接受的共享报价上限；NULL 表示上限不限';

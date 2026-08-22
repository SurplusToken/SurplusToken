-- Carpool car type (整数车型) + boarding risk acknowledgement (上车风险确认).
--
-- The new integer car_type replaces the deprecated seat-model car_type VARCHAR
-- ('small'/'large', created by migration 182, deprecated together with
-- capacity/base_fee_cny by 187 when the quota reservation model landed):
--
--   0 = custom 自定义规则车（pricing_model='custom'，人工结算，migration 192）
--   1 = 无保底机制的老 quota 车：发过车，但成员订阅没有 weekly_reserved_usd
--       （发车早于 migration 189 引入保底列）
--   2 = 现行额度预约制存量车（$2400 / ¥400 / ¥1000，保底 80%，个人上限 r+C）：
--       任一成员订阅带 weekly_reserved_usd NOT NULL，或从未发车
--       （launched_at IS NULL，覆盖 recruiting/confirmed 存量车——它们创建时
--       就是 quota 规则，只是还没有订阅可判）
--   3 = 新计价车（$2800 / 席位费 ¥50 每人固定（不按人头均摊）/ 变动池 ¥1200，
--       保底 80%，个人上限 2×申报，上车必须勾选风险确认）：此后 Create 新建的
--       quota 车恒为 3，列默认值也是 3。

-- 旧列与新的整数列同名但类型是 VARCHAR，先删旧列。仅当旧列仍不是 integer 时
-- 才删（重复执行时新列已是 integer，整个迁移变为无操作，不会误删已回填/已写入的数据）。
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'carpools' AND column_name = 'car_type' AND data_type <> 'integer'
    ) THEN
        ALTER TABLE carpools DROP COLUMN car_type;
    END IF;
END $$;

ALTER TABLE carpools ADD COLUMN IF NOT EXISTS car_type INTEGER;

-- 回填只填 NULL（与 192 的 pricing_model IS NULL 同一幂等口径）：判别顺序为
-- custom → 0；从未发车 → 2；有保底订阅 → 2；其余（发过车却无保底）→ 1。
UPDATE carpools
SET car_type = CASE
    WHEN COALESCE(pricing_model, 'quota') = 'custom' THEN 0
    WHEN launched_at IS NULL THEN 2
    WHEN EXISTS (
        SELECT 1 FROM carpool_members m
        JOIN user_subscriptions s ON s.id = m.subscription_id
        WHERE m.carpool_id = carpools.id
          AND s.weekly_reserved_usd IS NOT NULL
    ) THEN 2
    ELSE 1
END
WHERE car_type IS NULL;

ALTER TABLE carpools ALTER COLUMN car_type SET DEFAULT 3;
ALTER TABLE carpools ALTER COLUMN car_type SET NOT NULL;

-- 上车风险确认（type 3 车必填；其余车型不强制，传了也照存）。参照 188 的
-- joined_wechat_group：DEFAULT false 仅为兼容存量行，新上车行总是显式写入。
ALTER TABLE carpool_members ADD COLUMN IF NOT EXISTS acknowledged_risk BOOLEAN NOT NULL DEFAULT false;

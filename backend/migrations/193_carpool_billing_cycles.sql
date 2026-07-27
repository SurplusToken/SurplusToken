-- Per-cycle carpool billing history (计费周期台账).
--
-- 80% 地板必须按**周**结算，不能按月整体算：
--
--   正确  Σ max(周实际用量, 0.8 × 申报)
--   错误  max(Σ周实际用量, 0.8 × 申报 × 周数)
--
-- 两者不等价。申报 $240/周、四周实际用量 300/300/50/50 时，按周算是
-- 300+300+192+192 = 984，按月算是 max(700, 768) = 768，差 $216。按月算等于
-- 让超用的那周补贴没用满的那周，而设计文档写明"锁定额度按周刷新、未用完不
-- 结转"——不结转就该各周各算。
--
-- 但按周算需要历史周用量，而 user_subscriptions.weekly_usage_usd 每到周窗口
-- 重置就被清零，历史无处可寻。所以这里把每个**关闭的**计费周期落库：周窗口
-- 推进时写一行，记下那一周的申报、保底、实际用量与计费用量。
--
-- 当期（尚未关闭）的周期不在表里，结算时从订阅的当前 weekly_usage_usd 现取。
CREATE TABLE IF NOT EXISTS carpool_billing_cycles (
    id                        BIGSERIAL PRIMARY KEY,
    carpool_id                BIGINT NOT NULL REFERENCES carpools(id) ON DELETE CASCADE,
    user_id                   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id           BIGINT NULL,
    group_id                  BIGINT NULL,

    cycle_start               TIMESTAMPTZ NOT NULL,
    cycle_end                 TIMESTAMPTZ NOT NULL,

    declared_weekly_quota_usd DECIMAL(20, 10) NOT NULL,
    -- reserved = reserve_ratio x declared，即这一周的地板
    reserved_usd              DECIMAL(20, 10) NOT NULL,
    actual_usage_usd          DECIMAL(20, 10) NOT NULL,
    -- billable = max(actual, reserved)，落库时就定死，避免日后改参数改写历史
    billable_usage_usd        DECIMAL(20, 10) NOT NULL,

    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 幂等守卫：同一条订阅的同一个周期起点只能有一行。周重置可能被并发请求
-- 同时触发，没有这个约束会重复计费。
CREATE UNIQUE INDEX IF NOT EXISTS uq_carpool_billing_cycles_sub_start
    ON carpool_billing_cycles (subscription_id, cycle_start)
    WHERE subscription_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_carpool_billing_cycles_carpool
    ON carpool_billing_cycles (carpool_id, cycle_start);

CREATE INDEX IF NOT EXISTS idx_carpool_billing_cycles_user
    ON carpool_billing_cycles (user_id, cycle_start);

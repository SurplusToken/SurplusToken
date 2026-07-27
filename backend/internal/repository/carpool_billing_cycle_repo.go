package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type carpoolBillingCycleRepository struct {
	db *sql.DB
}

// NewCarpoolBillingCycleRepository 创建计费周期台账仓储。
func NewCarpoolBillingCycleRepository(db *sql.DB) service.CarpoolBillingCycleRecorder {
	return &carpoolBillingCycleRepository{db: db}
}

// RecordCycle 把订阅当前所处的周期落成台账。
//
// 所有数值都在同一条语句里从 user_subscriptions 行直接读取，不接受调用方传值：
// 调用点持有的是内存快照，而 ValidateAndCheckLimits 为了让预检查不误拒用户，
// 已经把 WeeklyUsageUSD 清零了；照它记账会让每个周期的实际用量都是 0、计费
// 恒等于保底，重度用户的超出部分全部漏计。
//
// 必须在 ResetWeeklyUsage 之前调用——之后订阅行上的用量就没了。
//
// ON CONFLICT DO NOTHING 配合唯一索引保证幂等：周重置可能被并发请求同时触发。
func (r *carpoolBillingCycleRepository) RecordCycle(ctx context.Context, subscriptionID int64, cycleEnd time.Time) error {
	if r == nil || r.db == nil || subscriptionID <= 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO carpool_billing_cycles (
    carpool_id, user_id, subscription_id, group_id,
    cycle_start, cycle_end,
    declared_weekly_quota_usd, reserved_usd, actual_usage_usd, billable_usage_usd)
SELECT c.id, s.user_id, s.id, s.group_id,
       s.weekly_window_start, $2,
       COALESCE(m.declared_weekly_quota_usd, 0),
       s.weekly_reserved_usd,
       s.weekly_usage_usd,
       GREATEST(s.weekly_usage_usd, s.weekly_reserved_usd)
FROM user_subscriptions s
JOIN carpools c ON c.group_id = s.group_id
JOIN carpool_members m ON m.carpool_id = c.id AND m.user_id = s.user_id
WHERE s.id = $1
  AND s.deleted_at IS NULL
  AND s.weekly_reserved_usd IS NOT NULL
  AND s.weekly_window_start IS NOT NULL
ON CONFLICT (subscription_id, cycle_start) WHERE subscription_id IS NOT NULL DO NOTHING`,
		subscriptionID, cycleEnd)
	if err != nil {
		return fmt.Errorf("record carpool billing cycle: %w", err)
	}
	return nil
}

// ListCyclesByCarpool 取回某辆车在 [from, to) 内的全部已关闭周期，按成员与时间排序。
func (r *carpoolBillingCycleRepository) ListCyclesByCarpool(ctx context.Context, carpoolID int64, from, to time.Time) ([]service.CarpoolBillingCycle, error) {
	if r == nil || r.db == nil || carpoolID <= 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT user_id, subscription_id, group_id, cycle_start, cycle_end,
       declared_weekly_quota_usd, reserved_usd, actual_usage_usd, billable_usage_usd
FROM carpool_billing_cycles
WHERE carpool_id = $1
  AND ($2::timestamptz IS NULL OR cycle_start >= $2)
  AND ($3::timestamptz IS NULL OR cycle_start < $3)
ORDER BY user_id, cycle_start`, carpoolID, nullableTime(from), nullableTime(to))
	if err != nil {
		return nil, fmt.Errorf("list carpool billing cycles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.CarpoolBillingCycle, 0)
	for rows.Next() {
		var item service.CarpoolBillingCycle
		var subID, groupID sql.NullInt64
		if err := rows.Scan(&item.UserID, &subID, &groupID,
			&item.CycleStart, &item.CycleEnd,
			&item.DeclaredWeeklyQuotaUSD, &item.ReservedUSD,
			&item.ActualUsageUSD, &item.BillableUsageUSD); err != nil {
			return nil, fmt.Errorf("scan carpool billing cycle: %w", err)
		}
		item.CarpoolID = carpoolID
		if subID.Valid {
			item.SubscriptionID = &subID.Int64
		}
		if groupID.Valid {
			item.GroupID = &groupID.Int64
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate carpool billing cycles: %w", err)
	}
	return items, nil
}

// nullableTime 把零值时间映射成 NULL，让查询里的范围条件可选。
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

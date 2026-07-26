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

// RecordCycle 写入一个已关闭的计费周期。
//
// carpool_id 由订阅所属的组反查（一辆车对应一个组）。查不到就说明这条订阅
// 不属于任何在跑的拼车，直接跳过——不为孤儿订阅造台账。
//
// ON CONFLICT DO NOTHING 配合唯一索引保证幂等：周重置可能被并发请求同时触发，
// 没有这一层会把同一周重复计费。
func (r *carpoolBillingCycleRepository) RecordCycle(ctx context.Context, cycle *service.CarpoolBillingCycle) error {
	if r == nil || r.db == nil || cycle == nil || cycle.SubscriptionID == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO carpool_billing_cycles (
    carpool_id, user_id, subscription_id, group_id,
    cycle_start, cycle_end,
    declared_weekly_quota_usd, reserved_usd, actual_usage_usd, billable_usage_usd)
SELECT c.id, $1, $2, $3, $4, $5,
       COALESCE(m.declared_weekly_quota_usd, 0), $6, $7, $8
FROM carpools c
JOIN carpool_members m ON m.carpool_id = c.id AND m.user_id = $1
WHERE c.group_id = $3
ON CONFLICT (subscription_id, cycle_start) WHERE subscription_id IS NOT NULL DO NOTHING`,
		cycle.UserID, cycle.SubscriptionID, cycle.GroupID,
		cycle.CycleStart, cycle.CycleEnd,
		cycle.ReservedUSD, cycle.ActualUsageUSD, cycle.BillableUsageUSD)
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
	defer rows.Close()

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

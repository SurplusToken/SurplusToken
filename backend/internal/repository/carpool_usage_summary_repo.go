package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.CarpoolUsageSummaryRepository = (*carpoolRepository)(nil)

// ListMemberWeekUsage 返回车上 active/joined 成员的本周用量行（service 层做可见性
// 与查看者窗口维护）。口径与订阅页用量面板一致：weekly_usage_usd /
// weekly_reserved_usd 取自成员订阅行，申报取自 carpool_members。成员行未绑定订阅
// 时（Transient 状态）订阅字段按 NULL/0 处理，与 ListSettlementMembers 同款 LEFT JOIN。
func (r *carpoolRepository) ListMemberWeekUsage(ctx context.Context, carpoolID int64) ([]service.CarpoolMemberWeekUsageRow, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT m.user_id, m.subscription_id, m.declared_weekly_quota_usd,
       COALESCE(s.weekly_usage_usd, 0), s.weekly_reserved_usd, s.weekly_window_start
FROM carpool_members m
LEFT JOIN user_subscriptions s ON s.id = m.subscription_id
WHERE m.carpool_id = $1 AND m.status IN ('joined', 'active')
ORDER BY m.id`, carpoolID)
	if err != nil {
		return nil, fmt.Errorf("list carpool member week usage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.CarpoolMemberWeekUsageRow, 0)
	for rows.Next() {
		var item service.CarpoolMemberWeekUsageRow
		var subscriptionID sql.NullInt64
		var reserved sql.NullFloat64
		var windowStart sql.NullTime
		if err := rows.Scan(&item.UserID, &subscriptionID, &item.DeclaredWeeklyQuotaUSD,
			&item.WeeklyUsageUSD, &reserved, &windowStart); err != nil {
			return nil, fmt.Errorf("scan carpool member week usage: %w", err)
		}
		if subscriptionID.Valid {
			item.SubscriptionID = subscriptionID.Int64
		}
		if reserved.Valid {
			reservedUSD := reserved.Float64
			item.WeeklyReservedUSD = &reservedUSD
		}
		if windowStart.Valid {
			start := windowStart.Time
			item.WeeklyWindowStart = &start
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate carpool member week usage: %w", err)
	}
	return items, nil
}

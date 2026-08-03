package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ResetGroupWeeklyWindow 实现 service.CarpoolGroupWindowResetter。
//
// 一个事务做三件事，顺序不能反：
//  1. 锁住该组全部拼车订阅（FOR UPDATE），顺便读出当前窗口起点用于并发去重；
//  2. 给每张订阅落上一周的计费台账——必须在清零之前，否则该周用量永远拿不回来；
//  3. 把全组的窗口起点改成同一个 target 并清零本周用量。
//
// 第 3 步写的是同一个值，这正是整件事的意义：全车窗口从此逐字节相同，
// 公共池计数器的 key 不会再裂开。
func (r *carpoolUpstreamWindowRepository) ResetGroupWeeklyWindow(
	ctx context.Context, groupID int64, target time.Time, tolerance time.Duration,
) (*service.CarpoolGroupWindowReset, error) {
	if r == nil || r.db == nil || groupID <= 0 || target.IsZero() {
		return &service.CarpoolGroupWindowReset{}, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin carpool group window reset: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 锁住全组的拼车订阅。拿 weekly_reserved_usd IS NOT NULL 区分拼车订阅与普通订阅。
	rows, err := tx.QueryContext(ctx, `
SELECT id, user_id, weekly_window_start
FROM user_subscriptions
WHERE group_id = $1
  AND deleted_at IS NULL
  AND weekly_reserved_usd IS NOT NULL
  AND weekly_window_start IS NOT NULL
ORDER BY id
FOR UPDATE`, groupID)
	if err != nil {
		return nil, fmt.Errorf("lock carpool group subscriptions: %w", err)
	}
	type subRow struct {
		id, userID  int64
		windowStart time.Time
	}
	subs := make([]subRow, 0, 16)
	var newest time.Time
	for rows.Next() {
		var s subRow
		if err := rows.Scan(&s.id, &s.userID, &s.windowStart); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan carpool group subscription: %w", err)
		}
		if s.windowStart.After(newest) {
			newest = s.windowStart
		}
		subs = append(subs, s)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate carpool group subscriptions: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close carpool group subscriptions: %w", err)
	}
	if len(subs) == 0 {
		return &service.CarpoolGroupWindowReset{}, nil
	}

	// 并发去重：已经有人把窗口推到 target 附近了，就不要再推一次。
	// 少了这一步，两个并发请求算出的 target 相差几秒，后到的会把用量二次清零。
	if diff := target.Sub(newest); diff <= tolerance && diff >= -tolerance {
		return &service.CarpoolGroupWindowReset{}, nil
	}
	// 窗口只向前推进。target 落在当前窗口之前说明上游数据回退了，宁可不动。
	if !target.After(newest) {
		return &service.CarpoolGroupWindowReset{}, nil
	}

	// 落台账。与 carpoolBillingCycleRepository.RecordCycle 同一套 SQL，
	// 只是一次性覆盖整组；ON CONFLICT 让重复执行安全。
	cycles, err := tx.ExecContext(ctx, `
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
WHERE s.group_id = $1
  AND s.deleted_at IS NULL
  AND s.weekly_reserved_usd IS NOT NULL
  AND s.weekly_window_start IS NOT NULL
  AND s.weekly_window_start < $2
ON CONFLICT (subscription_id, cycle_start) WHERE subscription_id IS NOT NULL DO NOTHING`,
		groupID, target)
	if err != nil {
		return nil, fmt.Errorf("record carpool group cycles: %w", err)
	}
	cycleCount, _ := cycles.RowsAffected()

	// 重锚 + 清零。整组写同一个 target。
	if _, err := tx.ExecContext(ctx, `
UPDATE user_subscriptions
SET weekly_window_start = $2, weekly_usage_usd = 0, updated_at = NOW()
WHERE group_id = $1
  AND deleted_at IS NULL
  AND weekly_reserved_usd IS NOT NULL
  AND weekly_window_start IS NOT NULL
  AND weekly_window_start < $2`, groupID, target); err != nil {
		return nil, fmt.Errorf("reanchor carpool group window: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit carpool group window reset: %w", err)
	}

	userIDs := make([]int64, 0, len(subs))
	for _, s := range subs {
		userIDs = append(userIDs, s.userID)
	}
	return &service.CarpoolGroupWindowReset{
		Applied: true,
		From:    newest,
		To:      target,
		UserIDs: userIDs,
		Cycles:  int(cycleCount),
	}, nil
}

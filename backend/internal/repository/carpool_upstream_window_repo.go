package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// carpoolUpstreamWindowTTL 是组→上游窗口映射的进程内缓存时长。
//
// 这个查询挂在计费热路径上（每个请求都要判断窗口是否漂移），而它的答案
// 一周才变一次，所以短 TTL 缓存足够，也避免每请求一次 JOIN 查询。
const carpoolUpstreamWindowTTL = 30 * time.Second

type carpoolUpstreamWindowEntry struct {
	window   *service.CarpoolUpstreamWindow
	cachedAt time.Time
}

type carpoolCapacityEntry struct {
	snapshot *service.CarpoolCapacitySnapshot
	cachedAt time.Time
	// smoothed 是跨多次采样保留的指数平滑值：容量决定公共池还剩多少，
	// 让它随每次采样上下跳会导致用户忽而放行忽而被拒。
	smoothed float64
}

type carpoolUpstreamWindowRepository struct {
	db *sql.DB

	mu       sync.RWMutex
	cache    map[int64]carpoolUpstreamWindowEntry
	capacity map[int64]carpoolCapacityEntry
	// reservedTotals 记住每组上一次同步后的 Σ保底，用于判断本次是否真的变了。
	reservedTotals map[int64]float64
}

// NewCarpoolUpstreamWindowRepository 创建上游周窗口查询器。
func NewCarpoolUpstreamWindowRepository(db *sql.DB) service.CarpoolUpstreamWindowSource {
	return &carpoolUpstreamWindowRepository{
		db:       db,
		cache:    make(map[int64]carpoolUpstreamWindowEntry),
		capacity:       make(map[int64]carpoolCapacityEntry),
		reservedTotals: make(map[int64]float64),
	}
}

// GroupUpstreamWeeklyWindow 读取组绑定账号上由网关刷新的 codex 7 天窗口。
//
// 一辆车的周期必须由一个账号说了算，取组内优先级最高的那个（account_groups.priority，
// 也正是调度器优先派发的账号）。这样"跟谁的周期"和"优先用谁"是同一个决定，
// 运营在一个地方调整即可。
//
// 早先取的是 reset_at 最晚的账号。这在只有一个账号时无所谓，多账号时是错的：
// 给车加一个补充额度的副账号，只要它的 reset_at 比主账号晚，全车周期就会当场
// 改跟副账号走并清零——生产上真发生过，主账号窗口内已用的 773.91 USD 就此
// 从计数里消失，等于凭空多发一轮额度。副账号是来补额度的，不该决定周期。
func (r *carpoolUpstreamWindowRepository) GroupUpstreamWeeklyWindow(ctx context.Context, groupID int64) (*service.CarpoolUpstreamWindow, error) {
	if r == nil || r.db == nil || groupID <= 0 {
		return nil, nil
	}
	now := time.Now()

	r.mu.RLock()
	entry, ok := r.cache[groupID]
	r.mu.RUnlock()
	if ok && now.Sub(entry.cachedAt) < carpoolUpstreamWindowTTL {
		return entry.window, nil
	}

	var resetAt, updatedAt sql.NullTime
	var windowMinutes, usedPercent sql.NullFloat64
	err := r.db.QueryRowContext(ctx, `
SELECT (a.extra->>'codex_7d_reset_at')::timestamptz,
       (a.extra->>'codex_7d_window_minutes')::numeric,
       (a.extra->>'codex_usage_updated_at')::timestamptz,
       (a.extra->>'codex_7d_used_percent')::numeric
FROM account_groups ag
JOIN accounts a ON a.id = ag.account_id
WHERE ag.group_id = $1
  AND a.deleted_at IS NULL
  AND a.extra->>'codex_7d_reset_at' IS NOT NULL
  AND a.extra->>'codex_7d_window_minutes' IS NOT NULL
ORDER BY ag.priority ASC, a.priority ASC, a.id ASC
LIMIT 1`, groupID).Scan(&resetAt, &windowMinutes, &updatedAt, &usedPercent)

	var window *service.CarpoolUpstreamWindow
	switch {
	case err == sql.ErrNoRows:
		// 组没绑账号，或账号还没被上游响应头刷新过——不是错误，按"没有数据"处理。
		window = nil
	case err != nil:
		// 查询失败不缓存，下次重试；调用方按"没有数据"降级到本地网格。
		return nil, err
	case resetAt.Valid && windowMinutes.Valid && windowMinutes.Float64 > 0:
		end := resetAt.Time
		observed := end
		if updatedAt.Valid {
			observed = updatedAt.Time
		}
		window = &service.CarpoolUpstreamWindow{
			Start:       end.Add(-time.Duration(windowMinutes.Float64) * time.Minute),
			End:         end,
			ObservedAt:  observed,
			UsedPercent: usedPercent.Float64,
		}
	}

	r.mu.Lock()
	r.cache[groupID] = carpoolUpstreamWindowEntry{window: window, cachedAt: now}
	r.mu.Unlock()
	return window, nil
}

// GroupObservedCapacity 反推该组当前窗口的真实容量。
//
// 上游只报百分比，我们只记美元；两者一除就得到"这辆车这一周到底有多少额度"，
// 于是 $2400 可以退回去只当定价基准，不必再靠人工标定去兼任执行上限。
//
// 全车用量与保底之和一次聚合取回。查询与窗口查询同频（30 秒），
// 结果做指数平滑后缓存。
func (r *carpoolUpstreamWindowRepository) GroupObservedCapacity(ctx context.Context, groupID int64, windowStart time.Time) (*service.CarpoolCapacitySnapshot, error) {
	if r == nil || r.db == nil || groupID <= 0 || windowStart.IsZero() {
		return nil, nil
	}
	now := time.Now()

	r.mu.RLock()
	cached, ok := r.capacity[groupID]
	r.mu.RUnlock()
	if ok && now.Sub(cached.cachedAt) < carpoolUpstreamWindowTTL {
		return cached.snapshot, nil
	}

	window, err := r.GroupUpstreamWeeklyWindow(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if !window.Fresh(now) {
		// 上游数据缺失或陈旧：不给实测值，调用方退回发车时锁定的公共池容量。
		return nil, nil
	}

	var totalUsage, totalReserved sql.NullFloat64
	err = r.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(us.weekly_usage_usd), 0), COALESCE(SUM(us.weekly_reserved_usd), 0)
FROM user_subscriptions us
WHERE us.group_id = $1
  AND us.weekly_window_start = $2
  AND us.weekly_reserved_usd IS NOT NULL
  AND us.deleted_at IS NULL`, groupID, windowStart).Scan(&totalUsage, &totalReserved)
	if err != nil {
		return nil, err
	}

	sample, trusted := service.CarpoolObservedTotalCapacityUSD(totalUsage.Float64, window.UsedPercent)
	smoothed := cached.smoothed
	if trusted {
		smoothed = service.CarpoolSmoothObservedCapacity(cached.smoothed, sample)
	}

	// 保底跟随实测容量。上游被风控缩水时，钉死的 0.8×申报 会让
	// 公共池 = 容量 − Σ保底 变成 0 甚至负数，把刚越过保底的成员全拦掉
	// （生产实例：实测 1899 < Σ保底 1920，一位成员超出 0.07 即被拒）。
	// 按份额缩放后公共池恒与容量同号。
	//
	// 写库放在这里是因为本函数带 30 秒进程内缓存，每组每 30 秒最多写一次；
	// 失败只记日志不阻断请求——退回本次读到的旧 Σ保底即可。
	reservedTotal := totalReserved.Float64
	reservesSynced := false
	if trusted && smoothed > 0 {
		if synced, err := r.syncGroupReserves(ctx, groupID, smoothed); err != nil {
			logger.LegacyPrintf("repository.carpool_window",
				"ALERT: sync carpool reserves failed group=%d capacity=%.2f: %v (保底维持原值)",
				groupID, smoothed, err)
		} else if synced.changed {
			reservedTotal = synced.reservedTotal
			reservesSynced = true
		} else {
			reservedTotal = synced.reservedTotal
		}
	}

	snapshot := service.BuildCarpoolCapacitySnapshot(smoothed, reservedTotal, trusted && smoothed > 0)
	snapshot.ReservesSynced = reservesSynced

	r.mu.Lock()
	r.capacity[groupID] = carpoolCapacityEntry{snapshot: snapshot, cachedAt: now, smoothed: smoothed}
	r.mu.Unlock()
	return snapshot, nil
}

// carpoolReserveSync 是一次保底同步的结果。
type carpoolReserveSync struct {
	reservedTotal float64
	changed       bool
}

// syncGroupReserves 按实测容量重写全组成员的 weekly_reserved_usd。
//
//	保底 = 容量 × (申报 ÷ 车周限额) × reserve_ratio
//
// 只动带保底的拼车订阅（weekly_reserved_usd IS NOT NULL）——保底为 NULL 的
// 是普通订阅或已撤销受管的车，不在此列；申报为 0 的自定义规则车算出来仍是 0，
// 天然不受影响。
//
// changed 用 1e-6 的阈值判定，避免浮点噪声导致每 30 秒都被判为"变了"而
// 反复触发计数器重算。
func (r *carpoolUpstreamWindowRepository) syncGroupReserves(ctx context.Context, groupID int64, capacityUSD float64) (carpoolReserveSync, error) {
	var out carpoolReserveSync
	if r == nil || r.db == nil || groupID <= 0 || capacityUSD <= 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
UPDATE user_subscriptions us
SET weekly_reserved_usd = $2 * (m.declared_weekly_quota_usd / c.weekly_limit_usd) * c.reserve_ratio,
    updated_at = NOW()
FROM carpools c, carpool_members m
WHERE c.group_id = $1
  AND m.carpool_id = c.id
  AND m.status IN ('joined', 'active')
  AND us.group_id = $1
  AND us.user_id = m.user_id
  AND us.deleted_at IS NULL
  AND us.weekly_reserved_usd IS NOT NULL
  AND c.weekly_limit_usd > 0
  AND c.reserve_ratio > 0
RETURNING us.weekly_reserved_usd,
          $2 * (m.declared_weekly_quota_usd / c.weekly_limit_usd) * c.reserve_ratio
            IS DISTINCT FROM us.weekly_reserved_usd`, groupID, capacityUSD)
	if err != nil {
		return out, fmt.Errorf("sync carpool reserves: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var reserved float64
		var differed bool
		if err := rows.Scan(&reserved, &differed); err != nil {
			return carpoolReserveSync{}, fmt.Errorf("scan carpool reserve sync: %w", err)
		}
		out.reservedTotal += reserved
		_ = differed
	}
	if err := rows.Err(); err != nil {
		return carpoolReserveSync{}, fmt.Errorf("iterate carpool reserve sync: %w", err)
	}

	r.mu.RLock()
	prev := r.reservedTotals[groupID]
	r.mu.RUnlock()
	out.changed = math.Abs(out.reservedTotal-prev) > 1e-6
	if out.changed {
		r.mu.Lock()
		r.reservedTotals[groupID] = out.reservedTotal
		r.mu.Unlock()
	}
	return out, nil
}

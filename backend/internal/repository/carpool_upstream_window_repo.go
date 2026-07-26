package repository

import (
	"context"
	"database/sql"
	"sync"
	"time"

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

type carpoolUpstreamWindowRepository struct {
	db *sql.DB

	mu    sync.RWMutex
	cache map[int64]carpoolUpstreamWindowEntry
}

// NewCarpoolUpstreamWindowRepository 创建上游周窗口查询器。
func NewCarpoolUpstreamWindowRepository(db *sql.DB) service.CarpoolUpstreamWindowSource {
	return &carpoolUpstreamWindowRepository{
		db:    db,
		cache: make(map[int64]carpoolUpstreamWindowEntry),
	}
}

// GroupUpstreamWeeklyWindow 读取组绑定账号上由网关刷新的 codex 7 天窗口。
//
// 一辆车对应一个上游账号，所以正常情况下只有一行。真出现多账号时取
// reset_at 最晚的那个：宁可我们的窗口比上游关得晚（用户被我们多挡一会儿），
// 也不要比上游开得早——后者会让全车拿着新保底去撞一个已经耗尽的上游账号，
// 保底承诺当场失效。
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
	var windowMinutes sql.NullFloat64
	err := r.db.QueryRowContext(ctx, `
SELECT (a.extra->>'codex_7d_reset_at')::timestamptz,
       (a.extra->>'codex_7d_window_minutes')::numeric,
       (a.extra->>'codex_usage_updated_at')::timestamptz
FROM account_groups ag
JOIN accounts a ON a.id = ag.account_id
WHERE ag.group_id = $1
  AND a.deleted_at IS NULL
  AND a.extra->>'codex_7d_reset_at' IS NOT NULL
  AND a.extra->>'codex_7d_window_minutes' IS NOT NULL
ORDER BY (a.extra->>'codex_7d_reset_at')::timestamptz DESC
LIMIT 1`, groupID).Scan(&resetAt, &windowMinutes, &updatedAt)

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
			Start:      end.Add(-time.Duration(windowMinutes.Float64) * time.Minute),
			End:        end,
			ObservedAt: observed,
		}
	}

	r.mu.Lock()
	r.cache[groupID] = carpoolUpstreamWindowEntry{window: window, cachedAt: now}
	r.mu.Unlock()
	return window, nil
}

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// 拼车周额度（cpr quota-forecast）服务端缓存。完整 key：
//   - 快照：     carpool:upstream_quota:<carpoolID>         （TTL 取 cpr.cache_ttl_seconds）
//   - 刷新冷却： carpool:upstream_quota:refresh:<carpoolID> （SET NX EX，窗口取 cpr.refresh_cooldown_seconds）
const (
	carpoolUpstreamQuotaKeyPrefix        = "carpool:upstream_quota:"
	carpoolUpstreamQuotaRefreshKeyPrefix = "carpool:upstream_quota:refresh:"
	carpoolUpstreamQuotaRefreshKeyValue  = "1"
)

func carpoolUpstreamQuotaKey(carpoolID int64) string {
	return fmt.Sprintf("%s%d", carpoolUpstreamQuotaKeyPrefix, carpoolID)
}

func carpoolUpstreamQuotaRefreshKey(carpoolID int64) string {
	return fmt.Sprintf("%s%d", carpoolUpstreamQuotaRefreshKeyPrefix, carpoolID)
}

// carpoolUpstreamQuotaCache 基于 Redis 的拼车周额度快照与刷新冷却（service.CarpoolUpstreamQuotaCache）。
type carpoolUpstreamQuotaCache struct {
	rdb *redis.Client
}

// NewCarpoolUpstreamQuotaCache 创建拼车周额度缓存。rdb 为 nil 时返回 nil：
// 调用方按未注入处理——缓存与刷新冷却整体跳过（每次 GET 都打到 cpr，refresh 无冷却）。
// 这只是优化层降级（不像公共池缓存承载硬约束），打 WARN 留痕即可。
func NewCarpoolUpstreamQuotaCache(rdb *redis.Client) service.CarpoolUpstreamQuotaCache {
	if rdb == nil {
		slog.Warn("carpool upstream quota cache disabled: redis is not configured; " +
			"GET always queries cpr and refresh has no server-side cooldown")
		return nil
	}
	return &carpoolUpstreamQuotaCache{rdb: rdb}
}

func (c *carpoolUpstreamQuotaCache) Get(ctx context.Context, carpoolID int64) (*service.CarpoolUpstreamQuotaCacheEntry, bool, error) {
	raw, err := c.rdb.Get(ctx, carpoolUpstreamQuotaKey(carpoolID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var entry service.CarpoolUpstreamQuotaCacheEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		return nil, false, fmt.Errorf("decode carpool upstream quota cache: %w", err)
	}
	return &entry, true, nil
}

func (c *carpoolUpstreamQuotaCache) Set(ctx context.Context, carpoolID int64, entry *service.CarpoolUpstreamQuotaCacheEntry, ttl time.Duration) error {
	raw, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode carpool upstream quota cache: %w", err)
	}
	return c.rdb.Set(ctx, carpoolUpstreamQuotaKey(carpoolID), raw, ttl).Err()
}

// TryAcquireRefreshCooldown 用 SET NX EX 抢占本车的刷新冷却窗口。
// 返回 false 表示冷却中（已有窗口未过期）。
func (c *carpoolUpstreamQuotaCache) TryAcquireRefreshCooldown(ctx context.Context, carpoolID int64, cooldown time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, carpoolUpstreamQuotaRefreshKey(carpoolID),
		carpoolUpstreamQuotaRefreshKeyValue, cooldown).Result()
}

// ReleaseRefreshCooldown 删除本车的刷新冷却窗口（刷新失败归还）。
func (c *carpoolUpstreamQuotaCache) ReleaseRefreshCooldown(ctx context.Context, carpoolID int64) error {
	return c.rdb.Del(ctx, carpoolUpstreamQuotaRefreshKey(carpoolID)).Err()
}

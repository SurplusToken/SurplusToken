package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	// carpoolCommonsKeyPrefix 拼车组级公共池计数器 key 前缀。
	// 完整 key：carpool:commons:{groupID}:{windowStartUnix}——窗口起点来自订阅的
	// weekly_window_start（发车对齐 + 周重置网格吸附保证全车一致），窗口轮换时
	// key 自然轮换，旧 key 由 TTL 回收。
	carpoolCommonsKeyPrefix = "carpool:commons:"
	// carpoolCommonsKeyTTL 覆盖 7 天周窗口 + 计费滞后（请求完成后才累加）的余量。
	carpoolCommonsKeyTTL = 8 * 24 * time.Hour
)

// carpoolCommonsRebuildScript 在 key 缺失（或落后于重算值）时把它补成 DB 重算值。
//
// 只在 GET miss 时执行。取 max(现值, 重算值) 而不是无脑覆盖：重算值来自 DB 快照，
// 若在"读 DB"与"执行脚本"之间有并发 INCRBYFLOAT 落地，现值可能已包含快照之后的
// 用量；取大值宁可略微高估，也不会把已记的公共池消耗抹掉（抹掉 = 全车凭空多一个
// 公共池，正是本重建机制要防的事）。
var carpoolCommonsRebuildScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
local rebuilt = tonumber(ARGV[1])
if current == false or tonumber(current) < rebuilt then
    redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
    return ARGV[1]
end
return current
`)

func carpoolCommonsKey(groupID int64, windowStart time.Time) string {
	return fmt.Sprintf("%s%d:%d", carpoolCommonsKeyPrefix, groupID, windowStart.Unix())
}

// carpoolCommonsCache 基于 Redis 的拼车组级公共池计数器（service.CarpoolCommonsCounter）。
// 累加用 INCRBYFLOAT 原子完成；读取是单点 GET（预检查语义，见接口注释），
// GET 未命中时从 DB 重算并回填（见 GetCommonsUsage）。
type carpoolCommonsCache struct {
	rdb *redis.Client
	db  *sql.DB
}

// NewCarpoolCommonsCache 创建拼车公共池计数器。
//
// rdb 为 nil 时返回 nil：调用方按未注入处理，公共池强制被整体跳过（保底仍然生效，
// 但"全车合计不超过整车周限额"的硬约束会失效）。这不是可以静默接受的降级，
// 所以这里直接打 ERROR——部署缺 Redis 时至少在日志里留下明确痕迹。
func NewCarpoolCommonsCache(rdb *redis.Client, db *sql.DB) service.CarpoolCommonsCounter {
	if rdb == nil {
		slog.Error("carpool commons counter disabled: redis is not configured; " +
			"shared-pool enforcement is OFF (per-member reserves still hold, " +
			"but total group usage is no longer capped at the upstream weekly limit)")
		return nil
	}
	return &carpoolCommonsCache{rdb: rdb, db: db}
}

// GetCommonsUsage 读取指定组在指定周窗口内已消耗的公共池用量（USD）。
//
// key 不存在有两种可能，必须区分：
//   - 本窗口确实还没产生任何超额消耗 → 正确答案是 0；
//   - Redis 重启 / FLUSH / maxmemory 驱逐把计数器弄丢了 → 返回 0 会让全车凭空
//     再得一个完整的公共池 C，"保底是硬保证"当场失效。
//
// 二者在 Redis 侧无法分辨，所以 miss 一律回到 DB 重算：公共池用量的定义
// Σ max(0, weekly_usage_usd − weekly_reserved_usd) 完全可以从订阅行还原。
// 重算失败时退回 0（fail-open，与整体降级策略一致）。
func (c *carpoolCommonsCache) GetCommonsUsage(ctx context.Context, groupID int64, windowStart time.Time) (float64, error) {
	key := carpoolCommonsKey(groupID, windowStart)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == nil {
		used, parseErr := strconv.ParseFloat(val, 64)
		if parseErr != nil {
			return 0, fmt.Errorf("parse carpool commons usage: %w", parseErr)
		}
		return used, nil
	}
	if !errors.Is(err, redis.Nil) {
		return 0, err
	}
	return c.rebuildCommonsUsage(ctx, key, groupID, windowStart)
}

// rebuildCommonsUsage 从订阅表重算本组本窗口的公共池已用量并回填计数器。
func (c *carpoolCommonsCache) rebuildCommonsUsage(ctx context.Context, key string, groupID int64, windowStart time.Time) (float64, error) {
	if c.db == nil {
		// 没有 DB 句柄就无法区分"真的是 0"和"计数器丢了"，只能按 0 处理。
		return 0, nil
	}
	var rebuilt float64
	err := c.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(GREATEST(0, us.weekly_usage_usd - us.weekly_reserved_usd)), 0)
FROM user_subscriptions us
WHERE us.group_id = $1
  AND us.weekly_window_start = $2
  AND us.weekly_reserved_usd IS NOT NULL
  AND us.deleted_at IS NULL`, groupID, windowStart).Scan(&rebuilt)
	if err != nil {
		slog.Error("carpool commons rebuild failed; shared-pool check degrades to fail-open",
			"group_id", groupID, "window_start", windowStart, "error", err)
		return 0, nil
	}
	if rebuilt <= 0 {
		// 本窗口确实没有超额消耗：不写 key，让它保持"不存在"，
		// 下一次真正的 INCRBYFLOAT 自然创建它。
		return 0, nil
	}
	res, err := carpoolCommonsRebuildScript.Run(ctx, c.rdb, []string{key},
		strconv.FormatFloat(rebuilt, 'f', -1, 64),
		int64(carpoolCommonsKeyTTL/time.Second)).Result()
	if err != nil {
		// 回填失败不影响本次判定：重算值本身就是正确答案。
		slog.Warn("carpool commons rebuild write-back failed",
			"group_id", groupID, "window_start", windowStart, "error", err)
		return rebuilt, nil
	}
	slog.Warn("carpool commons counter was missing and has been rebuilt from subscriptions",
		"group_id", groupID, "window_start", windowStart, "rebuilt_usd", rebuilt)
	if s, ok := res.(string); ok {
		if merged, parseErr := strconv.ParseFloat(s, 64); parseErr == nil {
			return merged, nil
		}
	}
	return rebuilt, nil
}

func (c *carpoolCommonsCache) AddCommonsUsage(ctx context.Context, groupID int64, windowStart time.Time, delta float64) error {
	if delta == 0 {
		return nil
	}
	key := carpoolCommonsKey(groupID, windowStart)
	pipe := c.rdb.Pipeline()
	pipe.IncrByFloat(ctx, key, delta)
	pipe.Expire(ctx, key, carpoolCommonsKeyTTL)
	_, err := pipe.Exec(ctx)
	return err
}

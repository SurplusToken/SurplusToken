package repository

import (
	"context"
	"errors"
	"fmt"
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

func carpoolCommonsKey(groupID int64, windowStart time.Time) string {
	return fmt.Sprintf("%s%d:%d", carpoolCommonsKeyPrefix, groupID, windowStart.Unix())
}

// carpoolCommonsCache 基于 Redis 的拼车组级公共池计数器（service.CarpoolCommonsCounter）。
// 累加用 INCRBYFLOAT 原子完成；读取是单点 GET（预检查语义，见接口注释）。
type carpoolCommonsCache struct {
	rdb *redis.Client
}

// NewCarpoolCommonsCache 创建拼车公共池计数器。rdb 为 nil 时返回 nil
// （调用方按未注入处理：跳过公共池强制，行为退化为仅个人限额）。
func NewCarpoolCommonsCache(rdb *redis.Client) service.CarpoolCommonsCounter {
	if rdb == nil {
		return nil
	}
	return &carpoolCommonsCache{rdb: rdb}
}

func (c *carpoolCommonsCache) GetCommonsUsage(ctx context.Context, groupID int64, windowStart time.Time) (float64, error) {
	val, err := c.rdb.Get(ctx, carpoolCommonsKey(groupID, windowStart)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	used, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("parse carpool commons usage: %w", err)
	}
	return used, nil
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

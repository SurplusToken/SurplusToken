package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ErrCarpoolSharedPoolExhausted 表示拼车组级公共池在本周窗口内已耗尽。
// 保底额度（weekly_reserved_usd）内的用量不受影响——保底放行不依赖公共池计数器。
var ErrCarpoolSharedPoolExhausted = infraerrors.TooManyRequests("CARPOOL_SHARED_POOL_EXHAUSTED", "carpool shared pool exhausted for this week")

// CarpoolCommonsUsageDelta 是一次计费提交后应计入组级公共池计数器的增量
// （仅成员用量越过保底 r 的部分占用公共池）。WindowStart 取用量落账时订阅的
// weekly_window_start——发车对齐 + 周重置网格吸附保证全车成员恒等。
type CarpoolCommonsUsageDelta struct {
	GroupID     int64
	WindowStart time.Time
	DeltaUSD    float64
}

// CarpoolCommonsCounter 是拼车组级公共池用量计数器（设计文档 §4.2，v3.2）。
//
// 语义：
//   - 计数器按 (group, 周窗口起点) 统计全车超额用量之和 Σ max(0, usage_i − r_i)；
//   - 限额检查为预检查（读计数器 < C 才放行），与既有 CheckWeeklyLimit 同属
//     TOCTOU 语义：并发 in-flight 请求可以带来有限超卖，不要求绝对为零；
//   - 实现要求原子累加（Redis INCRBYFLOAT），读取失败时调用方按 fail-open 处理
//     （保底硬保证不依赖本计数器，仅公共池强制在故障期间降级）。
type CarpoolCommonsCounter interface {
	// GetCommonsUsage 读取指定组在指定周窗口内已消耗的公共池用量（USD）。
	// 计数器不存在（窗口未产生超额消耗）时必须返回 0, nil。
	GetCommonsUsage(ctx context.Context, groupID int64, windowStart time.Time) (float64, error)
	// AddCommonsUsage 原子累加公共池用量。delta 通常为正（见
	// CarpoolCommonsExcessDelta）；调用方保证只在计费成功提交后调用。
	AddCommonsUsage(ctx context.Context, groupID int64, windowStart time.Time, delta float64) error
}

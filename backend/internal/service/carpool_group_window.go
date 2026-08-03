package service

import (
	"context"
	"time"
)

// CarpoolGroupWindowResetter 把整辆车的周窗口原子地重锚到同一个起点。
//
// 为什么必须整组一起做：
//
// 窗口起点是公共池计数器 key 的一部分（carpool:commons:{group}:{windowStart}），
// 设计前提是全车共用一个值。原来的实现是逐订阅懒重锚——谁发请求谁才重锚，
// 而上游的 codex_7d_reset_at 每次刷新都有秒级抖动，于是每个人在自己那次请求的
// 瞬间存下一个略微不同的起点。线上实测后果：
//
//   - 一辆 7 人的车分散在 6 个窗口上，Redis 里出现 3 个互不相干的公共池计数器，
//     "全车合计不超过整车周限额"这道硬约束事实上失效；
//   - 不发请求的成员永远不重锚，他的计费周期横跨了 4 天。
//
// 整组原子重锚把这两件事一并消掉：一次算出一个值，一个事务里写给所有人。
//
// 实现方由 CarpoolUpstreamWindowSource 兼任（同一个仓储对象持有 *sql.DB），
// 通过类型断言取用——与 CarpoolObservedCapacitySource 同一套路，不新增装配点。
type CarpoolGroupWindowResetter interface {
	// ResetGroupWeeklyWindow 在一个事务里给全组落台账并重锚窗口。
	//
	// tolerance 用于并发去重：若组内当前最新的窗口起点与 target 相差不超过
	// tolerance，视为已经有人重锚过，直接返回 Applied=false 不做任何写入。
	// 没有这道判断的话，两个并发请求会各自算出相差几秒的 target，
	// 后到的那个会把刚重置好的窗口再重置一遍，把用量二次清零。
	ResetGroupWeeklyWindow(ctx context.Context, groupID int64, target time.Time, tolerance time.Duration) (*CarpoolGroupWindowReset, error)
}

// CarpoolGroupWindowReset 是一次整组重锚的结果。
type CarpoolGroupWindowReset struct {
	// Applied 为 false 表示本次没有写入（组内窗口已在容差内）。
	Applied bool
	// From/To 是重锚前后的窗口起点，仅在 Applied 时有意义。
	From time.Time
	To   time.Time
	// UserIDs 是被重锚的成员，供调用方逐个失效订阅缓存——
	// 缓存里留着旧窗口和旧用量的话，重锚等于没做。
	UserIDs []int64
	// Cycles 是本次落库的计费周期条数，用于日志核对。
	Cycles int
}

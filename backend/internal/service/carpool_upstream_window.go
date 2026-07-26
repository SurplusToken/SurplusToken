package service

import (
	"context"
	"time"
)

const (
	// CarpoolUpstreamWindowMaxAge 是上游窗口快照的最大可信年龄。
	//
	// codex_7d_* 只在有请求真的打到上游时才刷新，车闲置就会陈旧。超过这个年龄
	// 就不再据此重锚——宁可退回本地 7 天网格，也不要拿一份过期的重置时刻去
	// 清空全车用量。
	CarpoolUpstreamWindowMaxAge = 36 * time.Hour

	// CarpoolUpstreamWindowTolerance 是判定"窗口起点变了"的容差。
	//
	// 上游给的是 reset-after-seconds（整秒），我们用本地时钟加出绝对时刻，
	// 每次请求算出来都会差几秒。没有容差的话每个请求都会被当成一次上游重置，
	// 全车用量会被反复清零。
	CarpoolUpstreamWindowTolerance = 10 * time.Minute
)

// CarpoolUpstreamWindow 是拼车组绑定的上游账号当前周窗口的快照。
type CarpoolUpstreamWindow struct {
	// Start/End 是上游周窗口的起止时刻。End 即上游 codex_7d_reset_at，
	// Start = End − 窗口长度。
	Start time.Time
	End   time.Time
	// ObservedAt 是这份数据最后一次从上游响应头刷新的时刻（codex_usage_updated_at），
	// 用于判断是否已经陈旧。
	ObservedAt time.Time
}

// Fresh 报告快照是否新到可以用来重锚全车窗口。
func (w *CarpoolUpstreamWindow) Fresh(now time.Time) bool {
	if w == nil || w.Start.IsZero() || w.End.IsZero() {
		return false
	}
	if !w.End.After(w.Start) {
		return false
	}
	return now.Sub(w.ObservedAt) <= CarpoolUpstreamWindowMaxAge
}

// CarpoolUpstreamWindowSource 提供拼车组对应上游账号的周窗口。
//
// 拼车的"一周"本该等于 OpenAI 的一周：上游在自己的时刻重置，可能与我们发车
// 那天的 7 天网格完全对不上。对不上时两头都出错——上游先重置，我们的计数器
// 还压着上一周的用量，用户被自己人挡在门外；我们先重置，全车拿到新的保底和
// 公共池，而上游快满了，于是集体撞 429、保底承诺当场失效。
type CarpoolUpstreamWindowSource interface {
	// GroupUpstreamWeeklyWindow 返回该组绑定账号的上游周窗口；
	// 组没有绑定账号、或账号上没有上游用量数据时返回 nil, nil。
	GroupUpstreamWeeklyWindow(ctx context.Context, groupID int64) (*CarpoolUpstreamWindow, error)
}

// CarpoolWeeklyWindowTarget 计算拼车订阅此刻应处的周窗口起点。
//
// 优先跟随上游真实窗口；上游数据缺失或陈旧时退回原来的 7 天网格（发车日锚定），
// 保证降级后全车仍然一致。第二个返回值表示是否采信了上游。
func CarpoolWeeklyWindowTarget(prev time.Time, upstream *CarpoolUpstreamWindow, now time.Time) (time.Time, bool) {
	if upstream.Fresh(now) {
		// 上游窗口起点是全车共享的外部事实，各成员各自算也必然一致——
		// 这正是原来那个 7 天网格想达到的效果，只是锚对了地方。
		return upstream.Start, true
	}
	return CarpoolWeeklyWindowGridStart(prev, now), false
}

// CarpoolWeeklyWindowDrifted 报告订阅记录的窗口起点是否已经偏离目标窗口，
// 即需要重锚 + 清零本周用量。容差见 CarpoolUpstreamWindowTolerance。
func CarpoolWeeklyWindowDrifted(current, target time.Time) bool {
	if current.IsZero() || target.IsZero() {
		return false
	}
	diff := target.Sub(current)
	if diff < 0 {
		diff = -diff
	}
	return diff > CarpoolUpstreamWindowTolerance
}

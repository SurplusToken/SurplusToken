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

	// CarpoolUpstreamWindowMinLength / MaxLength 界定"这看起来像一个周窗口"。
	//
	// 上游的 codex_7d_* 并不总是 7 天：生产数据里就有账号报 43800 分钟
	// （30.4 天）和 43200 分钟（30 天）——那是月度套餐，不是周套餐。把 30 天
	// 当成一周会让 weekly_window_start 退到一个月前，全车的周用量整月不重置，
	// 成员撞到保底后会被卡一个月。超出这个区间就不采信，退回本地 7 天网格。
	CarpoolUpstreamWindowMinLength = 24 * time.Hour
	CarpoolUpstreamWindowMaxLength = 14 * 24 * time.Hour

	// CarpoolUpstreamWindowTolerance 是判定"窗口起点变了"的容差。
	//
	// 上游给的是 reset-after-seconds（整秒），我们用本地时钟加出绝对时刻，
	// 每次请求算出来都会差几秒。没有容差的话每个请求都会被当成一次上游重置，
	// 全车用量会被反复清零。
	CarpoolUpstreamWindowTolerance = 10 * time.Minute

	// CarpoolUpstreamWindowMinAdvance 是"这确实是一个新窗口"所需的最小前移量。
	//
	// 10 分钟的容差不够。生产事故实测：同一个窗口内 codex_7d_reset_at 一度读到
	// 比真实值晚 12 分 43 秒的数值，随后又读回原值。那次抖动越过容差，被当成一次
	// 上游重置，整组 11 人的周用量被清零（2213.73 USD 的记录被截成一个伪周期），
	// 上游读回原值后又把成员逐个向回搬，写出 cycle_end < cycle_start 的倒挂周期。
	//
	// 抖动是分钟级的，真实的新窗口至少前移数天。取 1 小时：比观测到的抖动大近
	// 五倍，又远小于任何一个可信窗口长度（下限 24 小时），两头都不会误判。
	CarpoolUpstreamWindowMinAdvance = time.Hour
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
	// UsedPercent 是上游报告的本窗口已用百分比（0–100）。
	// 与全车美元用量一起可反推整车真实容量，见 CarpoolObservedTotalCapacityUSD。
	UsedPercent float64
}

// usablePeriod 验证快照是否足以确定上游周期边界。
func (w *CarpoolUpstreamWindow) usablePeriod(now time.Time) (time.Duration, bool) {
	if w == nil || w.Start.IsZero() || w.End.IsZero() {
		return 0, false
	}
	if !w.End.After(w.Start) {
		return 0, false
	}
	// 长度得像个周窗口。月度套餐的账号会报 30 天，拿它当"一周"会让全车
	// 的周用量整月不重置（见常量注释）。
	length := w.End.Sub(w.Start)
	if length < CarpoolUpstreamWindowMinLength || length > CarpoolUpstreamWindowMaxLength {
		return 0, false
	}
	if w.ObservedAt.IsZero() || now.Sub(w.ObservedAt) > CarpoolUpstreamWindowMaxAge {
		return 0, false
	}
	return length, true
}

// Fresh 报告快照是否仍描述当前尚未结束的窗口。已越过 End 的快照可以用其
// 周期边界推导新窗口起点，但旧窗口的 UsedPercent 不能再用于新窗口容量估算。
func (w *CarpoolUpstreamWindow) Fresh(now time.Time) bool {
	_, ok := w.usablePeriod(now)
	return ok && w.End.After(now)
}

// CurrentStartAt 返回 now 所处的上游周期起点。若最后一次已知 reset_at 已经过期，
// 按可信窗口长度向前推进；这样共享池在上游重置后无需先成功打出一个请求来刷新
// 快照，避免“本地先拦截 → 永远无法刷新上游快照”的闭环死锁。
func (w *CarpoolUpstreamWindow) CurrentStartAt(now time.Time) (time.Time, bool) {
	length, ok := w.usablePeriod(now)
	if !ok {
		return time.Time{}, false
	}
	if now.Before(w.End) {
		return w.Start, true
	}
	periodsAfterEnd := now.Sub(w.End) / length
	return w.End.Add(periodsAfterEnd * length), true
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
	if currentStart, ok := upstream.CurrentStartAt(now); ok {
		// 上游窗口起点是全车共享的外部事实，各成员各自算也必然一致——
		// 这正是原来那个 7 天网格想达到的效果，只是锚对了地方。
		return currentStart, true
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

// CarpoolWeeklyWindowAdvanced 报告目标窗口是否真的开启了新的一周，
// 即"可以重锚 + 清零"。这是重锚的唯一判据，比单纯的偏离更严格：
//
//   - 只认前移。目标早于当前窗口意味着上游数据回退（抖动、缓存、时钟），
//     跟着往回搬会把已经结算过的一周重新打开，并写出 cycle_end < cycle_start
//     的倒挂周期——生产上真的发生过。
//   - 前移量必须够大。分钟级的前移是抖动不是新窗口，见 MinAdvance 的注释。
func CarpoolWeeklyWindowAdvanced(current, target time.Time) bool {
	if current.IsZero() || target.IsZero() {
		return false
	}
	return target.Sub(current) >= CarpoolUpstreamWindowMinAdvance
}

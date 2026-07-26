package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func upstreamWindow(start time.Time, length time.Duration, observed time.Time) *CarpoolUpstreamWindow {
	return &CarpoolUpstreamWindow{Start: start, End: start.Add(length), ObservedAt: observed}
}

// 新鲜的上游窗口直接采信：拼车的"一周"应当等于 OpenAI 的一周。
func TestCarpoolWeeklyWindowTargetFollowsUpstream(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	// 发车锚在 7/1，但上游的窗口其实是 7/18 开始的
	prev := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	up := upstreamWindow(time.Date(2026, 7, 18, 3, 0, 0, 0, time.UTC), 7*24*time.Hour, now.Add(-time.Hour))

	target, fromUpstream := CarpoolWeeklyWindowTarget(prev, up, now)
	require.True(t, fromUpstream)
	require.Equal(t, up.Start, target)
}

// 上游数据陈旧时退回本地 7 天网格——宁可用旧规则，也不要拿一份过期的
// 重置时刻去清空全车用量。
func TestCarpoolWeeklyWindowTargetFallsBackWhenStale(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	prev := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	stale := upstreamWindow(time.Date(2026, 7, 18, 3, 0, 0, 0, time.UTC), 7*24*time.Hour,
		now.Add(-CarpoolUpstreamWindowMaxAge-time.Minute))

	target, fromUpstream := CarpoolWeeklyWindowTarget(prev, stale, now)
	require.False(t, fromUpstream)
	require.Equal(t, CarpoolWeeklyWindowGridStart(prev, now), target)
}

// 完全没有上游数据（组没绑账号 / 还没被响应头刷新过）时同样退回网格。
func TestCarpoolWeeklyWindowTargetWithoutUpstream(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	prev := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	target, fromUpstream := CarpoolWeeklyWindowTarget(prev, nil, now)
	require.False(t, fromUpstream)
	require.Equal(t, CarpoolWeeklyWindowGridStart(prev, now), target)
}

// 残缺的上游快照（缺起点/终点、或终点不晚于起点）不得采信。
func TestCarpoolUpstreamWindowFreshRejectsMalformed(t *testing.T) {
	now := time.Now()
	require.False(t, (*CarpoolUpstreamWindow)(nil).Fresh(now))
	require.False(t, (&CarpoolUpstreamWindow{ObservedAt: now}).Fresh(now))
	require.False(t, (&CarpoolUpstreamWindow{
		Start: now, End: now.Add(-time.Hour), ObservedAt: now,
	}).Fresh(now), "终点早于起点是脏数据")
	require.True(t, (&CarpoolUpstreamWindow{
		Start: now.Add(-24 * time.Hour), End: now.Add(24 * time.Hour), ObservedAt: now,
	}).Fresh(now))
}

// 漂移判定要有容差：上游给的是整秒的 reset-after，本地每次算出的绝对时刻
// 会差几秒；没有容差的话每个请求都被当成一次重置，全车用量会被反复清零。
func TestCarpoolWeeklyWindowDriftedTolerance(t *testing.T) {
	base := time.Date(2026, 7, 18, 3, 0, 0, 0, time.UTC)

	require.False(t, CarpoolWeeklyWindowDrifted(base, base.Add(9*time.Second)),
		"秒级抖动不算漂移")
	require.False(t, CarpoolWeeklyWindowDrifted(base, base.Add(-CarpoolUpstreamWindowTolerance+time.Second)))
	require.True(t, CarpoolWeeklyWindowDrifted(base, base.Add(7*24*time.Hour)),
		"上游整整前移一个窗口，必须判定为漂移")
	require.True(t, CarpoolWeeklyWindowDrifted(base, base.Add(-2*time.Hour)),
		"向后漂移同样要纠正")

	// 零值不参与判定，避免未初始化窗口触发误重置。
	require.False(t, CarpoolWeeklyWindowDrifted(time.Time{}, base))
	require.False(t, CarpoolWeeklyWindowDrifted(base, time.Time{}))
}

// 上游并不总是报 7 天窗口：生产数据里有账号报 30 天（月度套餐）。
// 把 30 天当成"一周"会让全车周用量整月不重置，成员撞到保底后卡一个月。
func TestCarpoolUpstreamWindowRejectsNonWeeklyLength(t *testing.T) {
	now := time.Now()
	mk := func(minutes float64) *CarpoolUpstreamWindow {
		end := now.Add(time.Hour)
		return &CarpoolUpstreamWindow{
			Start:      end.Add(-time.Duration(minutes) * time.Minute),
			End:        end,
			ObservedAt: now,
		}
	}

	require.True(t, mk(10080).Fresh(now), "10080 分钟 = 7 天，正常周窗口")
	require.False(t, mk(43800).Fresh(now), "43800 分钟 = 30.4 天，月度套餐，不能当一周")
	require.False(t, mk(43200).Fresh(now), "43200 分钟 = 30 天，同上")
	require.False(t, mk(60).Fresh(now), "1 小时太短，多半是脏数据")

	// 边界
	require.True(t, mk(24*60).Fresh(now), "1 天为下界，含")
	require.True(t, mk(14*24*60).Fresh(now), "14 天为上界，含")
	require.False(t, mk(24*60-1).Fresh(now))
	require.False(t, mk(14*24*60+1).Fresh(now))
}

// 长度不合格时目标窗口退回本地 7 天网格，全车依然一致。
func TestCarpoolWeeklyWindowTargetFallsBackOnMonthlyWindow(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	prev := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	monthly := upstreamWindow(now.AddDate(0, 0, -30), 30*24*time.Hour, now)

	target, fromUpstream := CarpoolWeeklyWindowTarget(prev, monthly, now)
	require.False(t, fromUpstream)
	require.Equal(t, CarpoolWeeklyWindowGridStart(prev, now), target)
}

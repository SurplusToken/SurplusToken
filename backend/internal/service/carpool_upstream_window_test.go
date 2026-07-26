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

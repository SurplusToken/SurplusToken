package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 复现生产事故的数值：同一个窗口内 codex_7d_reset_at 一度读到比真实值晚
// 12 分 43 秒的数值，随后读回原值。旧判据（±10 分钟容差、双向都算漂移）
// 把这次抖动当成了新窗口，整组 11 人的周用量被清零。
const incidentJitter = 12*time.Minute + 43*time.Second

func TestCarpoolWeeklyWindowAdvancedRejectsUpstreamJitter(t *testing.T) {
	current := time.Date(2026, 8, 1, 11, 32, 55, 0, time.UTC)

	// 事故当天的正向抖动：不能被当成新窗口
	require.False(t, CarpoolWeeklyWindowAdvanced(current, current.Add(incidentJitter)),
		"12 分 43 秒的正向抖动被误判成新窗口——这正是把 11 人用量清零的那次")

	// 旧判据会放行，用它对照说明为什么必须换
	require.True(t, CarpoolWeeklyWindowDrifted(current, current.Add(incidentJitter)),
		"旧的双向容差判据确实会放行这次抖动")

	// 秒级抖动同样不算
	require.False(t, CarpoolWeeklyWindowAdvanced(current, current.Add(3*time.Second)))

	// 回退绝不能触发重锚：跟着往回搬会把已结算的一周重新打开，
	// 并写出 cycle_end < cycle_start 的倒挂周期。
	require.False(t, CarpoolWeeklyWindowAdvanced(current, current.Add(-incidentJitter)),
		"窗口回退被当成了需要重锚")
	require.False(t, CarpoolWeeklyWindowAdvanced(current, current.Add(-7*24*time.Hour)))

	// 真正的新窗口（上游提前重置，前移数天）必须放行
	require.True(t, CarpoolWeeklyWindowAdvanced(current, current.Add(3*24*time.Hour)))
	require.True(t, CarpoolWeeklyWindowAdvanced(current, current.Add(7*24*time.Hour)))

	// 边界：恰好等于最小前移量应当放行
	require.True(t, CarpoolWeeklyWindowAdvanced(current, current.Add(CarpoolUpstreamWindowMinAdvance)))
	require.False(t, CarpoolWeeklyWindowAdvanced(current, current.Add(CarpoolUpstreamWindowMinAdvance-time.Second)))

	// 零值不参与判定
	require.False(t, CarpoolWeeklyWindowAdvanced(time.Time{}, current))
	require.False(t, CarpoolWeeklyWindowAdvanced(current, time.Time{}))
}

// 最小前移量必须明显大于观测到的抖动，又明显小于最短的可信窗口，
// 否则两头都会误判。这条断言把这个区间钉死，防止有人随手调小。
func TestCarpoolUpstreamWindowMinAdvanceSanity(t *testing.T) {
	require.Greater(t, CarpoolUpstreamWindowMinAdvance, incidentJitter,
		"最小前移量不大于实测抖动，事故会重演")
	require.Less(t, CarpoolUpstreamWindowMinAdvance, CarpoolUpstreamWindowMinLength,
		"最小前移量超过了最短可信窗口，真实重置会被漏掉")
}

// 「整组重锚拒绝执行」绝不能退回单订阅路径——单订阅路径没有全组视野，
// 会去做整组重锚刚刚拒绝的事。生产上因此写出了 6 条倒挂周期。
func TestDeclinedGroupResetDoesNotFallBack(t *testing.T) {
	target := time.Now()
	stub := &groupResetterStub{result: &CarpoolGroupWindowReset{Applied: false}}
	svc := &SubscriptionService{upstreamWindows: stub}

	outcome := svc.resetCarpoolGroupWindow(context.Background(), carpoolSub(target.Add(-72*time.Hour)), target)

	require.Equal(t, carpoolGroupResetDeclined, outcome,
		"拒绝必须与不支持区分开；返回 Unavailable 会让调用方退回单订阅路径")
	require.NotEqual(t, carpoolGroupResetUnavailable, outcome)
}

// 三种结局互不混淆
func TestGroupResetOutcomes(t *testing.T) {
	target := time.Now()
	sub := carpoolSub(target.Add(-72 * time.Hour))
	ctx := context.Background()

	applied := &SubscriptionService{upstreamWindows: &groupResetterStub{
		result: &CarpoolGroupWindowReset{Applied: true, UserIDs: []int64{1}},
	}}
	require.Equal(t, carpoolGroupResetApplied, applied.resetCarpoolGroupWindow(ctx, sub, target))

	errored := &SubscriptionService{upstreamWindows: &groupResetterStub{err: context.DeadlineExceeded}}
	require.Equal(t, carpoolGroupResetUnavailable, errored.resetCarpoolGroupWindow(ctx, sub, target))

	unsupported := &SubscriptionService{upstreamWindows: &upstreamOnlyStub{}}
	require.Equal(t, carpoolGroupResetUnavailable, unsupported.resetCarpoolGroupWindow(ctx, sub, target))

	unwired := &SubscriptionService{}
	require.Equal(t, carpoolGroupResetUnavailable, unwired.resetCarpoolGroupWindow(ctx, sub, target))
}

// 传给仓储的去重阈值必须是最小前移量本身，否则「判定要重锚、仓储又说不用」
// 会形成每次请求都尝试一遍的空转。
func TestGroupResetPassesMinAdvance(t *testing.T) {
	target := time.Now()
	stub := &groupResetterStub{result: &CarpoolGroupWindowReset{Applied: true}}
	svc := &SubscriptionService{upstreamWindows: stub}

	svc.resetCarpoolGroupWindow(context.Background(), carpoolSub(target.Add(-72*time.Hour)), target)
	require.Equal(t, CarpoolUpstreamWindowMinAdvance, stub.gotTol)
}

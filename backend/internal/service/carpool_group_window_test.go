package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// groupResetterStub 同时实现 CarpoolUpstreamWindowSource 与 CarpoolGroupWindowResetter，
// 模拟真实仓储对象（同一个对象兼任两个角色，靠类型断言取用）。
type groupResetterStub struct {
	window *CarpoolUpstreamWindow

	calls     int
	gotGroup  int64
	gotTarget time.Time
	gotTol    time.Duration
	result    *CarpoolGroupWindowReset
	err       error
}

func (s *groupResetterStub) GroupUpstreamWeeklyWindow(ctx context.Context, groupID int64) (*CarpoolUpstreamWindow, error) {
	return s.window, nil
}

func (s *groupResetterStub) ResetGroupWeeklyWindow(ctx context.Context, groupID int64, target time.Time, tol time.Duration) (*CarpoolGroupWindowReset, error) {
	s.calls++
	s.gotGroup = groupID
	s.gotTarget = target
	s.gotTol = tol
	return s.result, s.err
}

// upstreamOnlyStub 只实现窗口查询，不实现整组重锚——用于验证退回单订阅路径。
type upstreamOnlyStub struct{ window *CarpoolUpstreamWindow }

func (s *upstreamOnlyStub) GroupUpstreamWeeklyWindow(ctx context.Context, groupID int64) (*CarpoolUpstreamWindow, error) {
	return s.window, nil
}

func carpoolSub(windowStart time.Time) *UserSubscription {
	reserved := 240.0
	return &UserSubscription{
		ID:                7,
		UserID:            11,
		GroupID:           62,
		WeeklyReservedUSD: &reserved,
		WeeklyWindowStart: &windowStart,
		WeeklyUsageUSD:    500,
	}
}

// 整组重锚成功时必须真的被调用，且带上正确的组、目标窗口与去重容差。
func TestResetCarpoolGroupWindowAppliesToWholeGroup(t *testing.T) {
	target := time.Now().Add(-time.Hour).Truncate(time.Second)
	stub := &groupResetterStub{result: &CarpoolGroupWindowReset{
		Applied: true,
		From:    target.Add(-72 * time.Hour),
		To:      target,
		UserIDs: []int64{11, 22, 33},
		Cycles:  3,
	}}
	svc := &SubscriptionService{upstreamWindows: stub}

	outcome := svc.resetCarpoolGroupWindow(context.Background(), carpoolSub(target.Add(-72*time.Hour)), target)

	require.Equal(t, carpoolGroupResetApplied, outcome, "整组重锚已落库时调用方不应再走单订阅路径")
	require.Equal(t, 1, stub.calls)
	require.Equal(t, int64(62), stub.gotGroup)
	require.True(t, target.Equal(stub.gotTarget))
	// 阈值必须与重锚判定用同一个常量，否则会出现"判定要重锚、仓储又说不用"的空转
	require.Equal(t, CarpoolUpstreamWindowMinAdvance, stub.gotTol)
}

// Applied=false（并发下已有人重锚过）时必须退回单订阅路径，不能当成已完成。
func TestResetCarpoolGroupWindowFallsBackWhenNotApplied(t *testing.T) {
	target := time.Now()
	stub := &groupResetterStub{result: &CarpoolGroupWindowReset{Applied: false}}
	svc := &SubscriptionService{upstreamWindows: stub}

	require.NotEqual(t, carpoolGroupResetApplied, svc.resetCarpoolGroupWindow(context.Background(), carpoolSub(target.Add(-72*time.Hour)), target))
	require.Equal(t, 1, stub.calls)
}

// 仓储报错不能把用户请求挡住——退回单订阅路径，至少本人能重置。
func TestResetCarpoolGroupWindowFallsBackOnError(t *testing.T) {
	target := time.Now()
	stub := &groupResetterStub{err: context.DeadlineExceeded}
	svc := &SubscriptionService{upstreamWindows: stub}

	require.NotEqual(t, carpoolGroupResetApplied, svc.resetCarpoolGroupWindow(context.Background(), carpoolSub(target.Add(-72*time.Hour)), target))
}

// 窗口源没有实现整组重锚（旧实现）时，安静退回单订阅路径。
func TestResetCarpoolGroupWindowFallsBackWhenNotSupported(t *testing.T) {
	target := time.Now()
	svc := &SubscriptionService{upstreamWindows: &upstreamOnlyStub{}}
	require.NotEqual(t, carpoolGroupResetApplied, svc.resetCarpoolGroupWindow(context.Background(), carpoolSub(target.Add(-72*time.Hour)), target))
}

// 完全没注入窗口源时也不能 panic。
func TestResetCarpoolGroupWindowHandlesMissingWiring(t *testing.T) {
	target := time.Now()
	svc := &SubscriptionService{}
	require.NotEqual(t, carpoolGroupResetApplied, svc.resetCarpoolGroupWindow(context.Background(), carpoolSub(target.Add(-72*time.Hour)), target))
}

// 装配自检：两个可选注入缺失时必须能被查出来，而不是静默降级。
func TestCarpoolWiringReadyReportsMissingInjections(t *testing.T) {
	svc := &SubscriptionService{}
	upstream, recorder := svc.CarpoolWiringReady()
	require.False(t, upstream)
	require.False(t, recorder)

	svc.SetCarpoolUpstreamWindowSource(&upstreamOnlyStub{})
	upstream, recorder = svc.CarpoolWiringReady()
	require.True(t, upstream)
	require.False(t, recorder, "台账记录器仍未注入")
}

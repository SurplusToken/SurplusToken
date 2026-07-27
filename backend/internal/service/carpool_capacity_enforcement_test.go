package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// capacitySourceStub 返回预设的实测容量快照。
type capacitySourceStub struct {
	snapshot *CarpoolCapacitySnapshot
	err      error
	calls    int
}

func (s *capacitySourceStub) GroupObservedCapacity(ctx context.Context, groupID int64, windowStart time.Time) (*CarpoolCapacitySnapshot, error) {
	s.calls++
	return s.snapshot, s.err
}

func newCapacityTestService(counter CarpoolCommonsCounter, capacity CarpoolObservedCapacitySource) *SubscriptionService {
	billingCacheSvc := &BillingCacheService{cfg: &config.Config{}}
	billingCacheSvc.SetCarpoolCommonsCounter(counter)
	billingCacheSvc.SetCarpoolObservedCapacitySource(capacity)
	return NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, billingCacheSvc, nil, nil)
}

// 公共池容量为零时必须**拒绝**越过保底的用量，而不是跳过检查。
//
// 早先的实现在 capacity <= 0 时直接放行——当时容量只可能因参数配错而为零。
// 引入实测容量后它会随上游缩水动态跌到 Σr 以下，再跳过就等于"容量越紧、
// 约束越松"，正好反了。
func TestCommonsCheckRejectsWhenCapacityIsZero(t *testing.T) {
	counter := &fakeCarpoolCommonsCounter{used: 0}
	capacity := &capacitySourceStub{snapshot: &CarpoolCapacitySnapshot{
		ObservedTotalUSD: 1500, ReservedTotalUSD: 1920, CommonsUSD: 0,
		Trusted: true, Oversold: true,
	}}
	svc := newCapacityTestService(counter, capacity)

	// 周用量已达保底 → 接下来要吃公共池，而公共池容量为零
	sub := newCarpoolTestSub(91, 192, 480, 192)
	_, err := svc.ValidateAndCheckLimits(context.Background(), sub, carpoolTestGroup(91))
	require.ErrorIs(t, err, ErrCarpoolSharedPoolExhausted)
}

// 保底之内仍然无条件放行——容量缩水不得侵蚀已经承诺出去的保底。
func TestCommonsCheckStillHonoursReserveWhenOversold(t *testing.T) {
	counter := &fakeCarpoolCommonsCounter{used: 0}
	capacity := &capacitySourceStub{snapshot: &CarpoolCapacitySnapshot{
		ObservedTotalUSD: 1500, ReservedTotalUSD: 1920, CommonsUSD: 0,
		Trusted: true, Oversold: true,
	}}
	svc := newCapacityTestService(counter, capacity)

	sub := newCarpoolTestSub(91, 192, 480, 100) // 100 < 保底 192
	_, err := svc.ValidateAndCheckLimits(context.Background(), sub, carpoolTestGroup(91))
	require.NoError(t, err)
	require.Zero(t, capacity.calls, "保底内根本不该去问容量")
}

// 实测容量比发车时锁定的更小时，按实测执行（公共池被压缩）。
func TestCommonsCheckUsesObservedCapacityOverLaunchTime(t *testing.T) {
	// 发车时锁定 C=480；实测只有 180，而计数器已用 200
	counter := &fakeCarpoolCommonsCounter{used: 200}
	capacity := &capacitySourceStub{snapshot: &CarpoolCapacitySnapshot{
		ObservedTotalUSD: 2100, ReservedTotalUSD: 1920, CommonsUSD: 180, Trusted: true,
	}}
	svc := newCapacityTestService(counter, capacity)

	sub := newCarpoolTestSub(91, 192, 480, 250)
	_, err := svc.ValidateAndCheckLimits(context.Background(), sub, carpoolTestGroup(91))
	require.ErrorIs(t, err, ErrCarpoolSharedPoolExhausted,
		"按发车时的 480 会放行，按实测的 180 必须拒绝")
}

// 实测不可信时退回发车时锁定的容量，行为与引入实测前一致。
func TestCommonsCheckFallsBackToLaunchCapacityWhenUntrusted(t *testing.T) {
	counter := &fakeCarpoolCommonsCounter{used: 200}
	capacity := &capacitySourceStub{snapshot: &CarpoolCapacitySnapshot{Trusted: false}}
	svc := newCapacityTestService(counter, capacity)

	sub := newCarpoolTestSub(91, 192, 480, 250)
	_, err := svc.ValidateAndCheckLimits(context.Background(), sub, carpoolTestGroup(91))
	require.NoError(t, err, "锁定容量 480 > 已用 200，应放行")
}

// 实测查询失败同样退回锁定容量，不阻断请求。
func TestCommonsCheckFallsBackWhenCapacityLookupFails(t *testing.T) {
	counter := &fakeCarpoolCommonsCounter{used: 200}
	capacity := &capacitySourceStub{err: context.DeadlineExceeded}
	svc := newCapacityTestService(counter, capacity)

	sub := newCarpoolTestSub(91, 192, 480, 250)
	_, err := svc.ValidateAndCheckLimits(context.Background(), sub, carpoolTestGroup(91))
	require.NoError(t, err)
}

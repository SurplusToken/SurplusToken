package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newCapacityTestService(counter CarpoolCommonsCounter) *SubscriptionService {
	billingCacheSvc := &BillingCacheService{cfg: &config.Config{}}
	billingCacheSvc.SetCarpoolCommonsCounter(counter)
	return NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, billingCacheSvc, nil, nil)
}

// 公共池容量就是订阅上锁定的 C = 周限额 − 保底，边界两侧都要对。
//
// 这里不再有"实测反推"那一路：反推只在一辆车对应一个上游账号时成立，组里
// 有第二个账号时分子是全部账号的用量、分母只是其中一个账号的百分比，用得越多
// 推出的容量越大，限额等于自动放开。现在车的上限只由 weekly_limit_usd 决定。
func TestCommonsCapacityComesFromSubscriptionOnly(t *testing.T) {
	group := carpoolTestGroup(91)

	t.Run("已用量低于容量时放行", func(t *testing.T) {
		counter := &fakeCarpoolCommonsCounter{used: 479.99}
		svc := newCapacityTestService(counter)
		sub := newCarpoolTestSub(91, 192, 480, 250) // C = 480
		_, err := svc.ValidateAndCheckLimits(context.Background(), sub, group)
		require.NoError(t, err)
	})

	t.Run("已用量达到容量时拒绝", func(t *testing.T) {
		counter := &fakeCarpoolCommonsCounter{used: 480}
		svc := newCapacityTestService(counter)
		sub := newCarpoolTestSub(91, 192, 480, 250)
		_, err := svc.ValidateAndCheckLimits(context.Background(), sub, group)
		require.ErrorIs(t, err, ErrCarpoolSharedPoolExhausted)
	})
}

// 调小车周限额必须真的收紧公共池——这正是运营手上唯一的那个旋钮。
//
// 同一辆车、同样的保底、同样的已用量，只把周限额调小，原本放行的请求必须
// 转为拒绝。若这条挂了，说明容量又从别处（上游读数）取值，旋钮就失灵了。
func TestLoweringWeeklyLimitTightensCommons(t *testing.T) {
	group := carpoolTestGroup(91)
	const reserved, used = 1280.0, 300.0

	// 宽：C = 1600 − 1280 = 320 > 已用 300
	svc := newCapacityTestService(&fakeCarpoolCommonsCounter{used: used})
	_, err := svc.ValidateAndCheckLimits(context.Background(),
		newCarpoolTestSub(91, reserved, 320, reserved+10), group)
	require.NoError(t, err, "C=320 > 已用 300，应放行")

	// 紧：C = 1480 − 1280 = 200 < 已用 300
	svc = newCapacityTestService(&fakeCarpoolCommonsCounter{used: used})
	_, err = svc.ValidateAndCheckLimits(context.Background(),
		newCarpoolTestSub(91, reserved, 200, reserved+10), group)
	require.ErrorIs(t, err, ErrCarpoolSharedPoolExhausted, "C=200 < 已用 300，必须拒绝")
}

// 周限额被下调到 Σ保底 及以下时，公共池归零：越过保底的用量一律拒绝。
// 必须是拒绝而不是跳过检查，否则"限额越紧、约束越松"，正好反了。
func TestCommonsCheckRejectsWhenCapacityIsZero(t *testing.T) {
	counter := &fakeCarpoolCommonsCounter{used: 0}
	svc := newCapacityTestService(counter)

	sub := newCarpoolTestSub(91, 192, 0, 192) // 周限额 = 保底 → C = 0
	_, err := svc.ValidateAndCheckLimits(context.Background(), sub, carpoolTestGroup(91))
	require.ErrorIs(t, err, ErrCarpoolSharedPoolExhausted)
	require.Zero(t, counter.getCalls, "容量为零就该直接拒绝，不必再读计数器")
}

// 保底之内仍然无条件放行——限额下调不得侵蚀已经承诺出去的保底。
// 保底本身要跟着限额一起降（等比缩水），而不是靠这里的检查去削。
func TestCommonsCheckStillHonoursReserveWhenCapacityIsZero(t *testing.T) {
	counter := &fakeCarpoolCommonsCounter{used: 0}
	svc := newCapacityTestService(counter)

	sub := newCarpoolTestSub(91, 192, 0, 100) // 100 < 保底 192
	_, err := svc.ValidateAndCheckLimits(context.Background(), sub, carpoolTestGroup(91))
	require.NoError(t, err)
	require.Zero(t, counter.getCalls, "保底内根本不该去读计数器")
}

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckWeeklyLimitSubscriptionOverride(t *testing.T) {
	groupLimit := 100.0
	group := &Group{WeeklyLimitUSD: &groupLimit}

	// 无订阅级覆盖：回退分组级限额（向后兼容）
	sub := &UserSubscription{WeeklyUsageUSD: 50}
	require.True(t, sub.CheckWeeklyLimit(group, 60) == false)
	require.True(t, sub.CheckWeeklyLimit(group, 40))

	// 订阅级覆盖优先于分组级限额
	sub = &UserSubscription{WeeklyUsageUSD: 20, WeeklyLimitUSD: ptrFloat64(30)}
	require.False(t, sub.CheckWeeklyLimit(group, 15), "订阅级 30 限额应优先于分组级 100")
	require.True(t, sub.CheckWeeklyLimit(group, 10))

	// 订阅级覆盖在分组无限额时依然生效
	sub = &UserSubscription{WeeklyUsageUSD: 20, WeeklyLimitUSD: ptrFloat64(30)}
	require.False(t, sub.CheckWeeklyLimit(&Group{}, 15))

	// 分组为 nil 且订阅级有覆盖：依然生效
	require.False(t, sub.CheckWeeklyLimit(nil, 15))
	require.True(t, sub.CheckWeeklyLimit(nil, 5))

	// 双方都无限额：不限制
	sub = &UserSubscription{WeeklyUsageUSD: 9999}
	require.True(t, sub.CheckWeeklyLimit(&Group{}, 1))
	require.True(t, sub.CheckWeeklyLimit(nil, 1))
}

func TestEffectiveWeeklyLimit(t *testing.T) {
	groupLimit := 100.0
	group := &Group{WeeklyLimitUSD: &groupLimit}

	sub := &UserSubscription{}
	require.Same(t, &groupLimit, sub.EffectiveWeeklyLimit(group))

	override := 30.0
	sub = &UserSubscription{WeeklyLimitUSD: &override}
	require.Same(t, &override, sub.EffectiveWeeklyLimit(group))
	require.Same(t, &override, sub.EffectiveWeeklyLimit(nil))

	sub = &UserSubscription{}
	require.Nil(t, sub.EffectiveWeeklyLimit(&Group{}))
	require.Nil(t, sub.EffectiveWeeklyLimit(nil))
}

func TestCalculateProgressUsesSubscriptionWeeklyLimitOverride(t *testing.T) {
	svc := &SubscriptionService{}
	windowStart := time.Now().Add(-24 * time.Hour)
	groupLimit := 2400.0
	group := &Group{Name: "carpool", WeeklyLimitUSD: &groupLimit}

	// 订阅级覆盖优先：进度按订阅级限额计算
	sub := &UserSubscription{
		ExpiresAt:         time.Now().Add(24 * time.Hour),
		WeeklyWindowStart: &windowStart,
		WeeklyUsageUSD:    100,
		WeeklyLimitUSD:    ptrFloat64(672),
	}
	progress := svc.calculateProgress(sub, group)
	require.NotNil(t, progress.Weekly)
	require.InDelta(t, 672, progress.Weekly.LimitUSD, 1e-9)
	require.InDelta(t, 572, progress.Weekly.RemainingUSD, 1e-9)

	// 无覆盖：回退分组级限额（行为与现状一致）
	sub = &UserSubscription{
		ExpiresAt:         time.Now().Add(24 * time.Hour),
		WeeklyWindowStart: &windowStart,
		WeeklyUsageUSD:    100,
	}
	progress = svc.calculateProgress(sub, group)
	require.NotNil(t, progress.Weekly)
	require.InDelta(t, 2400, progress.Weekly.LimitUSD, 1e-9)

	// 双方都无限额：无周进度
	progress = svc.calculateProgress(sub, &Group{Name: "no-limit"})
	require.Nil(t, progress.Weekly)
}

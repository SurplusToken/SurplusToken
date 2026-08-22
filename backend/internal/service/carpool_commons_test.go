package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// fakeCarpoolCommonsCounter 记录调用的 CarpoolCommonsCounter 假实现。
type fakeCarpoolCommonsCounter struct {
	used     float64
	err      error
	getCalls int
	addCalls int
}

func (f *fakeCarpoolCommonsCounter) GetCommonsUsage(_ context.Context, _ int64, _ time.Time) (float64, error) {
	f.getCalls++
	return f.used, f.err
}

func (f *fakeCarpoolCommonsCounter) AddCommonsUsage(_ context.Context, _ int64, _ time.Time, _ float64) error {
	f.addCalls++
	return f.err
}

// newCarpoolTestSub 构造拼车成员订阅：保底 r、个人上限 r+C、周窗口进行中。
func newCarpoolTestSub(groupID int64, reserved, sharedPool, weeklyUsage float64) *UserSubscription {
	windowStart := time.Now().Add(-24 * time.Hour)
	limit := reserved + sharedPool
	return &UserSubscription{
		ID:                1,
		UserID:            10,
		GroupID:           groupID,
		Status:            SubscriptionStatusActive,
		StartsAt:          time.Now().Add(-48 * time.Hour),
		ExpiresAt:         time.Now().Add(30 * 24 * time.Hour),
		WeeklyWindowStart: &windowStart,
		WeeklyUsageUSD:    weeklyUsage,
		WeeklyReservedUSD: &reserved,
		WeeklyLimitUSD:    &limit,
	}
}

func newCarpoolSubscriptionService(counter CarpoolCommonsCounter) *SubscriptionService {
	billingCacheSvc := &BillingCacheService{cfg: &config.Config{}}
	billingCacheSvc.SetCarpoolCommonsCounter(counter)
	return NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, billingCacheSvc, nil, nil)
}

func carpoolTestGroup(groupID int64) *Group {
	return &Group{ID: groupID, SubscriptionType: SubscriptionTypeSubscription, Status: "active"}
}

// 保底内放行不依赖计数器：即使公共池已耗尽（或计数器根本未注入/读取失败），
// 周用量 < r 的成员无条件放行，且完全不读计数器。
func TestValidateAndCheckLimits_ReservePassesRegardlessOfPool(t *testing.T) {
	group := carpoolTestGroup(20)

	t.Run("公共池耗尽时保底内成员放行且不读计数器", func(t *testing.T) {
		counter := &fakeCarpoolCommonsCounter{used: 480} // C = 480，已耗尽
		svc := newCarpoolSubscriptionService(counter)
		sub := newCarpoolTestSub(20, 960, 480, 500) // 用量 500 < r=960

		needsMaintenance, err := svc.ValidateAndCheckLimits(context.Background(), sub, group)
		require.NoError(t, err)
		require.False(t, needsMaintenance)
		require.Zero(t, counter.getCalls, "保底内放行不得依赖公共池计数器")
	})

	t.Run("计数器未注入时保底内放行（行为与旧语义一致）", func(t *testing.T) {
		svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)
		sub := newCarpoolTestSub(20, 960, 480, 500)

		_, err := svc.ValidateAndCheckLimits(context.Background(), sub, group)
		require.NoError(t, err)
	})

	t.Run("计数器读取失败 fail-open", func(t *testing.T) {
		counter := &fakeCarpoolCommonsCounter{err: errors.New("redis down")}
		svc := newCarpoolSubscriptionService(counter)
		sub := newCarpoolTestSub(20, 960, 480, 1000) // 已越界吃公共池

		_, err := svc.ValidateAndCheckLimits(context.Background(), sub, group)
		require.NoError(t, err, "计数器故障应 fail-open（保底硬保证不受影响，公共池强制降级）")
	})
}

// 公共池硬约束：全车超额之和达到 C 后，越界成员被拒（429），
// 但同车保底内成员照常放行。
func TestValidateAndCheckLimits_PoolExhaustedRejectsExcessButNotReserve(t *testing.T) {
	group := carpoolTestGroup(20)
	counter := &fakeCarpoolCommonsCounter{used: 480} // Σ超额 = C
	svc := newCarpoolSubscriptionService(counter)

	excessSub := newCarpoolTestSub(20, 960, 480, 1000) // 用量 1000 ≥ r，开始吃公共池
	_, err := svc.ValidateAndCheckLimits(context.Background(), excessSub, group)
	require.ErrorIs(t, err, ErrCarpoolSharedPoolExhausted)

	reserveSub := newCarpoolTestSub(20, 960, 480, 959.9) // 保底内
	_, err = svc.ValidateAndCheckLimits(context.Background(), reserveSub, group)
	require.NoError(t, err, "公共池耗尽不得影响保底内成员")
}

func TestValidateAndCheckLimits_PoolNotFullAllowsExcess(t *testing.T) {
	group := carpoolTestGroup(20)
	counter := &fakeCarpoolCommonsCounter{used: 479.9} // 未满
	svc := newCarpoolSubscriptionService(counter)

	sub := newCarpoolTestSub(20, 960, 480, 1440) // 吃满个人上限 r+C 边缘
	_, err := svc.ValidateAndCheckLimits(context.Background(), sub, group)
	require.NoError(t, err)
	require.Equal(t, 1, counter.getCalls)
}

// 个人绝对上限防呆：用量 ≥ r+C 时按普通周限拒绝，与计数器状态无关。
func TestValidateAndCheckLimits_PersonalCapStillEnforced(t *testing.T) {
	group := carpoolTestGroup(20)
	counter := &fakeCarpoolCommonsCounter{used: 0}
	svc := newCarpoolSubscriptionService(counter)

	sub := newCarpoolTestSub(20, 960, 480, 1440.01) // 超过 r+C
	_, err := svc.ValidateAndCheckLimits(context.Background(), sub, group)
	require.ErrorIs(t, err, ErrWeeklyLimitExceeded)
	require.Zero(t, counter.getCalls, "个人上限拒绝在先，不应读计数器")
}

// 非拼车订阅回归：weekly_reserved_usd 为 NULL 时行为与现状完全一致——
// 不读计数器，限额只由 weekly_limit/group limit 决定。
func TestValidateAndCheckLimits_NonCarpoolSubscriptionUnchanged(t *testing.T) {
	windowStart := time.Now().Add(-24 * time.Hour)
	groupWeeklyLimit := 100.0
	group := &Group{
		ID:               20,
		SubscriptionType: SubscriptionTypeSubscription,
		Status:           "active",
		WeeklyLimitUSD:   &groupWeeklyLimit,
	}
	counter := &fakeCarpoolCommonsCounter{used: 1e9} // 即使计数器"满"也不应被读取
	svc := newCarpoolSubscriptionService(counter)

	under := &UserSubscription{
		ID: 1, UserID: 10, GroupID: 20,
		Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(24 * time.Hour),
		WeeklyWindowStart: &windowStart, WeeklyUsageUSD: 50,
	}
	_, err := svc.ValidateAndCheckLimits(context.Background(), under, group)
	require.NoError(t, err)

	over := &UserSubscription{
		ID: 2, UserID: 11, GroupID: 20,
		Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(24 * time.Hour),
		WeeklyWindowStart: &windowStart, WeeklyUsageUSD: 100.01,
	}
	_, err = svc.ValidateAndCheckLimits(context.Background(), over, group)
	require.ErrorIs(t, err, ErrWeeklyLimitExceeded)
	require.Zero(t, counter.getCalls, "非拼车订阅不得触碰公共池计数器")
}

// ── billing cache 层（checkSubscriptionEligibility）──────────────────────────

type carpoolSubCacheStub struct {
	BillingCache
	data *SubscriptionCacheData
}

func (s *carpoolSubCacheStub) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	return s.data, nil
}

func newCarpoolBillingCacheService(counter CarpoolCommonsCounter, cached *SubscriptionCacheData) *BillingCacheService {
	svc := &BillingCacheService{
		cache: &carpoolSubCacheStub{data: cached},
		cfg:   &config.Config{},
	}
	svc.SetCarpoolCommonsCounter(counter)
	return svc
}

func activeCachedSubscription(weeklyUsage float64) *SubscriptionCacheData {
	return &SubscriptionCacheData{
		Status:      SubscriptionStatusActive,
		ExpiresAt:   time.Now().Add(30 * 24 * time.Hour),
		WeeklyUsage: weeklyUsage,
	}
}

// billing cache 层同样执行公共池硬约束与订阅级周限覆盖。
func TestCheckSubscriptionEligibility_CarpoolEnforcement(t *testing.T) {
	group := carpoolTestGroup(20)

	t.Run("公共池耗尽拒绝越界请求", func(t *testing.T) {
		counter := &fakeCarpoolCommonsCounter{used: 480}
		svc := newCarpoolBillingCacheService(counter, activeCachedSubscription(1000))
		sub := newCarpoolTestSub(20, 960, 480, 1000)

		err := svc.checkSubscriptionEligibility(context.Background(), 10, group, sub)
		require.ErrorIs(t, err, ErrCarpoolSharedPoolExhausted)
	})

	t.Run("保底内不受公共池耗尽影响", func(t *testing.T) {
		counter := &fakeCarpoolCommonsCounter{used: 480}
		svc := newCarpoolBillingCacheService(counter, activeCachedSubscription(500))
		sub := newCarpoolTestSub(20, 960, 480, 500)

		err := svc.checkSubscriptionEligibility(context.Background(), 10, group, sub)
		require.NoError(t, err)
		require.Zero(t, counter.getCalls)
	})

	t.Run("订阅级周限额覆盖在缓存层生效（个人上限防呆）", func(t *testing.T) {
		counter := &fakeCarpoolCommonsCounter{}
		groupWeeklyLimit := 2400.0
		groupWithCap := carpoolTestGroup(20)
		groupWithCap.WeeklyLimitUSD = &groupWeeklyLimit
		svc := newCarpoolBillingCacheService(counter, activeCachedSubscription(1500))
		sub := newCarpoolTestSub(20, 960, 480, 1500) // 个人上限 1440 < 缓存用量 1500 < 组限 2400

		err := svc.checkSubscriptionEligibility(context.Background(), 10, groupWithCap, sub)
		require.ErrorIs(t, err, ErrWeeklyLimitExceeded, "订阅级覆盖 1440 应优先于分组级 2400")
	})

	t.Run("非拼车订阅行为不变且不读计数器", func(t *testing.T) {
		counter := &fakeCarpoolCommonsCounter{used: 1e9}
		groupWeeklyLimit := 2400.0
		groupWithCap := carpoolTestGroup(20)
		groupWithCap.WeeklyLimitUSD = &groupWeeklyLimit
		svc := newCarpoolBillingCacheService(counter, activeCachedSubscription(1500))
		sub := &UserSubscription{ID: 1, UserID: 10, GroupID: 20, Status: SubscriptionStatusActive}

		err := svc.checkSubscriptionEligibility(context.Background(), 10, groupWithCap, sub)
		require.NoError(t, err, "1500 < 组限 2400，非拼车订阅照常放行")
		require.Zero(t, counter.getCalls)
	})
}

// ── 周窗口重置网格吸附 ─────────────────────────────────────────────

type weeklyResetCaptureStub struct {
	userSubRepoNoop
	called   bool
	newStart time.Time
}

func (s *weeklyResetCaptureStub) ResetWeeklyUsage(_ context.Context, _ int64, _ *time.Time, newStart time.Time) error {
	s.called = true
	s.newStart = newStart
	return nil
}

// 拼车订阅周重置吸附回发车对齐的 7 天网格（而非当天零点），
// 保证全车成员窗口起点恒等、公共池计数器 key 一致。
func TestCheckAndResetWindows_CarpoolWeeklyResetSnapsToGrid(t *testing.T) {
	anchor := time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC) // 发车对齐的窗口起点
	reserved := 960.0
	sub := &UserSubscription{
		ID: 1, UserID: 10, GroupID: 20,
		Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		WeeklyWindowStart: &anchor, WeeklyUsageUSD: 700,
		WeeklyReservedUSD: &reserved,
	}
	stub := &weeklyResetCaptureStub{}
	svc := NewSubscriptionService(groupRepoNoop{}, stub, nil, nil, nil)

	require.NoError(t, svc.CheckAndResetWindows(context.Background(), sub))
	require.True(t, stub.called)

	week := 7 * 24 * time.Hour
	now := time.Now()
	require.Zero(t, stub.newStart.Sub(anchor)%week, "新起点必须落在发车锚定的 7 天网格上")
	require.False(t, now.Before(stub.newStart), "新起点必须 ≤ now")
	require.True(t, now.Before(stub.newStart.Add(week)), "now 必须落在新窗口内")
	require.NotEqual(t, startOfDay(now), stub.newStart, "网格起点一般不等于当天零点")
	require.Equal(t, 0.0, sub.WeeklyUsageUSD)
	require.Equal(t, stub.newStart, *sub.WeeklyWindowStart)
}

// 非拼车订阅跟随上游的期限对齐滚动窗口语义：从持久化锚点推进完整周期。
func TestCheckAndResetWindows_NonCarpoolWeeklyResetUsesRollingAnchor(t *testing.T) {
	now := time.Now()
	oldStart := now.Add(-8 * 24 * time.Hour)
	sub := &UserSubscription{
		ID: 1, UserID: 10, GroupID: 20,
		Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		WeeklyWindowStart: &oldStart, WeeklyUsageUSD: 700,
	}
	stub := &weeklyResetCaptureStub{}
	svc := NewSubscriptionService(groupRepoNoop{}, stub, nil, nil, nil)

	require.NoError(t, svc.CheckAndResetWindows(context.Background(), sub))
	require.True(t, stub.called)
	require.Equal(t, oldStart.Add(7*24*time.Hour), stub.newStart)
}

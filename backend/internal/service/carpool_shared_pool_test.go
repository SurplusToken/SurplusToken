package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type sharedPoolSubscriptionRepo struct {
	UserSubscriptionRepository
	subs []UserSubscription
	err  error
}

func (r *sharedPoolSubscriptionRepo) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]UserSubscription, *pagination.PaginationResult, error) {
	return r.subs, nil, r.err
}

type sharedPoolCapacitySource struct {
	snapshot *CarpoolCapacitySnapshot
	err      error
}

func (s *sharedPoolCapacitySource) GroupObservedCapacity(context.Context, int64, time.Time) (*CarpoolCapacitySnapshot, error) {
	return s.snapshot, s.err
}

func TestCarpoolSharedPoolSnapshotUsesRemainingQuota(t *testing.T) {
	windowStart := time.Date(2026, 8, 4, 3, 0, 0, 0, time.UTC)
	reserved, limit := 192.0, 672.0 // 发车时锁定的公共池 C = 480
	repo := &sharedPoolSubscriptionRepo{subs: []UserSubscription{{
		ID: 1, UserID: 10, GroupID: 20,
		Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		WeeklyWindowStart: &windowStart, WeeklyReservedUSD: &reserved, WeeklyLimitUSD: &limit,
	}}}
	counter := &fakeCarpoolCommonsCounter{used: 168}
	billing := &BillingCacheService{carpoolCommons: counter}
	subscriptions := &SubscriptionService{userSubRepo: repo, billingCacheService: billing}
	memberRole := "member"
	groupID := int64(20)
	carpools := &CarpoolService{
		repo: &carpoolRepoStub{carpool: &Carpool{
			ID: 7, Status: "active", PricingModel: CarpoolPricingQuota,
			GroupID: &groupID, MemberRole: &memberRole,
		}},
		subscriptionService: subscriptions,
	}

	snapshot, err := carpools.GetSharedPool(context.Background(), 7, 10, false)
	require.NoError(t, err)
	require.InDelta(t, 480, snapshot.CapacityUSD, 1e-9)
	require.InDelta(t, 168, snapshot.UsedUSD, 1e-9)
	require.InDelta(t, 312, snapshot.RemainingUSD, 1e-9)
	require.Equal(t, windowStart, snapshot.WindowStart)
	require.Equal(t, windowStart.Add(7*24*time.Hour), snapshot.ResetsAt)
	require.Equal(t, 1, counter.getCalls)
}

func TestCarpoolSharedPoolSnapshotPrefersObservedCapacityAndClampsRemaining(t *testing.T) {
	sub := newCarpoolTestSub(20, 192, 480, 0)
	billing := &BillingCacheService{
		carpoolCommons:  &fakeCarpoolCommonsCounter{used: 510},
		carpoolCapacity: &sharedPoolCapacitySource{snapshot: &CarpoolCapacitySnapshot{Trusted: true, CommonsUSD: 500}},
	}

	snapshot, err := billing.carpoolSharedPoolSnapshot(context.Background(), sub)
	require.NoError(t, err)
	require.InDelta(t, 500, snapshot.CapacityUSD, 1e-9)
	require.InDelta(t, 510, snapshot.UsedUSD, 1e-9)
	require.Zero(t, snapshot.RemainingUSD)
}

func TestCarpoolSharedPoolUnavailableDoesNotPretendPoolIsEmpty(t *testing.T) {
	sub := newCarpoolTestSub(20, 192, 480, 0)

	_, err := (&BillingCacheService{}).carpoolSharedPoolSnapshot(context.Background(), sub)
	require.ErrorIs(t, err, ErrCarpoolSharedPoolUnavailable)

	billing := &BillingCacheService{carpoolCommons: &fakeCarpoolCommonsCounter{err: errors.New("redis down")}}
	_, err = billing.carpoolSharedPoolSnapshot(context.Background(), sub)
	require.ErrorIs(t, err, ErrCarpoolSharedPoolUnavailable)
}

func TestGetSharedPoolRejectsNonMemberBeforeReadingRuntimeData(t *testing.T) {
	groupID := int64(20)
	carpools := &CarpoolService{repo: &carpoolRepoStub{carpool: &Carpool{
		ID: 7, Status: "active", GroupID: &groupID,
	}}}

	_, err := carpools.GetSharedPool(context.Background(), 7, 99, false)
	require.ErrorIs(t, err, ErrCarpoolForbidden)
}

func TestSelectCarpoolSharedPoolSubscriptionSkipsInactiveRows(t *testing.T) {
	now := time.Now()
	oldWindow, newWindow := now.Add(-48*time.Hour), now.Add(-24*time.Hour)
	reserved, limit := 10.0, 20.0
	rows := []UserSubscription{
		{Status: SubscriptionStatusExpired, ExpiresAt: now.Add(time.Hour), WeeklyWindowStart: &newWindow, WeeklyReservedUSD: &reserved, WeeklyLimitUSD: &limit},
		{Status: SubscriptionStatusActive, ExpiresAt: now.Add(-time.Hour), WeeklyWindowStart: &newWindow, WeeklyReservedUSD: &reserved, WeeklyLimitUSD: &limit},
		{ID: 1, Status: SubscriptionStatusActive, ExpiresAt: now.Add(time.Hour), WeeklyWindowStart: &oldWindow, WeeklyReservedUSD: &reserved, WeeklyLimitUSD: &limit},
		{ID: 2, Status: SubscriptionStatusActive, ExpiresAt: now.Add(time.Hour), WeeklyWindowStart: &newWindow, WeeklyReservedUSD: &reserved, WeeklyLimitUSD: &limit},
	}

	selected := selectCarpoolSharedPoolSubscription(rows, now)
	require.NotNil(t, selected)
	require.Equal(t, int64(2), selected.ID)
}

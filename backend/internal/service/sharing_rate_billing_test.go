//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type sharingRateCoOwnerReaderStub struct {
	ids   []int64
	err   error
	calls int
}

func TestShouldApplySharingRateBilling_RequiresDynamicStandardGroup(t *testing.T) {
	ctx := t.Context()
	require.False(t, shouldApplySharingRateBilling(ctx, nil, true))
	require.False(t, shouldApplySharingRateBilling(ctx, &Group{SubscriptionType: SubscriptionTypeStandard}, true))
	require.False(t, shouldApplySharingRateBilling(ctx, &Group{DynamicSharingPool: true, SubscriptionType: SubscriptionTypeStandard}, false))
	require.False(t, shouldApplySharingRateBilling(ctx, &Group{DynamicSharingPool: true, SubscriptionType: SubscriptionTypeSubscription}, true))
	require.True(t, shouldApplySharingRateBilling(ctx, &Group{DynamicSharingPool: true, SubscriptionType: SubscriptionTypeStandard}, true))
}

func TestShouldApplySharingRateBilling_ResolvedFallbackGroupWins(t *testing.T) {
	dynamic := &Group{DynamicSharingPool: true, SubscriptionType: SubscriptionTypeStandard}
	fixedResolved := &Group{ID: 9, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, SubscriptionType: SubscriptionTypeStandard}
	ctx := context.WithValue(t.Context(), ctxkey.Group, fixedResolved)
	require.False(t, shouldApplySharingRateBilling(ctx, dynamic, true))
}

func (s *sharingRateCoOwnerReaderStub) ListCoOwnerUserIDsByAccount(context.Context, int64) ([]int64, error) {
	s.calls++
	return s.ids, s.err
}

func TestApplySharingRateBilling_ExternalConsumerAppliesOnlyActualCost(t *testing.T) {
	ownerID := int64(10)
	sharingRate := 2.5
	cost := &CostBreakdown{
		InputCost:  2,
		OutputCost: 2,
		TotalCost:  4,
		ActualCost: 6,
	}
	reader := &sharingRateCoOwnerReaderStub{}

	decision := applySharingRateBilling(
		context.Background(),
		cost,
		&Account{ID: 7, OwnerUserID: &ownerID, SharingRateMultiplier: &sharingRate},
		&User{ID: 99},
		false,
		true,
		reader,
	)

	require.True(t, decision.ContributionEligible)
	require.Equal(t, 1, reader.calls)
	require.NotNil(t, decision.UsageSharingRateMultiplier)
	require.Equal(t, 2.5, *decision.UsageSharingRateMultiplier)
	require.Equal(t, 2.5, decision.ContributionSharingRateMultiplier)
	require.Equal(t, 15.0, cost.ActualCost)
	require.Equal(t, 4.0, cost.TotalCost)
	require.Equal(t, 2.0, cost.InputCost)
	require.Equal(t, 2.0, cost.OutputCost)
}

func TestApplySharingRateBilling_ExternalRateZeroAndOneAreSnapshotted(t *testing.T) {
	ownerID := int64(10)
	for _, rate := range []float64{0, 1} {
		t.Run(string(rune('0'+int(rate))), func(t *testing.T) {
			cost := &CostBreakdown{TotalCost: 4, ActualCost: 6}
			decision := applySharingRateBilling(
				context.Background(),
				cost,
				&Account{ID: 7, OwnerUserID: &ownerID, SharingRateMultiplier: &rate},
				&User{ID: 99},
				false,
				true,
				&sharingRateCoOwnerReaderStub{},
			)
			require.True(t, decision.ContributionEligible)
			require.NotNil(t, decision.UsageSharingRateMultiplier)
			require.Equal(t, rate, *decision.UsageSharingRateMultiplier)
			require.Equal(t, 6*rate, cost.ActualCost)
		})
	}
}

func TestApplySharingRateBilling_FlagOffKeepsLegacyRewardAtOneX(t *testing.T) {
	ownerID := int64(10)
	sharingRate := 3.0
	cost := &CostBreakdown{TotalCost: 4, ActualCost: 6}

	decision := applySharingRateBilling(
		context.Background(),
		cost,
		&Account{ID: 7, OwnerUserID: &ownerID, SharingRateMultiplier: &sharingRate},
		&User{ID: 99},
		false,
		false,
		&sharingRateCoOwnerReaderStub{},
	)

	require.True(t, decision.ContributionEligible)
	require.Nil(t, decision.UsageSharingRateMultiplier)
	require.Equal(t, 1.0, decision.ContributionSharingRateMultiplier)
	require.Equal(t, 6.0, cost.ActualCost)
}

func TestApplySharingRateBilling_OwnerPathsDoNotChargeOrReward(t *testing.T) {
	ownerID := int64(10)
	sharingRate := 3.0
	tests := []struct {
		name      string
		consumer  int64
		coOwners  []int64
		readerIDs []int64
		wantCalls int
	}{
		{name: "primary", consumer: ownerID},
		{name: "cached co-owner revalidated", consumer: 20, coOwners: []int64{20}, readerIDs: []int64{20}, wantCalls: 1},
		{name: "hydrated co-owner", consumer: 20, readerIDs: []int64{20}, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := &CostBreakdown{TotalCost: 4, ActualCost: 6}
			reader := &sharingRateCoOwnerReaderStub{ids: tt.readerIDs}
			decision := applySharingRateBilling(
				context.Background(),
				cost,
				&Account{ID: 7, OwnerUserID: &ownerID, CoOwnerUserIDs: tt.coOwners, SharingRateMultiplier: &sharingRate},
				&User{ID: tt.consumer},
				false,
				true,
				reader,
			)
			require.False(t, decision.ContributionEligible)
			require.Nil(t, decision.UsageSharingRateMultiplier)
			require.Equal(t, 6.0, cost.ActualCost)
			require.Equal(t, tt.wantCalls, reader.calls)
		})
	}
}

func TestApplySharingRateBilling_SubscriptionAndSystemAccountBypass(t *testing.T) {
	ownerID := int64(10)
	sharingRate := 3.0
	tests := []struct {
		name         string
		account      *Account
		subscription bool
	}{
		{name: "system", account: &Account{ID: 7, SharingRateMultiplier: &sharingRate}},
		{name: "subscription", account: &Account{ID: 7, OwnerUserID: &ownerID, SharingRateMultiplier: &sharingRate}, subscription: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := &CostBreakdown{TotalCost: 4, ActualCost: 6}
			reader := &sharingRateCoOwnerReaderStub{}
			decision := applySharingRateBilling(context.Background(), cost, tt.account, &User{ID: 99}, tt.subscription, true, reader)
			require.False(t, decision.ContributionEligible)
			require.Nil(t, decision.UsageSharingRateMultiplier)
			require.Equal(t, 6.0, cost.ActualCost)
			require.Zero(t, reader.calls)
		})
	}
}

func TestApplySharingRateBilling_HydrationFailureFailsClosed(t *testing.T) {
	ownerID := int64(10)
	sharingRate := 3.0
	cost := &CostBreakdown{TotalCost: 4, ActualCost: 6}
	reader := &sharingRateCoOwnerReaderStub{err: errors.New("database unavailable")}

	decision := applySharingRateBilling(
		context.Background(),
		cost,
		&Account{ID: 7, OwnerUserID: &ownerID, SharingRateMultiplier: &sharingRate},
		&User{ID: 99},
		false,
		true,
		reader,
	)

	require.False(t, decision.ContributionEligible)
	require.Nil(t, decision.UsageSharingRateMultiplier)
	require.Equal(t, 6.0, cost.ActualCost)
	require.Equal(t, 1, reader.calls)
}

func TestApplySharingRateBilling_AuthoritativeMembershipOverridesNonEmptyStaleCache(t *testing.T) {
	ownerID := int64(10)
	sharingRate := 2.0

	removedCost := &CostBreakdown{TotalCost: 3, ActualCost: 4}
	removed := applySharingRateBilling(
		context.Background(),
		removedCost,
		&Account{ID: 7, OwnerUserID: &ownerID, CoOwnerUserIDs: []int64{20}, SharingRateMultiplier: &sharingRate},
		&User{ID: 20},
		false,
		true,
		&sharingRateCoOwnerReaderStub{ids: []int64{30}},
	)
	require.True(t, removed.ContributionEligible)
	require.Equal(t, 8.0, removedCost.ActualCost)

	addedCost := &CostBreakdown{TotalCost: 3, ActualCost: 4}
	added := applySharingRateBilling(
		context.Background(),
		addedCost,
		&Account{ID: 7, OwnerUserID: &ownerID, CoOwnerUserIDs: []int64{30}, SharingRateMultiplier: &sharingRate},
		&User{ID: 20},
		false,
		true,
		&sharingRateCoOwnerReaderStub{ids: []int64{20}},
	)
	require.False(t, added.ContributionEligible)
	require.Equal(t, 4.0, addedCost.ActualCost)
}

//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Regression guard for the marketplace scoping bug: sharing rates are billed only
// inside dynamic sharing pool groups (shouldApplySharingRateBilling), so they must
// not skew scheduling anywhere else. Before the fix, price tiering keyed off the
// global sharing_range_filter_enabled setting alone, so a contributed account priced
// at 0.15 monopolized ORDINARY groups' traffic on a price nobody was ever charged.

func sharingScopeAccounts() []accountWithLoad {
	ownerID := int64(10)
	cheap := 0.15
	contributed := &Account{ID: 1, OwnerUserID: &ownerID, SharingRateMultiplier: &cheap}
	system := &Account{ID: 2} // no owner → effective sharing rate 1.0
	return []accountWithLoad{
		{account: contributed, loadInfo: &AccountLoadInfo{AccountID: 1}},
		{account: system, loadInfo: &AccountLoadInfo{AccountID: 2}},
	}
}

// consumer 20 is NOT the owner, so the contributed account's 0.15 price applies to it.
func sharingScopeCtx(dynamicPool bool) context.Context {
	ctx := WithRequestingUserID(context.Background(), 20)
	ctx = WithSharingRangeFilterEnabled(ctx, true) // global setting on (as in prod)
	ctx = WithDynamicSharingPoolEnabled(ctx, dynamicPool)
	return ctx
}

func TestFilterByMinSharingRate_InertOutsideDynamicPool(t *testing.T) {
	// Ordinary group: the price is not billed here, so it must not narrow candidates.
	got := filterByMinSharingRate(sharingScopeCtx(false), sharingScopeAccounts())
	require.Len(t, got, 2, "outside a dynamic sharing pool the sharing price must not drop candidates")
}

func TestFilterByMinSharingRate_AppliesInsideDynamicPool(t *testing.T) {
	// Dynamic sharing pool: price tiering is the point — keep only the cheapest tier.
	got := filterByMinSharingRate(sharingScopeCtx(true), sharingScopeAccounts())
	require.Len(t, got, 1)
	require.Equal(t, int64(1), got[0].account.ID)
}

func TestSharingRateActiveFromContext_RequiresBothPoolAndSetting(t *testing.T) {
	require.False(t, SharingRateActiveFromContext(sharingScopeCtx(false)), "setting alone must not activate sharing rates")
	require.True(t, SharingRateActiveFromContext(sharingScopeCtx(true)))

	// Setting off inside a pool: still inert.
	ctx := WithDynamicSharingPoolEnabled(context.Background(), true)
	ctx = WithSharingRangeFilterEnabled(ctx, false)
	require.False(t, SharingRateActiveFromContext(ctx))
}

func TestCompareSharingRateForScheduling_InertOutsideDynamicPool(t *testing.T) {
	accounts := sharingScopeAccounts()
	cheap, system := accounts[0].account, accounts[1].account

	require.Equal(t, 0, compareSharingRateForScheduling(sharingScopeCtx(false), cheap, system),
		"outside a dynamic sharing pool price must not order accounts")
	require.Negative(t, compareSharingRateForScheduling(sharingScopeCtx(true), cheap, system),
		"inside a dynamic sharing pool the cheaper account wins")
}

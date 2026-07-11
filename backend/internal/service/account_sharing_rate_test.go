package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func f64(v float64) *float64 { return &v }

func contributedAccount(ownerID int64, coOwners []int64, sharingRate *float64) Account {
	return Account{
		ID:                    1,
		Status:                StatusActive,
		Type:                  AccountTypeOAuth,
		OwnerUserID:           &ownerID,
		CoOwnerUserIDs:        coOwners,
		SharingRateMultiplier: sharingRate,
	}
}

func TestAccount_EffectiveSharingRateMultiplier_SystemAccountAlwaysOne(t *testing.T) {
	a := Account{ID: 1, Status: StatusActive, Type: AccountTypeOAuth, SharingRateMultiplier: f64(3.0)}
	require.Nil(t, a.OwnerUserID)
	require.Equal(t, 1.0, a.EffectiveSharingRateMultiplier(999))
}

func TestAccount_EffectiveSharingRateMultiplier_PrimaryOwnerSelfUseIsOne(t *testing.T) {
	a := contributedAccount(10, nil, f64(3.0))
	require.Equal(t, 1.0, a.EffectiveSharingRateMultiplier(10))
}

func TestAccount_EffectiveSharingRateMultiplier_CoOwnerSelfUseIsOne(t *testing.T) {
	a := contributedAccount(10, []int64{20, 30}, f64(3.0))
	require.Equal(t, 1.0, a.EffectiveSharingRateMultiplier(20))
	require.Equal(t, 1.0, a.EffectiveSharingRateMultiplier(30))
}

func TestAccount_EffectiveSharingRateMultiplier_ExternalConsumerGetsStoredRate(t *testing.T) {
	a := contributedAccount(10, []int64{20}, f64(2.5))
	require.Equal(t, 2.5, a.EffectiveSharingRateMultiplier(999))
}

func TestAccount_EffectiveSharingRateMultiplier_ExternalConsumerZeroIsLegalFree(t *testing.T) {
	a := contributedAccount(10, nil, f64(0))
	require.Equal(t, 0.0, a.EffectiveSharingRateMultiplier(999))
}

func TestAccount_EffectiveSharingRateMultiplier_NilStoredValueDefaultsToOne(t *testing.T) {
	a := contributedAccount(10, nil, nil)
	require.Equal(t, 1.0, a.EffectiveSharingRateMultiplier(999))
}

func TestAccount_EffectiveSharingRateMultiplier_NegativeStoredValueDefensesToOne(t *testing.T) {
	a := contributedAccount(10, nil, f64(-1))
	require.Equal(t, 1.0, a.EffectiveSharingRateMultiplier(999))
}

func TestAccount_EffectiveSharingRateMultiplier_NilAccountReturnsOne(t *testing.T) {
	var a *Account
	require.Equal(t, 1.0, a.EffectiveSharingRateMultiplier(1))
}

func TestAccount_IsSharingEligibleForConsumer_NilAccountIsIneligible(t *testing.T) {
	var a *Account
	require.False(t, a.IsSharingEligibleForConsumer(1, nil, nil, true))
}

func TestAccount_IsSharingEligibleForConsumer_FilterDisabledAlwaysEligible(t *testing.T) {
	a := contributedAccount(10, nil, f64(10)) // out-of-range value on purpose (defensive data)
	require.True(t, a.IsSharingEligibleForConsumer(999, f64(0), f64(1), false))
}

func TestAccount_IsSharingEligibleForConsumer_NonContributedAlwaysEligible(t *testing.T) {
	a := Account{ID: 1, Status: StatusActive, Type: AccountTypeOAuth}
	require.True(t, a.IsSharingEligibleForConsumer(999, f64(0), f64(0.5), true))
}

func TestAccount_IsSharingEligibleForConsumer_OwnerAndCoOwnerBypassRange(t *testing.T) {
	a := contributedAccount(10, []int64{20}, f64(4.0))
	require.True(t, a.IsSharingEligibleForConsumer(10, f64(0), f64(1), true))
	require.True(t, a.IsSharingEligibleForConsumer(20, f64(0), f64(1), true))
}

func TestAccount_IsSharingEligibleForConsumer_UnboundedRangeAcceptsAnyPrice(t *testing.T) {
	a := contributedAccount(10, nil, f64(4.0))
	require.True(t, a.IsSharingEligibleForConsumer(999, nil, nil, true))
}

func TestAccount_IsSharingEligibleForConsumer_MinOnlyClosedInterval(t *testing.T) {
	a := contributedAccount(10, nil, f64(1.0))
	require.True(t, a.IsSharingEligibleForConsumer(999, f64(1.0), nil, true), "rate == min must be accepted (closed interval)")
	require.False(t, a.IsSharingEligibleForConsumer(999, f64(1.01), nil, true))
}

func TestAccount_IsSharingEligibleForConsumer_MaxOnlyClosedInterval(t *testing.T) {
	a := contributedAccount(10, nil, f64(2.0))
	require.True(t, a.IsSharingEligibleForConsumer(999, nil, f64(2.0), true), "rate == max must be accepted (closed interval)")
	require.False(t, a.IsSharingEligibleForConsumer(999, nil, f64(1.99), true))
}

func TestAccount_IsSharingEligibleForConsumer_BothBoundsInsideAndOutside(t *testing.T) {
	a := contributedAccount(10, nil, f64(1.5))
	require.True(t, a.IsSharingEligibleForConsumer(999, f64(1.0), f64(2.0), true))

	tooExpensive := contributedAccount(10, nil, f64(2.5))
	require.False(t, tooExpensive.IsSharingEligibleForConsumer(999, f64(1.0), f64(2.0), true))

	tooCheap := contributedAccount(10, nil, f64(0.5))
	require.False(t, tooCheap.IsSharingEligibleForConsumer(999, f64(1.0), f64(2.0), true))
}

func TestAccount_IsSharingEligibleForConsumer_IllegalStoredRateFailsClosed(t *testing.T) {
	for _, rate := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -0.01, 5.01} {
		a := contributedAccount(10, nil, f64(rate))
		require.False(t, a.IsSharingEligibleForConsumer(999, nil, nil, true),
			"corrupt stored rate %v must fail closed for an external consumer under an unbounded accepted range", rate)
	}
}

func TestAccount_IsSharingEligibleForConsumer_NilAndZeroStoredRateStayLegal(t *testing.T) {
	nilRate := contributedAccount(10, nil, nil)
	require.True(t, nilRate.IsSharingEligibleForConsumer(999, nil, nil, true))

	zeroRate := contributedAccount(10, nil, f64(0))
	require.True(t, zeroRate.IsSharingEligibleForConsumer(999, f64(0), f64(1), true))
}

func TestAccount_IsSharingEligibleForConsumer_IllegalAcceptedBoundFailsClosed(t *testing.T) {
	a := contributedAccount(10, nil, f64(1.0))
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		require.False(t, a.IsSharingEligibleForConsumer(999, f64(bad), nil, true), "corrupt min bound %v must fail closed", bad)
		require.False(t, a.IsSharingEligibleForConsumer(999, nil, f64(bad), true), "corrupt max bound %v must fail closed", bad)
	}
}

func TestAccount_IsSharingEligibleForConsumer_OwnerAndSystemBypassIllegalRateAndBounds(t *testing.T) {
	badBound := f64(math.NaN())

	owned := contributedAccount(10, []int64{20}, f64(math.Inf(1)))
	require.True(t, owned.IsSharingEligibleForConsumer(10, badBound, badBound, true), "primary owner bypasses even corrupt rate/bounds")
	require.True(t, owned.IsSharingEligibleForConsumer(20, badBound, badBound, true), "co-owner bypasses even corrupt rate/bounds")

	system := Account{ID: 1, Status: StatusActive, Type: AccountTypeOAuth, SharingRateMultiplier: f64(math.NaN())}
	require.True(t, system.IsSharingEligibleForConsumer(999, badBound, badBound, true), "system (non-contributed) account bypasses even corrupt rate/bounds")
}

func TestUser_AcceptsSharingRate_NilUserAcceptsAny(t *testing.T) {
	var u *User
	require.True(t, u.AcceptsSharingRate(99))
}

func TestUser_AcceptsSharingRate_NoBoundsAcceptsAny(t *testing.T) {
	u := User{}
	require.True(t, u.AcceptsSharingRate(0))
	require.True(t, u.AcceptsSharingRate(5))
}

func TestUser_AcceptsSharingRate_MinOnlyClosedInterval(t *testing.T) {
	u := User{SharingRateMin: f64(1.0)}
	require.True(t, u.AcceptsSharingRate(1.0))
	require.False(t, u.AcceptsSharingRate(0.99))
}

func TestUser_AcceptsSharingRate_MaxOnlyClosedInterval(t *testing.T) {
	u := User{SharingRateMax: f64(2.0)}
	require.True(t, u.AcceptsSharingRate(2.0))
	require.False(t, u.AcceptsSharingRate(2.01))
}

func TestUser_AcceptsSharingRate_BothBounds(t *testing.T) {
	u := User{SharingRateMin: f64(1.0), SharingRateMax: f64(2.0)}
	require.True(t, u.AcceptsSharingRate(1.0))
	require.True(t, u.AcceptsSharingRate(1.5))
	require.True(t, u.AcceptsSharingRate(2.0))
	require.False(t, u.AcceptsSharingRate(0.99))
	require.False(t, u.AcceptsSharingRate(2.01))
}

func TestSharingRateAcceptedRangeContext_RoundTrip(t *testing.T) {
	ctx := WithSharingRateAcceptedRange(t.Context(), f64(1.0), f64(2.0))
	min, max := SharingRateAcceptedRangeFromContext(ctx)
	require.Equal(t, 1.0, *min)
	require.Equal(t, 2.0, *max)
}

func TestSharingRateAcceptedRangeContext_UnsetReturnsNil(t *testing.T) {
	min, max := SharingRateAcceptedRangeFromContext(t.Context())
	require.Nil(t, min)
	require.Nil(t, max)
}

func TestSharingRangeFilterEnabledContext_DefaultsFalse(t *testing.T) {
	require.False(t, SharingRangeFilterEnabledFromContext(t.Context()))
	ctx := WithSharingRangeFilterEnabled(t.Context(), true)
	require.True(t, SharingRangeFilterEnabledFromContext(ctx))
}

func TestCompareSharingRateForScheduling_UsesConsumerEffectiveRateOnlyWhenEnabled(t *testing.T) {
	consumerID := int64(99)
	cheapOwnerID := int64(10)
	expensiveOwnerID := int64(20)
	cheap := contributedAccount(cheapOwnerID, nil, f64(0.5))
	expensive := contributedAccount(expensiveOwnerID, nil, f64(2.0))

	disabled := WithRequestingUserID(t.Context(), consumerID)
	require.Zero(t, compareSharingRateForScheduling(disabled, &cheap, &expensive))

	enabled := WithSharingRangeFilterEnabled(disabled, true)
	require.Negative(t, compareSharingRateForScheduling(enabled, &cheap, &expensive))
	require.Positive(t, compareSharingRateForScheduling(enabled, &expensive, &cheap))

	system := Account{ID: 3, Type: AccountTypeOAuth}
	ownerSelfUse := contributedAccount(consumerID, nil, f64(4.0))
	unset := contributedAccount(cheapOwnerID, nil, nil)
	require.Zero(t, compareSharingRateForScheduling(enabled, &system, &ownerSelfUse), "system and owner self-use both rank as 1x")
	require.Zero(t, compareSharingRateForScheduling(enabled, &system, &unset), "an unset contributed price defaults to 1x")
}

func TestFilterByMinSharingRate_KeepsWholeLowestPriceTier(t *testing.T) {
	ownerID := int64(10)
	accounts := []accountWithLoad{
		{account: &Account{ID: 1, OwnerUserID: &ownerID, SharingRateMultiplier: f64(0.5)}},
		{account: &Account{ID: 2, OwnerUserID: &ownerID, SharingRateMultiplier: f64(0.5)}},
		{account: &Account{ID: 3, OwnerUserID: &ownerID, SharingRateMultiplier: f64(1.5)}},
	}
	ctx := WithRequestingUserID(t.Context(), 99)
	ctx = WithSharingRangeFilterEnabled(ctx, true)

	got := filterByMinSharingRate(ctx, accounts)
	require.Len(t, got, 2)
	require.ElementsMatch(t, []int64{1, 2}, []int64{got[0].account.ID, got[1].account.ID})
	require.Len(t, filterByMinSharingRate(t.Context(), accounts), 3, "feature-off path must remain inert")
}

func TestSharingRateWriteValidationRejectsMoreThanFourDecimals(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, ValidateSharingRateMultiplier(1.23451, 0, 5), ErrSharingRatePrecisionInvalid)
	require.NoError(t, ValidateSharingRateMultiplier(1.2345, 0, 5))
	require.ErrorIs(t, ValidateSharingRateAcceptedRange(f64(0.12345), nil), ErrSharingRatePrecisionInvalid)
	require.NoError(t, ValidateSharingRateAcceptedRange(f64(0.1234), f64(5)))
}

package service

import infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

// Errors for the shared account rate marketplace: owner price updates, user
// accepted-range preferences, and contribution-pool distribution.
var (
	// ErrSharingRateOutOfRange is returned when an owner-set sharing_rate_multiplier
	// falls outside the hard [0,5] range or the admin-configurable dynamic floor/cap.
	ErrSharingRateOutOfRange       = infraerrors.BadRequest("SHARING_RATE_OUT_OF_RANGE", "sharing rate multiplier is outside the allowed range")
	ErrSharingRatePrecisionInvalid = infraerrors.BadRequest("SHARING_RATE_PRECISION_INVALID", "sharing rates support at most 4 decimal places")

	// ErrSharingRateCooldown is returned when an owner tries to change an account's
	// sharing price again before the configured cooldown has elapsed.
	ErrSharingRateCooldown = infraerrors.TooManyRequests("SHARING_RATE_COOLDOWN", "sharing price was changed too recently; please wait before changing it again")

	// ErrSharingRateRangeInvalid is returned when a user's accepted range has min > max.
	ErrSharingRateRangeInvalid = infraerrors.BadRequest("SHARING_RATE_RANGE_INVALID", "sharing_rate_min must be less than or equal to sharing_rate_max")

	// ErrSharingRateSettingsInvalid rejects malformed admin policy instead of
	// silently clamping it to a different billing/scheduling configuration.
	ErrSharingRateSettingsInvalid = infraerrors.BadRequest("SHARING_RATE_SETTINGS_INVALID", "sharing rate floor/cap or cooldown is invalid")

	// ErrSharingRateCoOwnerCannotPrice is returned when a co-owner (not the primary
	// owner) attempts to change an account's sharing price.
	ErrSharingRateCoOwnerCannotPrice = infraerrors.Forbidden("SHARING_RATE_CO_OWNER_CANNOT_PRICE", "only the account's primary owner can change its sharing price")

	// ErrAccountDeleteBlockedNonzeroPool is returned when deleting a contributed
	// account would orphan an undistributed contribution-pool balance.
	ErrAccountDeleteBlockedNonzeroPool = infraerrors.Conflict("ACCOUNT_DELETE_BLOCKED_NONZERO_POOL", "cannot delete a contributed account with an undistributed reward pool balance; distribute the pool first")

	// ErrUserDeleteBlockedAccountOwnership prevents deleting the only principal
	// capable of managing a contributed account and its held reward pool.
	ErrUserDeleteBlockedAccountOwnership = infraerrors.Conflict("USER_DELETE_BLOCKED_ACCOUNT_OWNERSHIP", "cannot delete a user who still owns contributed accounts; distribute any held reward and delete those accounts first")

	// ErrContributionPoolDistributionModeInvalid is returned when the distribute
	// API is called with a mode outside the strict enum ('even' | 'custom').
	ErrContributionPoolDistributionModeInvalid = infraerrors.BadRequest("CONTRIBUTION_POOL_DISTRIBUTION_MODE_INVALID", "mode must be 'even' or 'custom'")
)

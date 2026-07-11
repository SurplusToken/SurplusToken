package service

import (
	"context"
	"log/slog"
)

type sharingRateCoOwnerReader interface {
	ListCoOwnerUserIDsByAccount(ctx context.Context, accountID int64) ([]int64, error)
}

type sharingRateBillingDecision struct {
	ContributionEligible              bool
	ContributionSharingRateMultiplier float64
	UsageSharingRateMultiplier        *float64
}

// applySharingRateBilling resolves ownership before applying marketplace
// pricing. Every non-primary consumer is checked against the authoritative
// relation: a non-empty scheduler snapshot may still be stale after a co-owner
// add/remove, and financial decisions cannot rely on that cache window.
func applySharingRateBilling(
	ctx context.Context,
	cost *CostBreakdown,
	account *Account,
	consumer *User,
	isSubscriptionBill bool,
	billingEnabled bool,
	coOwnerReader sharingRateCoOwnerReader,
) sharingRateBillingDecision {
	decision := sharingRateBillingDecision{
		ContributionSharingRateMultiplier: SharingRateMultiplierDefault,
	}
	if cost == nil || account == nil || consumer == nil || consumer.ID <= 0 ||
		isSubscriptionBill || !account.IsUserContributed() {
		return decision
	}
	if account.OwnerUserID != nil && *account.OwnerUserID == consumer.ID {
		return decision
	}

	if coOwnerReader == nil {
		slog.Warn("shared account ownership unavailable; using 1x without contribution reward",
			"account_id", account.ID,
			"consumer_user_id", consumer.ID,
		)
		return decision
	}
	lookupCtx, cancel := detachedBillingContext(ctx)
	coOwnerIDs, err := coOwnerReader.ListCoOwnerUserIDsByAccount(lookupCtx, account.ID)
	cancel()
	if err != nil {
		slog.Warn("failed to hydrate shared account co-owners; using 1x without contribution reward",
			"account_id", account.ID,
			"consumer_user_id", consumer.ID,
			"error", err,
		)
		return decision
	}
	accountCopy := *account
	accountCopy.CoOwnerUserIDs = coOwnerIDs
	resolvedAccount := &accountCopy
	if resolvedAccount.IsSurplusAIOwner(consumer.ID) {
		return decision
	}

	decision.ContributionEligible = true
	if !billingEnabled {
		return decision
	}

	rate := resolvedAccount.EffectiveSharingRateMultiplier(consumer.ID)
	cost.ActualCost *= rate
	decision.ContributionSharingRateMultiplier = rate
	decision.UsageSharingRateMultiplier = &rate
	return decision
}

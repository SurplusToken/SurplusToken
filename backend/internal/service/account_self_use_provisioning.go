package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// Contributor self-use provisioning.
//
// When a user contributes an OAuth account, we give them a private, free,
// unlimited path to their OWN contributed accounts by reusing the existing
// subscription machinery (subscription usage never touches platform balance):
//
//   - one per-user "self-use" group (auto_self_use=true, subscription-type,
//     unlimited limits, image generation enabled) holding only that user's own
//     contributed accounts;
//   - a long-lived UserSubscription binding the user to that group;
//   - one unlimited-quota API key bound to that group.
//
// The contributed account is ALSO left in whatever marketplace groups the
// contribution flow bound it to, so other users can still consume it (and the
// owner still earns contribution rewards). The two are separated by which group
// the request's API key belongs to; billing/scheduling need no changes.
//
// All steps are idempotent, so a user's second contribution just adds the new
// account to their existing self-use group.

// selfUseSubscriptionPort is the narrow subscription surface used by self-use
// provisioning. Satisfied by *SubscriptionService.
type selfUseSubscriptionPort interface {
	ListActiveUserSubscriptions(ctx context.Context, userID int64) ([]UserSubscription, error)
	AssignOrExtendSubscription(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, bool, error)
	GetActiveSubscription(ctx context.Context, userID, groupID int64) (*UserSubscription, error)
	RevokeSubscription(ctx context.Context, subscriptionID int64) error
}

// selfUseAPIKeyPort is the narrow API-key surface used by self-use provisioning.
// Satisfied by *APIKeyService.
type selfUseAPIKeyPort interface {
	List(ctx context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error)
	Create(ctx context.Context, userID int64, req CreateAPIKeyRequest) (*APIKey, error)
	Delete(ctx context.Context, id int64, userID int64) error
}

const (
	// selfUseGroupName is the display name of the auto-managed per-user self-use
	// group. The group is hidden from user-facing surfaces, so a fixed name is fine.
	selfUseGroupName = "自用直连（系统自动）"
	// selfUseAPIKeyName names the auto-provisioned self-use API key.
	selfUseAPIKeyName = "自用直连"
	// selfUseSubscriptionValidityDays is ~100 years: the subscription is revoked
	// on withdrawal rather than left to expire (capped at MaxValidityDays=36500).
	selfUseSubscriptionValidityDays = 36500
	// selfUseSubscriptionNote marks the auto-assigned self-use subscription.
	selfUseSubscriptionNote = "auto: contributor self-use"
)

// selfUseProvisioningEnabled reports whether the optional subscription + API-key
// deps have been wired. When false, all self-use provisioning is a no-op and
// account contribution behaves exactly as before.
func (s *AccountService) selfUseProvisioningEnabled() bool {
	return s != nil && s.subscriptionSvc != nil && s.apiKeyService != nil
}

// ensureSelfUseAccess guarantees the contributing user has a self-use
// group + subscription + API key, and binds `account` to that group (additive,
// leaving marketplace memberships untouched). Idempotent across contributions.
func (s *AccountService) ensureSelfUseAccess(ctx context.Context, userID int64, account *Account) error {
	if !s.selfUseProvisioningEnabled() || userID <= 0 || account == nil || account.ID <= 0 {
		return nil
	}

	groupID, found, err := s.findUserSelfUseGroupID(ctx, userID)
	if err != nil {
		return fmt.Errorf("lookup self-use group: %w", err)
	}
	if !found {
		groupID, err = s.createSelfUseGroup(ctx, account.Platform)
		if err != nil {
			return fmt.Errorf("create self-use group: %w", err)
		}
	}

	// Additive + idempotent (INSERT ... ON CONFLICT DO NOTHING). Never touches the
	// account's marketplace-group memberships.
	if err := s.groupRepo.BindAccountsToGroup(ctx, groupID, []int64{account.ID}); err != nil {
		return fmt.Errorf("bind account to self-use group: %w", err)
	}

	// The subscription must exist BEFORE the API key can bind a subscription-type
	// group (APIKeyService.Create requires an active subscription for such groups).
	if _, _, err := s.subscriptionSvc.AssignOrExtendSubscription(ctx, &AssignSubscriptionInput{
		UserID:       userID,
		GroupID:      groupID,
		ValidityDays: selfUseSubscriptionValidityDays,
		Notes:        selfUseSubscriptionNote,
	}); err != nil {
		return fmt.Errorf("assign self-use subscription: %w", err)
	}

	if err := s.ensureSelfUseAPIKey(ctx, userID, groupID); err != nil {
		return fmt.Errorf("ensure self-use api key: %w", err)
	}
	return nil
}

// findUserSelfUseGroupID resolves the user's existing self-use group (if any) via
// their active subscriptions → the group flagged auto_self_use. A user has at
// most one such group, so the first match is authoritative.
func (s *AccountService) findUserSelfUseGroupID(ctx context.Context, userID int64) (int64, bool, error) {
	subs, err := s.subscriptionSvc.ListActiveUserSubscriptions(ctx, userID)
	if err != nil {
		return 0, false, err
	}
	for i := range subs {
		g, err := s.groupRepo.GetByIDLite(ctx, subs[i].GroupID)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				continue
			}
			return 0, false, err
		}
		if g != nil && g.AutoSelfUse {
			return g.ID, true, nil
		}
	}
	return 0, false, nil
}

// createSelfUseGroup creates the per-user self-use group: subscription-type,
// unlimited (nil limits), image generation enabled, flagged auto_self_use so it
// is hidden from user-facing surfaces.
func (s *AccountService) createSelfUseGroup(ctx context.Context, platform string) (int64, error) {
	if platform == "" {
		platform = PlatformOpenAI
	}
	group := &Group{
		Name:                 selfUseGroupName,
		Description:          "系统自动创建：贡献者自用专属分组（仅含本人贡献的账号，按订阅计费、不扣平台余额）",
		Platform:             platform,
		RateMultiplier:       1.0,
		Status:               StatusActive,
		SubscriptionType:     SubscriptionTypeSubscription,
		AutoSelfUse:          true,
		DefaultValidityDays:  selfUseSubscriptionValidityDays,
		AllowImageGeneration: true,
		ImageRateMultiplier:  1.0,
		// DailyLimitUSD/WeeklyLimitUSD/MonthlyLimitUSD left nil => unlimited.
	}
	if err := s.groupRepo.Create(ctx, group); err != nil {
		return 0, err
	}
	return group.ID, nil
}

// ensureSelfUseAPIKey creates the unlimited-quota self-use API key bound to the
// group, unless the user already has a key on it.
func (s *AccountService) ensureSelfUseAPIKey(ctx context.Context, userID, groupID int64) error {
	existing, _, err := s.apiKeyService.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 1}, APIKeyListFilters{GroupID: &groupID})
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	if _, err := s.apiKeyService.Create(ctx, userID, CreateAPIKeyRequest{
		Name:    selfUseAPIKeyName,
		GroupID: &groupID,
		Quota:   0, // unlimited: subscription-billed, platform balance untouched
	}); err != nil {
		return err
	}
	return nil
}

// teardownSelfUseAccessIfEmpty revokes the user's self-use subscription, deletes
// the self-use API key(s), and soft-deletes the self-use group once the user has
// no remaining (non-deleted) contributed accounts in it. No-op otherwise, and
// idempotent. Best-effort: callers log but do not fail the delete on error.
func (s *AccountService) teardownSelfUseAccessIfEmpty(ctx context.Context, userID int64) error {
	if !s.selfUseProvisioningEnabled() || userID <= 0 {
		return nil
	}
	groupID, found, err := s.findUserSelfUseGroupID(ctx, userID)
	if err != nil {
		return fmt.Errorf("lookup self-use group: %w", err)
	}
	if !found {
		return nil
	}
	// total counts only non-deleted accounts bound to the group, so a just
	// soft-deleted account is already excluded here.
	total, _, err := s.groupRepo.GetAccountCount(ctx, groupID)
	if err != nil {
		return fmt.Errorf("count self-use group accounts: %w", err)
	}
	if total > 0 {
		return nil // other contributed accounts remain — keep the self-use setup
	}

	// No live accounts left. Revoking the subscription alone already makes the key
	// inert (auth returns SUBSCRIPTION_NOT_FOUND); we still delete the key and drop
	// the group for hygiene.
	if sub, err := s.subscriptionSvc.GetActiveSubscription(ctx, userID, groupID); err == nil && sub != nil {
		if err := s.subscriptionSvc.RevokeSubscription(ctx, sub.ID); err != nil {
			return fmt.Errorf("revoke self-use subscription: %w", err)
		}
	}
	if err := s.deleteSelfUseAPIKeys(ctx, userID, groupID); err != nil {
		return fmt.Errorf("delete self-use api keys: %w", err)
	}
	if err := s.groupRepo.Delete(ctx, groupID); err != nil {
		return fmt.Errorf("delete self-use group: %w", err)
	}
	return nil
}

func (s *AccountService) deleteSelfUseAPIKeys(ctx context.Context, userID, groupID int64) error {
	keys, _, err := s.apiKeyService.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 100}, APIKeyListFilters{GroupID: &groupID})
	if err != nil {
		return err
	}
	for i := range keys {
		if err := s.apiKeyService.Delete(ctx, keys[i].ID, userID); err != nil {
			return err
		}
	}
	return nil
}

// BackfillSelfUseForUser provisions self-use access for a contributor's EXISTING
// OAuth accounts — those contributed before this feature shipped, which never ran
// through the on-contribution hook. Idempotent (reuses ensureSelfUseAccess), so it
// is safe to re-run. Returns the number of owned accounts (re)provisioned.
func (s *AccountService) BackfillSelfUseForUser(ctx context.Context, ownerUserID int64) (int, error) {
	if !s.selfUseProvisioningEnabled() {
		return 0, errors.New("self-use provisioning is not wired (subscription/api-key services missing)")
	}
	if ownerUserID <= 0 {
		return 0, errors.New("owner user id required")
	}
	// User contribution is OpenAI-OAuth only (see isUserOAuthPlatformAllowed), so
	// that is the only platform to scan.
	accounts, err := s.ListOwnedUserOAuthAccounts(ctx, ownerUserID, PlatformOpenAI)
	if err != nil {
		return 0, err
	}
	provisioned := 0
	for i := range accounts {
		acc := accounts[i]
		if err := s.ensureSelfUseAccess(ctx, ownerUserID, &acc); err != nil {
			return provisioned, fmt.Errorf("backfill self-use for account %d: %w", acc.ID, err)
		}
		provisioned++
	}
	return provisioned, nil
}

// logSelfUseProvisioningError records a non-fatal provisioning failure. Account
// contribution succeeds regardless — the account still serves the marketplace;
// only the owner's private self-use path is (temporarily) unavailable and will
// be re-attempted on the owner's next contribution.
func logSelfUseProvisioningError(userID, accountID int64, err error) {
	slog.Warn("contributor self-use provisioning failed; account still contributed",
		"user_id", userID,
		"account_id", accountID,
		"error", err,
	)
}

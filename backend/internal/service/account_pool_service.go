package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const maxUserAccountLimitUSD = 1_000_000

var (
	ErrAccountOwnerRequired = infraerrors.Forbidden("ACCOUNT_OWNER_REQUIRED", "account is not owned by current user")
	ErrUserOAuthOnly        = infraerrors.BadRequest("ACCOUNT_TYPE_NOT_ALLOWED", "only OAuth accounts are supported")
)

type UserAccountPoolListFilters struct {
	Platform string
	Search   string
}

type UserAccountPoolItem struct {
	ID                                 int64      `json:"id"`
	Name                               string     `json:"name"`
	Platform                           string     `json:"platform"`
	Type                               string     `json:"type"`
	Status                             string     `json:"status"`
	IsMine                             bool       `json:"is_mine"`
	IsUserContributed                  bool       `json:"is_user_contributed"`
	Schedulable                        bool       `json:"schedulable"`
	EffectiveSchedulable               bool       `json:"effective_schedulable"`
	ContributionFiveHourReservePercent float64    `json:"contribution_5h_reserve_percent"`
	ContributionWeeklyReservePercent   float64    `json:"contribution_weekly_reserve_percent"`
	ContributionProbeFailurePolicy     string     `json:"contribution_probe_failure_policy"`
	ContributionFiveHourUsagePercent   *float64   `json:"contribution_5h_usage_percent,omitempty"`
	ContributionWeeklyUsagePercent     *float64   `json:"contribution_weekly_usage_percent,omitempty"`
	ContributionProtectionBlocked      bool       `json:"contribution_protection_blocked"`
	ContributionProtectionReason       string     `json:"contribution_protection_reason,omitempty"`
	WindowCostLimit                    float64    `json:"window_cost_limit"`
	WindowCostStickyReserve            float64    `json:"window_cost_sticky_reserve"`
	CurrentWindowCost                  *float64   `json:"current_window_cost,omitempty"`
	QuotaWeeklyLimit                   float64    `json:"quota_weekly_limit"`
	QuotaWeeklyUsed                    float64    `json:"quota_weekly_used"`
	QuotaWeeklyRemaining               float64    `json:"quota_weekly_remaining"`
	QuotaWeeklyMinRemaining            float64    `json:"quota_weekly_min_remaining"`
	WeeklyRemainingBelowPolicy         bool       `json:"weekly_remaining_below_policy"`
	CreatedAt                          time.Time  `json:"created_at"`
	UpdatedAt                          time.Time  `json:"updated_at"`
	WindowCostStart                    *time.Time `json:"-"`
}

type CreateUserOAuthAccountRequest struct {
	Name                               string         `json:"name"`
	Platform                           string         `json:"platform"`
	Type                               string         `json:"type"`
	Credentials                        map[string]any `json:"credentials"`
	Extra                              map[string]any `json:"extra"`
	Schedulable                        *bool          `json:"schedulable"`
	ContributionFiveHourReservePercent *float64       `json:"contribution_5h_reserve_percent"`
	ContributionWeeklyReservePercent   *float64       `json:"contribution_weekly_reserve_percent"`
	ContributionProbeFailurePolicy     *string        `json:"contribution_probe_failure_policy"`
	WindowCostLimit                    *float64       `json:"window_cost_limit"`
	WindowCostStickyReserve            *float64       `json:"window_cost_sticky_reserve"`
	QuotaWeeklyLimit                   *float64       `json:"quota_weekly_limit"`
	QuotaWeeklyMinRemaining            *float64       `json:"quota_weekly_min_remaining"`
}

type UpdateUserAccountLimitsRequest struct {
	ContributionFiveHourReservePercent *float64 `json:"contribution_5h_reserve_percent"`
	ContributionWeeklyReservePercent   *float64 `json:"contribution_weekly_reserve_percent"`
	ContributionProbeFailurePolicy     *string  `json:"contribution_probe_failure_policy"`
	WindowCostLimit                    *float64 `json:"window_cost_limit"`
	WindowCostStickyReserve            *float64 `json:"window_cost_sticky_reserve"`
	QuotaWeeklyLimit                   *float64 `json:"quota_weekly_limit"`
	QuotaWeeklyMinRemaining            *float64 `json:"quota_weekly_min_remaining"`
}

func (s *AccountService) ListUserAccountPool(ctx context.Context, userID int64, params pagination.PaginationParams, filters UserAccountPoolListFilters) ([]UserAccountPoolItem, *pagination.PaginationResult, error) {
	if userID <= 0 {
		return nil, nil, infraerrors.Unauthorized("USER_NOT_AUTHENTICATED", "user not authenticated")
	}

	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 50
	}
	if strings.TrimSpace(params.SortBy) == "" {
		params.SortBy = "name"
	}
	if strings.TrimSpace(params.SortOrder) == "" {
		params.SortOrder = pagination.SortOrderAsc
	}

	accounts, result, err := s.accountRepo.ListWithFilters(
		ctx,
		params,
		strings.TrimSpace(filters.Platform),
		AccountTypeOAuth,
		"",
		strings.TrimSpace(filters.Search),
		0,
		"",
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list account pool: %w", err)
	}

	items := make([]UserAccountPoolItem, 0, len(accounts))
	for i := range accounts {
		items = append(items, accountToUserPoolItem(&accounts[i], userID))
	}
	return items, result, nil
}

func (s *AccountService) CreateUserOAuthAccount(ctx context.Context, userID int64, req CreateUserOAuthAccountRequest) (*UserAccountPoolItem, error) {
	if userID <= 0 {
		return nil, infraerrors.Unauthorized("USER_NOT_AUTHENTICATED", "user not authenticated")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, infraerrors.BadRequest("ACCOUNT_NAME_REQUIRED", "account name is required")
	}

	platform := strings.TrimSpace(req.Platform)
	if !isUserOAuthPlatformAllowed(platform) {
		return nil, infraerrors.BadRequest("ACCOUNT_PLATFORM_NOT_ALLOWED", "unsupported OAuth account platform")
	}

	if accountType := strings.TrimSpace(req.Type); accountType != "" && accountType != AccountTypeOAuth {
		return nil, ErrUserOAuthOnly
	}
	if len(req.Credentials) == 0 {
		return nil, infraerrors.BadRequest("ACCOUNT_CREDENTIALS_REQUIRED", "OAuth credentials are required")
	}

	limitUpdates, err := buildUserAccountLimitUpdates(req.Extra, UpdateUserAccountLimitsRequest{
		ContributionFiveHourReservePercent: req.ContributionFiveHourReservePercent,
		ContributionWeeklyReservePercent:   req.ContributionWeeklyReservePercent,
		ContributionProbeFailurePolicy:     req.ContributionProbeFailurePolicy,
		WindowCostLimit:                    req.WindowCostLimit,
		WindowCostStickyReserve:            req.WindowCostStickyReserve,
		QuotaWeeklyLimit:                   req.QuotaWeeklyLimit,
		QuotaWeeklyMinRemaining:            req.QuotaWeeklyMinRemaining,
	})
	if err != nil {
		return nil, err
	}

	schedulable := true
	if req.Schedulable != nil {
		schedulable = *req.Schedulable
	}

	ownerUserID := userID
	account := &Account{
		Name:               name,
		Platform:           platform,
		Type:               AccountTypeOAuth,
		Credentials:        req.Credentials,
		Extra:              limitUpdates,
		OwnerUserID:        &ownerUserID,
		Concurrency:        1,
		Priority:           100,
		Status:             StatusActive,
		Schedulable:        schedulable,
		AutoPauseOnExpired: true,
	}

	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, fmt.Errorf("create user OAuth account: %w", err)
	}
	item := accountToUserPoolItem(account, userID)
	return &item, nil
}

func (s *AccountService) SetUserAccountSchedulable(ctx context.Context, userID, accountID int64, schedulable bool) (*UserAccountPoolItem, error) {
	account, err := s.getOwnedUserOAuthAccount(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}
	account.Schedulable = schedulable

	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, fmt.Errorf("update user account schedulable: %w", err)
	}
	item := accountToUserPoolItem(account, userID)
	return &item, nil
}

func (s *AccountService) UpdateUserAccountLimits(ctx context.Context, userID, accountID int64, req UpdateUserAccountLimitsRequest) (*UserAccountPoolItem, error) {
	account, err := s.getOwnedUserOAuthAccount(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}

	updates, err := buildUserAccountLimitUpdates(nil, req)
	if err != nil {
		return nil, err
	}
	if len(updates) > 0 {
		if err := s.accountRepo.UpdateExtra(ctx, account.ID, updates); err != nil {
			return nil, fmt.Errorf("update user account limits: %w", err)
		}
		if account.Extra == nil {
			account.Extra = map[string]any{}
		}
		for key, value := range updates {
			account.Extra[key] = value
		}
	}

	item := accountToUserPoolItem(account, userID)
	return &item, nil
}

func (s *AccountService) getOwnedUserOAuthAccount(ctx context.Context, userID, accountID int64) (*Account, error) {
	if userID <= 0 {
		return nil, infraerrors.Unauthorized("USER_NOT_AUTHENTICATED", "user not authenticated")
	}
	if accountID <= 0 {
		return nil, infraerrors.BadRequest("ACCOUNT_ID_INVALID", "invalid account ID")
	}

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	if account.OwnerUserID == nil || *account.OwnerUserID != userID {
		return nil, ErrAccountOwnerRequired
	}
	if account.Type != AccountTypeOAuth {
		return nil, ErrUserOAuthOnly
	}
	return account, nil
}

func accountToUserPoolItem(account *Account, currentUserID int64) UserAccountPoolItem {
	if account == nil {
		return UserAccountPoolItem{}
	}
	isMine := account.OwnerUserID != nil && *account.OwnerUserID == currentUserID
	windowCostLimit := account.GetWindowCostLimit()
	windowCostStickyReserve := 0.0
	if windowCostLimit > 0 {
		windowCostStickyReserve = account.GetWindowCostStickyReserve()
	}
	protection := account.EvaluateContributionProtection()
	item := UserAccountPoolItem{
		ID:                                 account.ID,
		Name:                               account.Name,
		Platform:                           account.Platform,
		Type:                               account.Type,
		Status:                             account.Status,
		IsMine:                             isMine,
		IsUserContributed:                  account.IsUserContributed(),
		Schedulable:                        account.Schedulable,
		EffectiveSchedulable:               account.IsSchedulable() && !protection.Blocked && !account.IsWeeklyRemainingBelowThreshold(),
		ContributionFiveHourReservePercent: account.GetContributionFiveHourReservePercent(),
		ContributionWeeklyReservePercent:   account.GetContributionWeeklyReservePercent(),
		ContributionProbeFailurePolicy:     account.GetContributionProbeFailurePolicy(),
		ContributionFiveHourUsagePercent:   protection.FiveHourUsagePercent,
		ContributionWeeklyUsagePercent:     protection.WeeklyUsagePercent,
		ContributionProtectionBlocked:      protection.Blocked,
		ContributionProtectionReason:       protection.Reason,
		WindowCostLimit:                    windowCostLimit,
		WindowCostStickyReserve:            windowCostStickyReserve,
		QuotaWeeklyLimit:                   account.GetQuotaWeeklyLimit(),
		QuotaWeeklyUsed:                    account.GetEffectiveQuotaWeeklyUsed(),
		QuotaWeeklyRemaining:               account.GetQuotaWeeklyRemaining(),
		QuotaWeeklyMinRemaining:            account.GetQuotaWeeklyMinRemaining(),
		WeeklyRemainingBelowPolicy:         account.IsWeeklyRemainingBelowThreshold(),
		CreatedAt:                          account.CreatedAt,
		UpdatedAt:                          account.UpdatedAt,
	}
	if windowCostLimit > 0 {
		start := account.GetCurrentWindowStartTime()
		item.WindowCostStart = &start
	}
	return item
}

func isUserOAuthPlatformAllowed(platform string) bool {
	switch platform {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity:
		return true
	default:
		return false
	}
}

func buildUserAccountLimitUpdates(extra map[string]any, req UpdateUserAccountLimitsRequest) (map[string]any, error) {
	updates := make(map[string]any, 4)
	if extra != nil {
		for _, key := range []string{
			"contribution_5h_reserve_percent",
			"contribution_weekly_reserve_percent",
			"contribution_probe_failure_policy",
			"window_cost_limit",
			"window_cost_sticky_reserve",
			"quota_weekly_limit",
			"quota_weekly_min_remaining",
			"weekly_remaining_threshold",
		} {
			if key == "contribution_probe_failure_policy" {
				raw, ok := extra[key]
				if !ok {
					continue
				}
				value, err := parseContributionProbeFailurePolicy(raw)
				if err != nil {
					return nil, err
				}
				updates[key] = value
				continue
			}
			value, ok, err := parseUserLimitFromMap(extra, key)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			targetKey := key
			if key == "weekly_remaining_threshold" {
				targetKey = "quota_weekly_min_remaining"
			}
			updates[targetKey] = value
		}
	}

	if err := setUserLimitUpdate(updates, "contribution_5h_reserve_percent", req.ContributionFiveHourReservePercent); err != nil {
		return nil, err
	}
	if err := setUserLimitUpdate(updates, "contribution_weekly_reserve_percent", req.ContributionWeeklyReservePercent); err != nil {
		return nil, err
	}
	if req.ContributionProbeFailurePolicy != nil {
		policy, err := normalizeContributionProbeFailurePolicy(*req.ContributionProbeFailurePolicy)
		if err != nil {
			return nil, err
		}
		updates["contribution_probe_failure_policy"] = policy
	}
	if err := setUserLimitUpdate(updates, "window_cost_limit", req.WindowCostLimit); err != nil {
		return nil, err
	}
	if err := setUserLimitUpdate(updates, "window_cost_sticky_reserve", req.WindowCostStickyReserve); err != nil {
		return nil, err
	}
	if err := setUserLimitUpdate(updates, "quota_weekly_limit", req.QuotaWeeklyLimit); err != nil {
		return nil, err
	}
	if err := setUserLimitUpdate(updates, "quota_weekly_min_remaining", req.QuotaWeeklyMinRemaining); err != nil {
		return nil, err
	}
	return updates, nil
}

func setUserLimitUpdate(updates map[string]any, key string, value *float64) error {
	if value == nil {
		return nil
	}
	if err := validateUserAccountLimit(key, *value); err != nil {
		return err
	}
	updates[key] = *value
	return nil
}

func parseUserLimitFromMap(extra map[string]any, key string) (float64, bool, error) {
	raw, ok := extra[key]
	if !ok {
		return 0, false, nil
	}
	value, err := parseUserAccountLimitValue(raw)
	if err != nil {
		return 0, true, infraerrors.BadRequest("ACCOUNT_LIMIT_INVALID", fmt.Sprintf("%s must be a number", key))
	}
	if err := validateUserAccountLimit(key, value); err != nil {
		return 0, true, err
	}
	return value, true, nil
}

func parseUserAccountLimitValue(raw any) (float64, error) {
	switch v := raw.(type) {
	case nil:
		return 0, nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		return strconvParseUserLimit(v)
	default:
		return 0, fmt.Errorf("unsupported limit type %T", raw)
	}
}

func strconvParseUserLimit(value string) (float64, error) {
	value = strings.TrimSpace(value)
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func validateUserAccountLimit(name string, value float64) error {
	if name == "contribution_5h_reserve_percent" || name == "contribution_weekly_reserve_percent" {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
			return infraerrors.BadRequest("ACCOUNT_LIMIT_OUT_OF_RANGE", fmt.Sprintf("%s must be between 0 and 100", name))
		}
		return nil
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > maxUserAccountLimitUSD {
		return infraerrors.BadRequest("ACCOUNT_LIMIT_OUT_OF_RANGE", fmt.Sprintf("%s must be between 0 and %.0f", name, float64(maxUserAccountLimitUSD)))
	}
	return nil
}

func parseContributionProbeFailurePolicy(raw any) (string, error) {
	if raw == nil {
		return ContributionProbeFailurePolicyContinue, nil
	}
	switch v := raw.(type) {
	case string:
		return normalizeContributionProbeFailurePolicy(v)
	default:
		return "", infraerrors.BadRequest("ACCOUNT_LIMIT_INVALID", "contribution_probe_failure_policy must be a string")
	}
}

func normalizeContributionProbeFailurePolicy(value string) (string, error) {
	policy := strings.TrimSpace(value)
	if policy == "" {
		return ContributionProbeFailurePolicyContinue, nil
	}
	switch policy {
	case ContributionProbeFailurePolicyContinue, ContributionProbeFailurePolicyPause, ContributionProbeFailurePolicyLocal:
		return policy, nil
	default:
		return "", infraerrors.BadRequest("ACCOUNT_LIMIT_INVALID", "contribution_probe_failure_policy must be continue, pause, or local")
	}
}

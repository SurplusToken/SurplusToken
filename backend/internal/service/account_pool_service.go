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
	PlanType string
	Search   string
}

type UserAccountPoolItem struct {
	ID                                 int64                  `json:"id"`
	Name                               string                 `json:"name"`
	Platform                           string                 `json:"platform"`
	Type                               string                 `json:"type"`
	PlanType                           string                 `json:"plan_type,omitempty"`
	PrivacyMode                        string                 `json:"privacy_mode,omitempty"`
	SubscriptionExpiresAt              string                 `json:"subscription_expires_at,omitempty"`
	ProxyID                            *int64                 `json:"proxy_id,omitempty"`
	Proxy                              *Proxy                 `json:"proxy,omitempty"`
	Status                             string                 `json:"status"`
	IsMine                             bool                   `json:"is_mine"`
	IsUserContributed                  bool                   `json:"is_user_contributed"`
	Schedulable                        bool                   `json:"schedulable"`
	EffectiveSchedulable               bool                   `json:"effective_schedulable"`
	GroupIDs                           []int64                `json:"group_ids,omitempty"`
	Groups                             []UserAccountPoolGroup `json:"groups,omitempty"`
	ExpiresAt                          *time.Time             `json:"expires_at,omitempty"`
	AutoPauseOnExpired                 bool                   `json:"auto_pause_on_expired"`
	ModelMapping                       map[string]string      `json:"model_mapping,omitempty"`
	CodexCLIOnly                       bool                   `json:"codex_cli_only"`
	ContributionFiveHourReservePercent float64                `json:"contribution_5h_reserve_percent"`
	ContributionWeeklyReservePercent   float64                `json:"contribution_weekly_reserve_percent"`
	ContributionProbeFailurePolicy     string                 `json:"contribution_probe_failure_policy"`
	ContributionFiveHourUsagePercent   *float64               `json:"contribution_5h_usage_percent,omitempty"`
	ContributionWeeklyUsagePercent     *float64               `json:"contribution_weekly_usage_percent,omitempty"`
	ContributionProtectionBlocked      bool                   `json:"contribution_protection_blocked"`
	ContributionProtectionReason       string                 `json:"contribution_protection_reason,omitempty"`
	WindowCostLimit                    float64                `json:"window_cost_limit"`
	WindowCostStickyReserve            float64                `json:"window_cost_sticky_reserve"`
	CurrentWindowCost                  *float64               `json:"current_window_cost,omitempty"`
	QuotaWeeklyLimit                   float64                `json:"quota_weekly_limit"`
	QuotaWeeklyUsed                    float64                `json:"quota_weekly_used"`
	QuotaWeeklyRemaining               float64                `json:"quota_weekly_remaining"`
	QuotaWeeklyMinRemaining            float64                `json:"quota_weekly_min_remaining"`
	WeeklyRemainingBelowPolicy         bool                   `json:"weekly_remaining_below_policy"`
	CreatedAt                          time.Time              `json:"created_at"`
	UpdatedAt                          time.Time              `json:"updated_at"`
	WindowCostStart                    *time.Time             `json:"-"`
}

type userAccountPoolLister interface {
	ListUserAccountPoolWithFilters(ctx context.Context, params pagination.PaginationParams, platform, planType, search string) ([]Account, *pagination.PaginationResult, error)
}

type UserAccountPoolGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

type CreateUserOAuthAccountRequest struct {
	Name                               string             `json:"name"`
	Platform                           string             `json:"platform"`
	Type                               string             `json:"type"`
	Credentials                        map[string]any     `json:"credentials"`
	ModelMapping                       *map[string]string `json:"model_mapping"`
	Extra                              map[string]any     `json:"extra"`
	ProxyID                            *int64             `json:"proxy_id"`
	Schedulable                        *bool              `json:"schedulable"`
	GroupIDs                           []int64            `json:"group_ids"`
	ExpiresAt                          *int64             `json:"expires_at"`
	AutoPauseOnExpired                 *bool              `json:"auto_pause_on_expired"`
	ContributionFiveHourReservePercent *float64           `json:"contribution_5h_reserve_percent"`
	ContributionWeeklyReservePercent   *float64           `json:"contribution_weekly_reserve_percent"`
	ContributionProbeFailurePolicy     *string            `json:"contribution_probe_failure_policy"`
}

type UpdateUserAccountScopeRequest struct {
	GroupIDs                           *[]int64           `json:"group_ids"`
	ProxyID                            *int64             `json:"proxy_id"`
	ExpiresAt                          *int64             `json:"expires_at"`
	AutoPauseOnExpired                 *bool              `json:"auto_pause_on_expired"`
	ModelMapping                       *map[string]string `json:"model_mapping"`
	CodexCLIOnly                       *bool              `json:"codex_cli_only"`
	ContributionFiveHourReservePercent *float64           `json:"contribution_5h_reserve_percent"`
	ContributionWeeklyReservePercent   *float64           `json:"contribution_weekly_reserve_percent"`
	ContributionProbeFailurePolicy     *string            `json:"contribution_probe_failure_policy"`
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

	platform := strings.TrimSpace(filters.Platform)
	if platform != "" && !isUserAccountPoolPlatformVisible(platform) {
		return []UserAccountPoolItem{}, emptyUserAccountPoolPagination(params), nil
	}
	planType := strings.TrimSpace(filters.PlanType)
	search := strings.TrimSpace(filters.Search)

	var accounts []Account
	var result *pagination.PaginationResult
	var err error
	if lister, ok := s.accountRepo.(userAccountPoolLister); ok {
		accounts, result, err = lister.ListUserAccountPoolWithFilters(ctx, params, platform, planType, search)
	} else {
		accounts, result, err = s.accountRepo.ListWithFilters(ctx, params, platform, AccountTypeOAuth, "", search, 0, "")
		if err == nil {
			accounts = filterUserAccountPoolAccountsByAllowedPlatform(accounts)
			if planType != "" {
				accounts = filterUserAccountPoolAccountsByPlanType(accounts, planType)
			}
			result = paginationResultFromFilteredAccounts(result, int64(len(accounts)))
		}
	}
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

	extra := buildUserOAuthAccountExtra(platform, req.Extra)
	protectionUpdates, err := buildUserContributionProtectionUpdates(
		req.ContributionFiveHourReservePercent,
		req.ContributionWeeklyReservePercent,
		req.ContributionProbeFailurePolicy,
	)
	if err != nil {
		return nil, err
	}
	for key, value := range protectionUpdates {
		extra[key] = value
	}

	schedulable := true
	if req.Schedulable != nil {
		schedulable = *req.Schedulable
	}

	autoPauseOnExpired := true
	if req.AutoPauseOnExpired != nil {
		autoPauseOnExpired = *req.AutoPauseOnExpired
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt > 0 {
		t := time.Unix(*req.ExpiresAt, 0)
		expiresAt = &t
	}

	credentials := cloneCredentials(req.Credentials)
	if req.ModelMapping != nil {
		mapping := normalizeUserModelMapping(*req.ModelMapping)
		if len(mapping) > 0 {
			credentials["model_mapping"] = mapping
		} else {
			delete(credentials, "model_mapping")
		}
	}

	ownerUserID := userID
	account := &Account{
		Name:               name,
		Platform:           platform,
		Type:               AccountTypeOAuth,
		Credentials:        credentials,
		Extra:              extra,
		ProxyID:            normalizeUserAccountProxyID(req.ProxyID),
		OwnerUserID:        &ownerUserID,
		Concurrency:        1,
		Priority:           100,
		Status:             StatusActive,
		Schedulable:        schedulable,
		ExpiresAt:          expiresAt,
		AutoPauseOnExpired: autoPauseOnExpired,
	}
	if err := s.validateUserAccountProxy(ctx, account.ProxyID); err != nil {
		return nil, err
	}

	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, fmt.Errorf("create user OAuth account: %w", err)
	}
	if len(req.GroupIDs) > 0 {
		if err := s.accountRepo.BindGroups(ctx, account.ID, req.GroupIDs); err != nil {
			return nil, fmt.Errorf("bind user account groups: %w", err)
		}
		account.GroupIDs = append([]int64(nil), req.GroupIDs...)
	}
	item := accountToUserPoolItem(account, userID)
	return &item, nil
}

func buildUserOAuthAccountExtra(platform string, rawExtra map[string]any) map[string]any {
	extra := make(map[string]any, 4)
	if rawExtra == nil {
		return extra
	}
	for _, key := range []string{"email", "name", "privacy_mode"} {
		if value, ok := safeUserOAuthExtraString(rawExtra[key]); ok {
			extra[key] = value
		}
	}
	if platform == PlatformOpenAI {
		if value, ok := rawExtra["codex_cli_only"].(bool); ok && value {
			extra["codex_cli_only"] = true
		}
	}
	return extra
}

func safeUserOAuthExtraString(raw any) (string, bool) {
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
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

func (s *AccountService) UpdateUserAccountScope(ctx context.Context, userID, accountID int64, req UpdateUserAccountScopeRequest) (*UserAccountPoolItem, error) {
	account, err := s.getOwnedUserOAuthAccount(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}

	if req.ModelMapping != nil {
		credentials := cloneCredentials(account.Credentials)
		mapping := normalizeUserModelMapping(*req.ModelMapping)
		if len(mapping) > 0 {
			credentials["model_mapping"] = mapping
		} else {
			delete(credentials, "model_mapping")
		}
		if err := persistAccountCredentials(ctx, s.accountRepo, account, credentials); err != nil {
			return nil, fmt.Errorf("update user account model scope: %w", err)
		}
	}

	extraChanged := false
	if req.CodexCLIOnly != nil && account.Platform == PlatformOpenAI {
		if account.Extra == nil {
			account.Extra = map[string]any{}
		}
		if *req.CodexCLIOnly {
			account.Extra["codex_cli_only"] = true
		} else {
			delete(account.Extra, "codex_cli_only")
		}
		extraChanged = true
	}
	protectionUpdates, err := buildUserContributionProtectionUpdates(
		req.ContributionFiveHourReservePercent,
		req.ContributionWeeklyReservePercent,
		req.ContributionProbeFailurePolicy,
	)
	if err != nil {
		return nil, err
	}
	if len(protectionUpdates) > 0 {
		if account.Extra == nil {
			account.Extra = map[string]any{}
		}
		for key, value := range protectionUpdates {
			account.Extra[key] = value
		}
		extraChanged = true
	}

	if req.ExpiresAt != nil {
		if *req.ExpiresAt > 0 {
			t := time.Unix(*req.ExpiresAt, 0)
			account.ExpiresAt = &t
		} else {
			account.ExpiresAt = nil
		}
	}
	if req.AutoPauseOnExpired != nil {
		account.AutoPauseOnExpired = *req.AutoPauseOnExpired
	}
	if req.ProxyID != nil {
		account.ProxyID = normalizeUserAccountProxyID(req.ProxyID)
		account.Proxy = nil
		if err := s.validateUserAccountProxy(ctx, account.ProxyID); err != nil {
			return nil, err
		}
	}

	needsAccountUpdate := extraChanged || req.ProxyID != nil || req.ExpiresAt != nil || req.AutoPauseOnExpired != nil
	if needsAccountUpdate {
		if err := s.accountRepo.Update(ctx, account); err != nil {
			return nil, fmt.Errorf("update user account scope: %w", err)
		}
	}

	if req.GroupIDs != nil {
		if err := s.accountRepo.BindGroups(ctx, account.ID, *req.GroupIDs); err != nil {
			return nil, fmt.Errorf("bind user account groups: %w", err)
		}
		account.GroupIDs = append([]int64(nil), (*req.GroupIDs)...)
	}

	refreshed, err := s.accountRepo.GetByID(ctx, account.ID)
	if err == nil && refreshed != nil {
		account = refreshed
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
		PlanType:                           account.GetCredential("plan_type"),
		PrivacyMode:                        account.getExtraString("privacy_mode"),
		SubscriptionExpiresAt:              account.GetCredential("subscription_expires_at"),
		ProxyID:                            account.ProxyID,
		Status:                             account.Status,
		IsMine:                             isMine,
		IsUserContributed:                  account.IsUserContributed(),
		Schedulable:                        account.Schedulable,
		EffectiveSchedulable:               account.IsSchedulable() && !protection.Blocked && !account.IsWeeklyRemainingBelowThreshold(),
		ExpiresAt:                          account.ExpiresAt,
		AutoPauseOnExpired:                 account.AutoPauseOnExpired,
		ModelMapping:                       userVisibleModelMapping(account, isMine),
		CodexCLIOnly:                       isMine && account.IsCodexCLIOnlyEnabled(),
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
	if isMine {
		item.GroupIDs = append([]int64(nil), account.GroupIDs...)
		if account.Proxy != nil {
			proxy := *account.Proxy
			proxy.Password = ""
			item.Proxy = &proxy
		}
	}
	if isMine && len(account.Groups) > 0 {
		item.Groups = make([]UserAccountPoolGroup, 0, len(account.Groups))
		for _, group := range account.Groups {
			if group == nil {
				continue
			}
			item.Groups = append(item.Groups, UserAccountPoolGroup{
				ID:       group.ID,
				Name:     group.Name,
				Platform: group.Platform,
			})
		}
	}
	if windowCostLimit > 0 {
		start := account.GetCurrentWindowStartTime()
		item.WindowCostStart = &start
	}
	return item
}

func filterUserAccountPoolAccountsByPlanType(accounts []Account, planType string) []Account {
	normalized := normalizeUserAccountPoolPlanType(planType)
	if normalized == "" {
		return accounts
	}
	out := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if normalizeUserAccountPoolPlanType(account.GetCredential("plan_type")) == normalized {
			out = append(out, account)
		}
	}
	return out
}

func filterUserAccountPoolAccountsByAllowedPlatform(accounts []Account) []Account {
	out := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if isUserAccountPoolPlatformVisible(account.Platform) {
			out = append(out, account)
		}
	}
	return out
}

func normalizeUserAccountPoolPlanType(planType string) string {
	switch strings.ToLower(strings.TrimSpace(planType)) {
	case "plus":
		return "plus"
	case "pro", "chatgptpro":
		return "pro"
	default:
		return ""
	}
}

func paginationResultFromFilteredAccounts(base *pagination.PaginationResult, total int64) *pagination.PaginationResult {
	if base == nil {
		return &pagination.PaginationResult{Total: total}
	}
	copied := *base
	copied.Total = total
	if copied.PageSize > 0 {
		copied.Pages = int(math.Ceil(float64(total) / float64(copied.PageSize)))
	}
	return &copied
}

func emptyUserAccountPoolPagination(params pagination.PaginationParams) *pagination.PaginationResult {
	return &pagination.PaginationResult{
		Total:    0,
		Page:     params.Page,
		PageSize: params.Limit(),
		Pages:    0,
	}
}

func userVisibleModelMapping(account *Account, isMine bool) map[string]string {
	if !isMine || account == nil || account.Credentials == nil {
		return nil
	}
	raw, ok := account.Credentials["model_mapping"]
	if !ok || raw == nil {
		return nil
	}
	out := map[string]string{}
	switch v := raw.(type) {
	case map[string]string:
		for from, to := range v {
			from = strings.TrimSpace(from)
			to = strings.TrimSpace(to)
			if from != "" && to != "" {
				out[from] = to
			}
		}
	case map[string]any:
		for from, rawTo := range v {
			to, ok := rawTo.(string)
			if !ok {
				continue
			}
			from = strings.TrimSpace(from)
			to = strings.TrimSpace(to)
			if from != "" && to != "" {
				out[from] = to
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeUserModelMapping(input map[string]string) map[string]any {
	out := make(map[string]any, len(input))
	for from, to := range input {
		from = strings.TrimSpace(from)
		to = strings.TrimSpace(to)
		if from == "" || to == "" {
			continue
		}
		out[from] = to
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeUserAccountProxyID(proxyID *int64) *int64 {
	if proxyID == nil || *proxyID <= 0 {
		return nil
	}
	id := *proxyID
	return &id
}

func (s *AccountService) validateUserAccountProxy(ctx context.Context, proxyID *int64) error {
	if proxyID == nil {
		return nil
	}
	if s.proxyRepo == nil {
		return infraerrors.BadRequest("ACCOUNT_PROXY_NOT_AVAILABLE", "proxy service is not configured")
	}
	proxy, err := s.proxyRepo.GetByID(ctx, *proxyID)
	if err != nil {
		return fmt.Errorf("get proxy: %w", err)
	}
	if proxy == nil || !proxy.IsActive() {
		return infraerrors.BadRequest("ACCOUNT_PROXY_NOT_AVAILABLE", "proxy is not available")
	}
	return nil
}

func isUserOAuthPlatformAllowed(platform string) bool {
	switch platform {
	case PlatformOpenAI:
		return true
	default:
		return false
	}
}

func isUserAccountPoolPlatformVisible(platform string) bool {
	switch platform {
	case PlatformAnthropic, PlatformOpenAI:
		return true
	default:
		return false
	}
}

func buildUserContributionProtectionUpdates(fiveHourReserve, weeklyReserve *float64, probeFailurePolicy *string) (map[string]any, error) {
	updates := make(map[string]any, 3)
	if err := setUserLimitUpdate(updates, "contribution_5h_reserve_percent", fiveHourReserve); err != nil {
		return nil, err
	}
	if err := setUserLimitUpdate(updates, "contribution_weekly_reserve_percent", weeklyReserve); err != nil {
		return nil, err
	}
	if probeFailurePolicy != nil {
		policy, err := normalizeContributionProbeFailurePolicy(*probeFailurePolicy)
		if err != nil {
			return nil, err
		}
		updates["contribution_probe_failure_policy"] = policy
	}
	return updates, nil
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

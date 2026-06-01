package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type AccountPoolHandler struct {
	accountService          *service.AccountService
	apiKeyService           *service.APIKeyService
	accountUsageService     *service.AccountUsageService
	proxyService            *service.ProxyService
	openaiOAuthService      *service.OpenAIOAuthService
	geminiOAuthService      *service.GeminiOAuthService
	antigravityOAuthService *service.AntigravityOAuthService
	accountTestService      *service.AccountTestService
	scheduledTestService    *service.ScheduledTestService
	rateLimitService        *service.RateLimitService
	tokenCacheInvalidator   service.TokenCacheInvalidator
	privacyClientFactory    service.PrivacyClientFactory
}

func NewAccountPoolHandler(
	accountService *service.AccountService,
	apiKeyService *service.APIKeyService,
	accountUsageService *service.AccountUsageService,
	proxyService *service.ProxyService,
	openaiOAuthService *service.OpenAIOAuthService,
	geminiOAuthService *service.GeminiOAuthService,
	antigravityOAuthService *service.AntigravityOAuthService,
	accountTestService *service.AccountTestService,
	scheduledTestService *service.ScheduledTestService,
	rateLimitService *service.RateLimitService,
	tokenCacheInvalidator service.TokenCacheInvalidator,
	privacyClientFactory service.PrivacyClientFactory,
) *AccountPoolHandler {
	return &AccountPoolHandler{
		accountService:          accountService,
		apiKeyService:           apiKeyService,
		accountUsageService:     accountUsageService,
		proxyService:            proxyService,
		openaiOAuthService:      openaiOAuthService,
		geminiOAuthService:      geminiOAuthService,
		antigravityOAuthService: antigravityOAuthService,
		accountTestService:      accountTestService,
		scheduledTestService:    scheduledTestService,
		rateLimitService:        rateLimitService,
		tokenCacheInvalidator:   tokenCacheInvalidator,
		privacyClientFactory:    privacyClientFactory,
	}
}

type createUserOAuthAccountPayload struct {
	Name                               string             `json:"name"`
	Platform                           string             `json:"platform"`
	Type                               string             `json:"type"`
	Credentials                        map[string]any     `json:"credentials"`
	ModelMapping                       *map[string]string `json:"model_mapping"`
	Extra                              map[string]any     `json:"extra"`
	ProxyID                            *int64             `json:"proxy_id"`
	Concurrency                        *int               `json:"concurrency"`
	Schedulable                        *bool              `json:"schedulable"`
	GroupIDs                           []int64            `json:"group_ids"`
	ExpiresAt                          *int64             `json:"expires_at"`
	AutoPauseOnExpired                 *bool              `json:"auto_pause_on_expired"`
	ContributionFiveHourReservePercent *float64           `json:"contribution_5h_reserve_percent"`
	ContributionWeeklyReservePercent   *float64           `json:"contribution_weekly_reserve_percent"`
	ContributionProbeFailurePolicy     *string            `json:"contribution_probe_failure_policy"`
}

type setUserAccountSchedulablePayload struct {
	Schedulable *bool `json:"schedulable"`
}

type updateUserAccountLimitsPayload struct {
	ContributionFiveHourReservePercent *float64 `json:"contribution_5h_reserve_percent"`
	ContributionWeeklyReservePercent   *float64 `json:"contribution_weekly_reserve_percent"`
	ContributionProbeFailurePolicy     *string  `json:"contribution_probe_failure_policy"`
	WindowCostLimit                    *float64 `json:"window_cost_limit"`
	WindowCostStickyReserve            *float64 `json:"window_cost_sticky_reserve"`
	QuotaWeeklyLimit                   *float64 `json:"quota_weekly_limit"`
	QuotaWeeklyMinRemaining            *float64 `json:"quota_weekly_min_remaining"`
}

type updateUserAccountScopePayload struct {
	GroupIDs                           *[]int64           `json:"group_ids"`
	ProxyID                            *int64             `json:"proxy_id"`
	Concurrency                        *int               `json:"concurrency"`
	ExpiresAt                          *int64             `json:"expires_at"`
	AutoPauseOnExpired                 *bool              `json:"auto_pause_on_expired"`
	ModelMapping                       *map[string]string `json:"model_mapping"`
	CodexCLIOnly                       *bool              `json:"codex_cli_only"`
	ContributionFiveHourReservePercent *float64           `json:"contribution_5h_reserve_percent"`
	ContributionWeeklyReservePercent   *float64           `json:"contribution_weekly_reserve_percent"`
	ContributionProbeFailurePolicy     *string            `json:"contribution_probe_failure_policy"`
}

type testUserAccountPayload struct {
	ModelID string `json:"model_id"`
	Prompt  string `json:"prompt"`
	Mode    string `json:"mode"`
}

type applyUserOAuthCredentialsPayload struct {
	Type        string         `json:"type" binding:"required,oneof=oauth"`
	Credentials map[string]any `json:"credentials" binding:"required"`
	Extra       map[string]any `json:"extra"`
}

type userOpenAIRefreshTokenPayload struct {
	RefreshToken string `json:"refresh_token"`
	RT           string `json:"rt"`
	ClientID     string `json:"client_id"`
	ProxyID      *int64 `json:"proxy_id"`
}

type userCodexSessionImportPayload struct {
	Content                            string         `json:"content"`
	Contents                           []string       `json:"contents"`
	Name                               string         `json:"name"`
	GroupIDs                           []int64        `json:"group_ids"`
	ProxyID                            *int64         `json:"proxy_id"`
	Concurrency                        *int           `json:"concurrency"`
	ExpiresAt                          *int64         `json:"expires_at"`
	AutoPauseOnExpired                 *bool          `json:"auto_pause_on_expired"`
	CredentialExtras                   map[string]any `json:"credential_extras"`
	Extra                              map[string]any `json:"extra"`
	UpdateExisting                     *bool          `json:"update_existing"`
	Schedulable                        *bool          `json:"schedulable"`
	CodexCLIOnly                       *bool          `json:"codex_cli_only"`
	ContributionFiveHourReservePercent *float64       `json:"contribution_5h_reserve_percent"`
	ContributionWeeklyReservePercent   *float64       `json:"contribution_weekly_reserve_percent"`
	ContributionProbeFailurePolicy     *string        `json:"contribution_probe_failure_policy"`
}

type createUserScheduledTestPlanPayload struct {
	AccountID      int64  `json:"account_id" binding:"required"`
	ModelID        string `json:"model_id"`
	CronExpression string `json:"cron_expression" binding:"required"`
	Enabled        *bool  `json:"enabled"`
	MaxResults     int    `json:"max_results"`
	AutoRecover    *bool  `json:"auto_recover"`
}

type updateUserScheduledTestPlanPayload struct {
	ModelID        string `json:"model_id"`
	CronExpression string `json:"cron_expression"`
	Enabled        *bool  `json:"enabled"`
	MaxResults     int    `json:"max_results"`
	AutoRecover    *bool  `json:"auto_recover"`
}

func (h *AccountPoolHandler) ListProxies(c *gin.Context) {
	if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.proxyService == nil {
		response.InternalError(c, "proxy service is not configured")
		return
	}

	proxies, err := h.proxyService.ListActive(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.Proxy, 0, len(proxies))
	for i := range proxies {
		out = append(out, *dto.ProxyFromService(&proxies[i]))
	}
	response.Success(c, out)
}

func (h *AccountPoolHandler) TestProxy(c *gin.Context) {
	if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.proxyService == nil {
		response.InternalError(c, "proxy service is not configured")
		return
	}
	proxyID, ok := parseProxyIDParam(c)
	if !ok {
		return
	}
	result, err := h.proxyService.Test(c.Request.Context(), proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AccountPoolHandler) requireOwnedUserAccount(c *gin.Context, userID, accountID int64) (*service.Account, bool) {
	account, err := h.accountService.GetOwnedUserOAuthAccount(c.Request.Context(), userID, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	return account, true
}

func (h *AccountPoolHandler) invalidateUserAccountToken(ctx context.Context, accountID int64) {
	if h.tokenCacheInvalidator == nil {
		return
	}
	account, err := h.accountService.GetByID(ctx, accountID)
	if err != nil || account == nil || !account.IsOAuth() {
		return
	}
	if err := h.tokenCacheInvalidator.InvalidateToken(ctx, account); err != nil {
		slog.Warn("user_account_pool.invalidate_token_failed", "account_id", accountID, "err", err)
	}
}

func buildUserOpenAIExtra(raw map[string]any) map[string]any {
	extra := make(map[string]any, 2)
	for _, key := range []string{"email", "name", "privacy_mode"} {
		if value, ok := raw[key].(string); ok && strings.TrimSpace(value) != "" {
			extra[key] = strings.TrimSpace(value)
		}
	}
	return extra
}

type userOAuthAuthURLPayload struct {
	Platform    string `json:"platform"`
	RedirectURI string `json:"redirect_uri"`
	ProjectID   string `json:"project_id"`
	OAuthType   string `json:"oauth_type"`
	TierID      string `json:"tier_id"`
	ProxyID     *int64 `json:"proxy_id"`
}

type userOAuthExchangePayload struct {
	Platform  string `json:"platform"`
	SessionID string `json:"session_id" binding:"required"`
	Code      string `json:"code" binding:"required"`
	State     string `json:"state"`
	ProjectID string `json:"project_id"`
	OAuthType string `json:"oauth_type"`
	TierID    string `json:"tier_id"`
	ProxyID   *int64 `json:"proxy_id"`
}

func (h *AccountPoolHandler) ListPool(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    strings.TrimSpace(c.DefaultQuery("sort_by", "name")),
		SortOrder: strings.TrimSpace(c.DefaultQuery("sort_order", pagination.SortOrderAsc)),
	}
	filters := service.UserAccountPoolListFilters{
		Platform: strings.TrimSpace(c.Query("platform")),
		PlanType: strings.TrimSpace(c.Query("plan_type")),
		Search:   strings.TrimSpace(c.Query("search")),
	}

	items, result, err := h.accountService.ListUserAccountPool(c.Request.Context(), subject.UserID, params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.hydrateCurrentWindowCost(c.Request.Context(), items)
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *AccountPoolHandler) CreateOAuth(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var payload createUserOAuthAccountPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	groupIDs, ok := h.validateUserAccountGroupIDs(c, subject.UserID, payload.Platform, payload.GroupIDs)
	if !ok {
		return
	}

	req := service.CreateUserOAuthAccountRequest{
		Name:                               payload.Name,
		Platform:                           payload.Platform,
		Type:                               payload.Type,
		Credentials:                        payload.Credentials,
		ModelMapping:                       payload.ModelMapping,
		Extra:                              payload.Extra,
		ProxyID:                            payload.ProxyID,
		Concurrency:                        payload.Concurrency,
		Schedulable:                        payload.Schedulable,
		GroupIDs:                           groupIDs,
		ExpiresAt:                          payload.ExpiresAt,
		AutoPauseOnExpired:                 payload.AutoPauseOnExpired,
		ContributionFiveHourReservePercent: payload.ContributionFiveHourReservePercent,
		ContributionWeeklyReservePercent:   payload.ContributionWeeklyReservePercent,
		ContributionProbeFailurePolicy:     payload.ContributionProbeFailurePolicy,
	}

	item, err := h.accountService.CreateUserOAuthAccount(c.Request.Context(), subject.UserID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := []service.UserAccountPoolItem{*item}
	h.hydrateCurrentWindowCost(c.Request.Context(), items)
	response.Success(c, items[0])
}

func (h *AccountPoolHandler) validateUserAccountGroupIDs(c *gin.Context, userID int64, platform string, rawGroupIDs []int64) ([]int64, bool) {
	groupIDs := normalizeInt64IDs(rawGroupIDs)
	if len(groupIDs) == 0 {
		return nil, true
	}
	if h.apiKeyService == nil {
		response.InternalError(c, "group service is not configured")
		return nil, false
	}

	availableGroups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	allowedByID := make(map[int64]service.Group, len(availableGroups))
	for _, group := range availableGroups {
		allowedByID[group.ID] = group
	}

	normalizedPlatform := strings.TrimSpace(platform)
	for _, groupID := range groupIDs {
		group, ok := allowedByID[groupID]
		if !ok {
			response.ErrorFrom(c, service.ErrGroupNotAllowed)
			return nil, false
		}
		if group.Platform != normalizedPlatform {
			response.BadRequest(c, "group platform does not match account platform")
			return nil, false
		}
	}
	return groupIDs, true
}

func normalizeInt64IDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (h *AccountPoolHandler) GenerateOAuthAuthURL(c *gin.Context) {
	if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var payload userOAuthAuthURLPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	switch strings.TrimSpace(payload.Platform) {
	case service.PlatformOpenAI:
		result, err := h.openaiOAuthService.GenerateAuthURL(c.Request.Context(), payload.ProxyID, strings.TrimSpace(payload.RedirectURI), service.PlatformOpenAI)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, result)
	default:
		response.BadRequest(c, "unsupported OAuth account platform")
	}
}

func (h *AccountPoolHandler) ExchangeOAuthCode(c *gin.Context) {
	if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var payload userOAuthExchangePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	switch strings.TrimSpace(payload.Platform) {
	case service.PlatformOpenAI:
		if strings.TrimSpace(payload.State) == "" {
			response.BadRequest(c, "state is required")
			return
		}
		tokenInfo, err := h.openaiOAuthService.ExchangeCode(c.Request.Context(), &service.OpenAIExchangeCodeInput{
			SessionID: payload.SessionID,
			Code:      payload.Code,
			State:     payload.State,
			ProxyID:   payload.ProxyID,
		})
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, tokenInfo)
	default:
		response.BadRequest(c, "unsupported OAuth account platform")
	}
}

func (h *AccountPoolHandler) userProxyURL(c *gin.Context, proxyID *int64) (string, bool) {
	if proxyID == nil || *proxyID <= 0 {
		return "", true
	}
	if h.proxyService == nil {
		response.InternalError(c, "proxy service is not configured")
		return "", false
	}
	proxy, err := h.proxyService.GetByID(c.Request.Context(), *proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return "", false
	}
	if proxy == nil || !proxy.IsActive() {
		response.ErrorFrom(c, infraerrors.New(http.StatusBadRequest, "ACCOUNT_PROXY_NOT_AVAILABLE", "proxy is not available"))
		return "", false
	}
	return proxy.URL(), true
}

func (h *AccountPoolHandler) RefreshOpenAIToken(c *gin.Context) {
	if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.openaiOAuthService == nil {
		response.InternalError(c, "OpenAI OAuth service is not configured")
		return
	}

	var payload userOpenAIRefreshTokenPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	refreshToken := strings.TrimSpace(payload.RefreshToken)
	if refreshToken == "" {
		refreshToken = strings.TrimSpace(payload.RT)
	}
	if refreshToken == "" {
		response.BadRequest(c, "refresh_token is required")
		return
	}

	proxyURL, ok := h.userProxyURL(c, payload.ProxyID)
	if !ok {
		return
	}
	clientID := strings.TrimSpace(payload.ClientID)
	if clientID == "" {
		clientID, _ = openai.OAuthClientConfigByPlatform(service.PlatformOpenAI)
	}

	tokenInfo, err := h.openaiOAuthService.RefreshTokenWithClientID(c.Request.Context(), refreshToken, proxyURL, clientID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

func (h *AccountPoolHandler) ImportCodexSession(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var payload userCodexSessionImportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if payload.Concurrency != nil && *payload.Concurrency < 1 {
		response.BadRequest(c, "concurrency must be >= 1")
		return
	}

	groupIDs, ok := h.validateUserAccountGroupIDs(c, subject.UserID, service.PlatformOpenAI, payload.GroupIDs)
	if !ok {
		return
	}

	importReq := admin.CodexSessionImportRequest{
		Content:            payload.Content,
		Contents:           payload.Contents,
		Name:               payload.Name,
		GroupIDs:           groupIDs,
		ProxyID:            payload.ProxyID,
		Concurrency:        payload.Concurrency,
		ExpiresAt:          payload.ExpiresAt,
		AutoPauseOnExpired: payload.AutoPauseOnExpired,
		CredentialExtras:   payload.CredentialExtras,
		Extra:              payload.Extra,
		UpdateExisting:     payload.UpdateExisting,
	}
	entries, err := admin.ParseCodexSessionImportEntries(importReq)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(entries) == 0 {
		response.BadRequest(c, "请输入 accessToken 或 Codex session JSON")
		return
	}

	result, err := h.importUserCodexSessions(c.Request.Context(), subject.UserID, payload, importReq, entries, groupIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AccountPoolHandler) importUserCodexSessions(
	ctx context.Context,
	userID int64,
	payload userCodexSessionImportPayload,
	importReq admin.CodexSessionImportRequest,
	entries []admin.CodexImportEntry,
	groupIDs []int64,
) (admin.CodexSessionImportResult, error) {
	result := admin.CodexSessionImportResult{
		Total: len(entries),
		Items: make([]admin.CodexSessionImportItem, 0, len(entries)),
	}

	existingAccounts, err := h.accountService.ListOwnedUserOAuthAccounts(ctx, userID, service.PlatformOpenAI)
	if err != nil {
		return result, err
	}
	index := admin.BuildCodexAccountIndex(existingAccounts)

	updateExisting := true
	if payload.UpdateExisting != nil {
		updateExisting = *payload.UpdateExisting
	}
	credentialExtras := admin.SanitizeCodexImportCredentialExtras(payload.CredentialExtras)
	seenIdentity := map[string]int{}

	for _, entry := range entries {
		item, err := admin.NormalizeCodexImportEntry(entry)
		if err != nil {
			appendUserCodexImportFailure(&result, entry.Index, "", err.Error())
			continue
		}

		accountName := admin.BuildCodexCreateAccountName(payload.Name, item, entry.Index, len(entries))
		effectiveExpiresAt, credentialExpiresAt, autoPauseOnExpired, expiryWarnings, expiryErr := admin.ResolveCodexImportExpiry(importReq, item)
		if expiryErr != nil {
			appendUserCodexImportFailure(&result, entry.Index, accountName, expiryErr.Error())
			continue
		}
		item.WarningTexts = append(item.WarningTexts, expiryWarnings...)
		if credentialExpiresAt != nil {
			item.Credentials["expires_at"] = credentialExpiresAt.Format(time.RFC3339)
		}
		credentials := admin.MergeCodexImportMap(item.Credentials, credentialExtras)
		extra := admin.MergeCodexImportMap(payload.Extra, item.Extra)
		if payload.CodexCLIOnly != nil && *payload.CodexCLIOnly {
			if extra == nil {
				extra = map[string]any{}
			}
			extra["codex_cli_only"] = true
		}
		for _, warning := range item.WarningTexts {
			result.Warnings = append(result.Warnings, admin.CodexSessionImportMessage{
				Index:   entry.Index,
				Name:    accountName,
				Message: warning,
			})
		}

		if duplicateIndex, ok := admin.FirstSeenCodexIdentity(seenIdentity, item.IdentityKeys); ok {
			message := fmt.Sprintf("与第 %d 条导入项重复，已跳过", duplicateIndex)
			result.Skipped++
			result.Items = append(result.Items, admin.CodexSessionImportItem{
				Index:   entry.Index,
				Name:    accountName,
				Action:  "skipped",
				Message: message,
			})
			result.Warnings = append(result.Warnings, admin.CodexSessionImportMessage{
				Index:   entry.Index,
				Name:    accountName,
				Message: message,
			})
			continue
		}
		admin.MarkCodexIdentitySeen(seenIdentity, item.IdentityKeys, entry.Index)

		if existing := index.Find(item.IdentityKeys); existing != nil {
			if !updateExisting {
				message := "账号已存在，已跳过"
				result.Skipped++
				result.Items = append(result.Items, admin.CodexSessionImportItem{
					Index:     entry.Index,
					Name:      accountName,
					Action:    "skipped",
					AccountID: existing.ID,
					Message:   message,
				})
				result.Warnings = append(result.Warnings, admin.CodexSessionImportMessage{
					Index:   entry.Index,
					Name:    accountName,
					Message: message,
				})
				continue
			}

			mergedCredentials := admin.MergeCodexImportCredentials(existing.Credentials, credentials, item)
			if strings.TrimSpace(item.RefreshToken) == "" {
				mergedCredentials["refresh_token"] = ""
				mergedCredentials["client_id"] = ""
			}
			if strings.TrimSpace(item.IDToken) == "" {
				mergedCredentials["id_token"] = ""
			}
			mergedExtra := admin.MergeCodexImportMap(existing.Extra, extra)
			_, updateErr := h.accountService.ApplyUserOAuthCredentials(ctx, userID, existing.ID, service.ApplyUserOAuthCredentialsRequest{
				Type:        service.AccountTypeOAuth,
				Credentials: mergedCredentials,
				Extra:       mergedExtra,
			})
			if updateErr == nil {
				_, updateErr = h.accountService.UpdateUserAccountScope(ctx, userID, existing.ID, service.UpdateUserAccountScopeRequest{
					GroupIDs:                           &groupIDs,
					ProxyID:                            payload.ProxyID,
					Concurrency:                        payload.Concurrency,
					ExpiresAt:                          effectiveExpiresAt,
					AutoPauseOnExpired:                 autoPauseOnExpired,
					CodexCLIOnly:                       payload.CodexCLIOnly,
					ContributionFiveHourReservePercent: payload.ContributionFiveHourReservePercent,
					ContributionWeeklyReservePercent:   payload.ContributionWeeklyReservePercent,
					ContributionProbeFailurePolicy:     payload.ContributionProbeFailurePolicy,
				})
			}
			if updateErr == nil && payload.Schedulable != nil {
				_, updateErr = h.accountService.SetUserAccountSchedulable(ctx, userID, existing.ID, *payload.Schedulable)
			}
			if updateErr != nil {
				appendUserCodexImportFailure(&result, entry.Index, accountName, updateErr.Error())
				continue
			}
			if h.tokenCacheInvalidator != nil {
				if account, getErr := h.accountService.GetOwnedUserOAuthAccount(ctx, userID, existing.ID); getErr == nil && account != nil {
					_ = h.tokenCacheInvalidator.InvalidateToken(ctx, account)
					index.Add(*account)
				}
			}
			result.Updated++
			accountID := existing.ID
			result.Items = append(result.Items, admin.CodexSessionImportItem{
				Index:     entry.Index,
				Name:      accountName,
				Action:    "updated",
				AccountID: accountID,
			})
			continue
		}

		created, createErr := h.accountService.CreateUserOAuthAccount(ctx, userID, service.CreateUserOAuthAccountRequest{
			Name:                               accountName,
			Platform:                           service.PlatformOpenAI,
			Type:                               service.AccountTypeOAuth,
			Credentials:                        credentials,
			Extra:                              extra,
			ProxyID:                            payload.ProxyID,
			Concurrency:                        payload.Concurrency,
			Schedulable:                        payload.Schedulable,
			GroupIDs:                           groupIDs,
			ExpiresAt:                          effectiveExpiresAt,
			AutoPauseOnExpired:                 autoPauseOnExpired,
			ContributionFiveHourReservePercent: payload.ContributionFiveHourReservePercent,
			ContributionWeeklyReservePercent:   payload.ContributionWeeklyReservePercent,
			ContributionProbeFailurePolicy:     payload.ContributionProbeFailurePolicy,
		})
		if createErr != nil {
			appendUserCodexImportFailure(&result, entry.Index, accountName, createErr.Error())
			continue
		}
		if createdAccount, getErr := h.accountService.GetOwnedUserOAuthAccount(ctx, userID, created.ID); getErr == nil && createdAccount != nil {
			index.Add(*createdAccount)
		}
		result.Created++
		result.Items = append(result.Items, admin.CodexSessionImportItem{
			Index:     entry.Index,
			Name:      accountName,
			Action:    "created",
			AccountID: created.ID,
		})
	}

	return result, nil
}

func appendUserCodexImportFailure(result *admin.CodexSessionImportResult, index int, name, message string) {
	result.Failed++
	result.Items = append(result.Items, admin.CodexSessionImportItem{
		Index:   index,
		Name:    name,
		Action:  "failed",
		Message: message,
	})
	result.Errors = append(result.Errors, admin.CodexSessionImportMessage{
		Index:   index,
		Name:    name,
		Message: message,
	})
}

func (h *AccountPoolHandler) SetSchedulable(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}

	var payload setUserAccountSchedulablePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if payload.Schedulable == nil {
		response.BadRequest(c, "schedulable is required")
		return
	}

	item, err := h.accountService.SetUserAccountSchedulable(c.Request.Context(), subject.UserID, accountID, *payload.Schedulable)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := []service.UserAccountPoolItem{*item}
	h.hydrateCurrentWindowCost(c.Request.Context(), items)
	response.Success(c, items[0])
}

func (h *AccountPoolHandler) UpdateScope(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}

	var payload updateUserAccountScopePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	account, err := h.accountService.GetByID(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if account.OwnerUserID == nil || *account.OwnerUserID != subject.UserID {
		response.ErrorFrom(c, service.ErrAccountOwnerRequired)
		return
	}

	var groupIDs *[]int64
	if payload.GroupIDs != nil {
		normalized, ok := h.validateUserAccountGroupIDs(c, subject.UserID, account.Platform, *payload.GroupIDs)
		if !ok {
			return
		}
		groupIDs = &normalized
	}

	item, err := h.accountService.UpdateUserAccountScope(c.Request.Context(), subject.UserID, accountID, service.UpdateUserAccountScopeRequest{
		GroupIDs:                           groupIDs,
		ProxyID:                            payload.ProxyID,
		Concurrency:                        payload.Concurrency,
		ExpiresAt:                          payload.ExpiresAt,
		AutoPauseOnExpired:                 payload.AutoPauseOnExpired,
		ModelMapping:                       payload.ModelMapping,
		CodexCLIOnly:                       payload.CodexCLIOnly,
		ContributionFiveHourReservePercent: payload.ContributionFiveHourReservePercent,
		ContributionWeeklyReservePercent:   payload.ContributionWeeklyReservePercent,
		ContributionProbeFailurePolicy:     payload.ContributionProbeFailurePolicy,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := []service.UserAccountPoolItem{*item}
	h.hydrateCurrentWindowCost(c.Request.Context(), items)
	response.Success(c, items[0])
}

func (h *AccountPoolHandler) Delete(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}

	if err := h.accountService.DeleteUserAccount(c.Request.Context(), subject.UserID, accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "account deleted"})
}

func (h *AccountPoolHandler) GetStats(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.accountUsageService == nil {
		response.InternalError(c, "account usage service is not configured")
		return
	}

	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}

	account, err := h.accountService.GetByID(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if account.OwnerUserID == nil || *account.OwnerUserID != subject.UserID {
		response.ErrorFrom(c, service.ErrAccountOwnerRequired)
		return
	}

	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays > 0 && parsedDays <= 90 {
			days = parsedDays
		}
	}

	now := timezone.Now()
	endTime := timezone.StartOfDay(now.AddDate(0, 0, 1))
	startTime := timezone.StartOfDay(now.AddDate(0, 0, -days+1))

	stats, err := h.accountUsageService.GetAccountUsageStats(c.Request.Context(), accountID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

func (h *AccountPoolHandler) GetAvailableModels(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}

	account, ok := h.requireOwnedUserAccount(c, subject.UserID, accountID)
	if !ok {
		return
	}

	if account.IsOpenAI() {
		if account.IsOpenAIPassthroughEnabled() {
			response.Success(c, openai.DefaultModels)
			return
		}
		mapping := account.GetModelMapping()
		if len(mapping) == 0 {
			response.Success(c, openai.DefaultModels)
			return
		}
		models := make([]openai.Model, 0, len(mapping))
		for requestedModel := range mapping {
			var found bool
			for _, model := range openai.DefaultModels {
				if model.ID == requestedModel {
					models = append(models, model)
					found = true
					break
				}
			}
			if !found {
				models = append(models, openai.Model{
					ID:          requestedModel,
					Object:      "model",
					Type:        "model",
					DisplayName: requestedModel,
				})
			}
		}
		response.Success(c, models)
		return
	}

	if account.IsGemini() {
		response.Success(c, geminicli.DefaultModels)
		return
	}

	if account.Platform == service.PlatformAntigravity {
		response.Success(c, antigravity.DefaultModels())
		return
	}

	if account.IsOAuth() {
		response.Success(c, claude.DefaultModels)
		return
	}

	response.Success(c, claude.DefaultModels)
}

func (h *AccountPoolHandler) Test(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.accountTestService == nil {
		response.InternalError(c, "account test service is not configured")
		return
	}

	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	if _, ok := h.requireOwnedUserAccount(c, subject.UserID, accountID); !ok {
		return
	}

	var payload testUserAccountPayload
	_ = c.ShouldBindJSON(&payload)

	if err := h.accountTestService.TestAccountConnection(c, accountID, payload.ModelID, payload.Prompt, payload.Mode); err != nil {
		return
	}

	if h.rateLimitService != nil {
		if _, err := h.rateLimitService.RecoverAccountAfterSuccessfulTest(c.Request.Context(), accountID); err != nil {
			_ = c.Error(err)
		}
	}
}

func (h *AccountPoolHandler) Refresh(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	account, ok := h.requireOwnedUserAccount(c, subject.UserID, accountID)
	if !ok {
		return
	}
	if !account.IsOpenAI() {
		response.ErrorFrom(c, infraerrors.BadRequest("ACCOUNT_PLATFORM_NOT_ALLOWED", "only OpenAI OAuth accounts are supported"))
		return
	}
	if h.openaiOAuthService == nil {
		response.InternalError(c, "OpenAI OAuth service is not configured")
		return
	}

	tokenInfo, err := h.openaiOAuthService.RefreshAccountToken(c.Request.Context(), account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	newCredentials := h.openaiOAuthService.BuildAccountCredentials(tokenInfo)
	for key, value := range account.Credentials {
		if _, exists := newCredentials[key]; !exists {
			newCredentials[key] = value
		}
	}

	item, err := h.accountService.ApplyUserOAuthCredentials(c.Request.Context(), subject.UserID, accountID, service.ApplyUserOAuthCredentialsRequest{
		Type:        service.AccountTypeOAuth,
		Credentials: newCredentials,
		Extra: buildUserOpenAIExtra(map[string]any{
			"email":        tokenInfo.Email,
			"privacy_mode": tokenInfo.PrivacyMode,
		}),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.invalidateUserAccountToken(c.Request.Context(), accountID)
	items := []service.UserAccountPoolItem{*item}
	h.hydrateCurrentWindowCost(c.Request.Context(), items)
	response.Success(c, items[0])
}

func (h *AccountPoolHandler) ApplyOAuthCredentials(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	account, ok := h.requireOwnedUserAccount(c, subject.UserID, accountID)
	if !ok {
		return
	}
	if !account.IsOpenAI() {
		response.ErrorFrom(c, infraerrors.BadRequest("ACCOUNT_PLATFORM_NOT_ALLOWED", "only OpenAI OAuth accounts are supported"))
		return
	}

	var payload applyUserOAuthCredentialsPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	item, err := h.accountService.ApplyUserOAuthCredentials(c.Request.Context(), subject.UserID, accountID, service.ApplyUserOAuthCredentialsRequest{
		Type:        payload.Type,
		Credentials: payload.Credentials,
		Extra:       payload.Extra,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.invalidateUserAccountToken(c.Request.Context(), accountID)
	items := []service.UserAccountPoolItem{*item}
	h.hydrateCurrentWindowCost(c.Request.Context(), items)
	response.Success(c, items[0])
}

func (h *AccountPoolHandler) RecoverState(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.rateLimitService == nil {
		response.InternalError(c, "rate limit service is not configured")
		return
	}

	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	if _, ok := h.requireOwnedUserAccount(c, subject.UserID, accountID); !ok {
		return
	}

	if _, err := h.rateLimitService.RecoverAccountState(c.Request.Context(), accountID, service.AccountRecoveryOptions{
		InvalidateToken: true,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	refreshed, err := h.accountService.GetOwnedUserOAuthAccount(c.Request.Context(), subject.UserID, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item := service.UserAccountPoolItemFromAccount(refreshed, subject.UserID)
	items := []service.UserAccountPoolItem{item}
	h.hydrateCurrentWindowCost(c.Request.Context(), items)
	response.Success(c, items[0])
}

func (h *AccountPoolHandler) SetPrivacy(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	account, ok := h.requireOwnedUserAccount(c, subject.UserID, accountID)
	if !ok {
		return
	}

	var mode string
	switch account.Platform {
	case service.PlatformOpenAI:
		mode = h.accountService.ForceUserOpenAIPrivacy(c.Request.Context(), account, h.privacyClientFactory)
	case service.PlatformAntigravity:
		mode = h.accountService.ForceUserAntigravityPrivacy(c.Request.Context(), account)
	default:
		response.ErrorFrom(c, infraerrors.BadRequest("ACCOUNT_PLATFORM_NOT_ALLOWED", "only OpenAI and Antigravity OAuth accounts support privacy setting"))
		return
	}
	if mode == "" {
		response.ErrorFrom(c, infraerrors.BadRequest("ACCOUNT_PRIVACY_NOT_AVAILABLE", "cannot set privacy: missing access token"))
		return
	}

	refreshed, err := h.accountService.GetOwnedUserOAuthAccount(c.Request.Context(), subject.UserID, accountID)
	if err == nil && refreshed != nil {
		account = refreshed
	} else {
		if account.Extra == nil {
			account.Extra = map[string]any{}
		}
		account.Extra["privacy_mode"] = mode
	}
	item := service.UserAccountPoolItemFromAccount(account, subject.UserID)
	items := []service.UserAccountPoolItem{item}
	h.hydrateCurrentWindowCost(c.Request.Context(), items)
	response.Success(c, items[0])
}

func (h *AccountPoolHandler) ListScheduledTestPlans(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.scheduledTestService == nil {
		response.InternalError(c, "scheduled test service is not configured")
		return
	}
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	if _, ok := h.requireOwnedUserAccount(c, subject.UserID, accountID); !ok {
		return
	}

	plans, err := h.scheduledTestService.ListPlansByAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plans)
}

func (h *AccountPoolHandler) CreateScheduledTestPlan(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.scheduledTestService == nil {
		response.InternalError(c, "scheduled test service is not configured")
		return
	}

	var payload createUserScheduledTestPlanPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if _, ok := h.requireOwnedUserAccount(c, subject.UserID, payload.AccountID); !ok {
		return
	}

	plan := &service.ScheduledTestPlan{
		AccountID:      payload.AccountID,
		ModelID:        payload.ModelID,
		CronExpression: payload.CronExpression,
		Enabled:        true,
		MaxResults:     payload.MaxResults,
	}
	if payload.Enabled != nil {
		plan.Enabled = *payload.Enabled
	}
	if payload.AutoRecover != nil {
		plan.AutoRecover = *payload.AutoRecover
	}

	created, err := h.scheduledTestService.CreatePlan(c.Request.Context(), plan)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("SCHEDULED_TEST_PLAN_INVALID", err.Error()))
		return
	}
	response.Success(c, created)
}

func (h *AccountPoolHandler) UpdateScheduledTestPlan(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.scheduledTestService == nil {
		response.InternalError(c, "scheduled test service is not configured")
		return
	}
	planID, ok := parsePlanIDParam(c)
	if !ok {
		return
	}

	plan, err := h.scheduledTestService.GetPlan(c.Request.Context(), planID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if _, ok := h.requireOwnedUserAccount(c, subject.UserID, plan.AccountID); !ok {
		return
	}

	var payload updateUserScheduledTestPlanPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if payload.ModelID != "" {
		plan.ModelID = payload.ModelID
	}
	if payload.CronExpression != "" {
		plan.CronExpression = payload.CronExpression
	}
	if payload.Enabled != nil {
		plan.Enabled = *payload.Enabled
	}
	if payload.MaxResults > 0 {
		plan.MaxResults = payload.MaxResults
	}
	if payload.AutoRecover != nil {
		plan.AutoRecover = *payload.AutoRecover
	}

	updated, err := h.scheduledTestService.UpdatePlan(c.Request.Context(), plan)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("SCHEDULED_TEST_PLAN_INVALID", err.Error()))
		return
	}
	response.Success(c, updated)
}

func (h *AccountPoolHandler) DeleteScheduledTestPlan(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.scheduledTestService == nil {
		response.InternalError(c, "scheduled test service is not configured")
		return
	}
	planID, ok := parsePlanIDParam(c)
	if !ok {
		return
	}

	plan, err := h.scheduledTestService.GetPlan(c.Request.Context(), planID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if _, ok := h.requireOwnedUserAccount(c, subject.UserID, plan.AccountID); !ok {
		return
	}

	if err := h.scheduledTestService.DeletePlan(c.Request.Context(), planID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

func (h *AccountPoolHandler) ListScheduledTestResults(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.scheduledTestService == nil {
		response.InternalError(c, "scheduled test service is not configured")
		return
	}
	planID, ok := parsePlanIDParam(c)
	if !ok {
		return
	}

	plan, err := h.scheduledTestService.GetPlan(c.Request.Context(), planID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if _, ok := h.requireOwnedUserAccount(c, subject.UserID, plan.AccountID); !ok {
		return
	}

	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	results, err := h.scheduledTestService.ListResults(c.Request.Context(), planID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, results)
}

func (h *AccountPoolHandler) UpdateLimits(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}

	var payload updateUserAccountLimitsPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	item, err := h.accountService.UpdateUserAccountLimits(c.Request.Context(), subject.UserID, accountID, service.UpdateUserAccountLimitsRequest{
		ContributionFiveHourReservePercent: payload.ContributionFiveHourReservePercent,
		ContributionWeeklyReservePercent:   payload.ContributionWeeklyReservePercent,
		ContributionProbeFailurePolicy:     payload.ContributionProbeFailurePolicy,
		WindowCostLimit:                    payload.WindowCostLimit,
		WindowCostStickyReserve:            payload.WindowCostStickyReserve,
		QuotaWeeklyLimit:                   payload.QuotaWeeklyLimit,
		QuotaWeeklyMinRemaining:            payload.QuotaWeeklyMinRemaining,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := []service.UserAccountPoolItem{*item}
	h.hydrateCurrentWindowCost(c.Request.Context(), items)
	response.Success(c, items[0])
}

func parseAccountIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return 0, false
	}
	return id, true
}

func parseProxyIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid proxy ID")
		return 0, false
	}
	return id, true
}

func parsePlanIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid plan ID")
		return 0, false
	}
	return id, true
}

func (h *AccountPoolHandler) hydrateCurrentWindowCost(ctx context.Context, items []service.UserAccountPoolItem) {
	if h.accountUsageService == nil {
		return
	}
	for i := range items {
		if items[i].WindowCostStart == nil || items[i].WindowCostLimit <= 0 {
			continue
		}
		stats, err := h.accountUsageService.GetAccountWindowStats(ctx, items[i].ID, *items[i].WindowCostStart)
		if err != nil || stats == nil {
			continue
		}
		cost := stats.StandardCost
		items[i].CurrentWindowCost = &cost
	}
}

func normalizedGeminiOAuthType(value string) string {
	oauthType := strings.TrimSpace(value)
	if oauthType == "" {
		oauthType = "code_assist"
	}
	switch oauthType {
	case "code_assist", "google_one", "ai_studio":
		return oauthType
	default:
		return ""
	}
}

func deriveUserOAuthRedirectURI(c *gin.Context) string {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin != "" {
		return strings.TrimRight(origin, "/") + "/auth/callback"
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xfProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); xfProto != "" {
		scheme = strings.TrimSpace(strings.Split(xfProto, ",")[0])
	}

	host := strings.TrimSpace(c.Request.Host)
	if xfHost := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); xfHost != "" {
		host = strings.TrimSpace(strings.Split(xfHost, ",")[0])
	}

	return fmt.Sprintf("%s://%s/auth/callback", scheme, host)
}

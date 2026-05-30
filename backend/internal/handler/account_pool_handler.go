package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
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
}

func NewAccountPoolHandler(
	accountService *service.AccountService,
	apiKeyService *service.APIKeyService,
	accountUsageService *service.AccountUsageService,
	proxyService *service.ProxyService,
	openaiOAuthService *service.OpenAIOAuthService,
	geminiOAuthService *service.GeminiOAuthService,
	antigravityOAuthService *service.AntigravityOAuthService,
) *AccountPoolHandler {
	return &AccountPoolHandler{
		accountService:          accountService,
		apiKeyService:           apiKeyService,
		accountUsageService:     accountUsageService,
		proxyService:            proxyService,
		openaiOAuthService:      openaiOAuthService,
		geminiOAuthService:      geminiOAuthService,
		antigravityOAuthService: antigravityOAuthService,
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
	ExpiresAt                          *int64             `json:"expires_at"`
	AutoPauseOnExpired                 *bool              `json:"auto_pause_on_expired"`
	ModelMapping                       *map[string]string `json:"model_mapping"`
	CodexCLIOnly                       *bool              `json:"codex_cli_only"`
	ContributionFiveHourReservePercent *float64           `json:"contribution_5h_reserve_percent"`
	ContributionWeeklyReservePercent   *float64           `json:"contribution_weekly_reserve_percent"`
	ContributionProbeFailurePolicy     *string            `json:"contribution_probe_failure_policy"`
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

type userOAuthAuthURLPayload struct {
	Platform    string `json:"platform"`
	RedirectURI string `json:"redirect_uri"`
	ProjectID   string `json:"project_id"`
	OAuthType   string `json:"oauth_type"`
	TierID      string `json:"tier_id"`
}

type userOAuthExchangePayload struct {
	Platform  string `json:"platform"`
	SessionID string `json:"session_id" binding:"required"`
	Code      string `json:"code" binding:"required"`
	State     string `json:"state"`
	ProjectID string `json:"project_id"`
	OAuthType string `json:"oauth_type"`
	TierID    string `json:"tier_id"`
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
		result, err := h.openaiOAuthService.GenerateAuthURL(c.Request.Context(), nil, strings.TrimSpace(payload.RedirectURI), service.PlatformOpenAI)
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

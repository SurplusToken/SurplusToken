package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type AccountPoolHandler struct {
	accountService          *service.AccountService
	accountUsageService     *service.AccountUsageService
	openaiOAuthService      *service.OpenAIOAuthService
	geminiOAuthService      *service.GeminiOAuthService
	antigravityOAuthService *service.AntigravityOAuthService
}

func NewAccountPoolHandler(
	accountService *service.AccountService,
	accountUsageService *service.AccountUsageService,
	openaiOAuthService *service.OpenAIOAuthService,
	geminiOAuthService *service.GeminiOAuthService,
	antigravityOAuthService *service.AntigravityOAuthService,
) *AccountPoolHandler {
	return &AccountPoolHandler{
		accountService:          accountService,
		accountUsageService:     accountUsageService,
		openaiOAuthService:      openaiOAuthService,
		geminiOAuthService:      geminiOAuthService,
		antigravityOAuthService: antigravityOAuthService,
	}
}

type createUserOAuthAccountPayload struct {
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

	req := service.CreateUserOAuthAccountRequest{
		Name:                               payload.Name,
		Platform:                           payload.Platform,
		Type:                               payload.Type,
		Credentials:                        payload.Credentials,
		Extra:                              payload.Extra,
		Schedulable:                        payload.Schedulable,
		ContributionFiveHourReservePercent: payload.ContributionFiveHourReservePercent,
		ContributionWeeklyReservePercent:   payload.ContributionWeeklyReservePercent,
		ContributionProbeFailurePolicy:     payload.ContributionProbeFailurePolicy,
		WindowCostLimit:                    payload.WindowCostLimit,
		WindowCostStickyReserve:            payload.WindowCostStickyReserve,
		QuotaWeeklyLimit:                   payload.QuotaWeeklyLimit,
		QuotaWeeklyMinRemaining:            payload.QuotaWeeklyMinRemaining,
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
	case service.PlatformGemini:
		oauthType := normalizedGeminiOAuthType(payload.OAuthType)
		if oauthType == "" {
			response.BadRequest(c, "Invalid oauth_type: must be 'code_assist', 'google_one', or 'ai_studio'")
			return
		}
		result, err := h.geminiOAuthService.GenerateAuthURL(c.Request.Context(), nil, deriveUserOAuthRedirectURI(c), strings.TrimSpace(payload.ProjectID), oauthType, strings.TrimSpace(payload.TierID))
		if err != nil {
			response.BadRequest(c, "Failed to generate auth URL: "+err.Error())
			return
		}
		response.Success(c, result)
	case service.PlatformAntigravity:
		result, err := h.antigravityOAuthService.GenerateAuthURL(c.Request.Context(), nil)
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
	case service.PlatformGemini:
		if strings.TrimSpace(payload.State) == "" {
			response.BadRequest(c, "state is required")
			return
		}
		oauthType := normalizedGeminiOAuthType(payload.OAuthType)
		if oauthType == "" {
			response.BadRequest(c, "Invalid oauth_type: must be 'code_assist', 'google_one', or 'ai_studio'")
			return
		}
		tokenInfo, err := h.geminiOAuthService.ExchangeCode(c.Request.Context(), &service.GeminiExchangeCodeInput{
			SessionID: payload.SessionID,
			State:     payload.State,
			Code:      payload.Code,
			OAuthType: oauthType,
			TierID:    strings.TrimSpace(payload.TierID),
		})
		if err != nil {
			response.BadRequest(c, "Failed to exchange code: "+err.Error())
			return
		}
		response.Success(c, tokenInfo)
	case service.PlatformAntigravity:
		if strings.TrimSpace(payload.State) == "" {
			response.BadRequest(c, "state is required")
			return
		}
		tokenInfo, err := h.antigravityOAuthService.ExchangeCode(c.Request.Context(), &service.AntigravityExchangeCodeInput{
			SessionID: payload.SessionID,
			State:     payload.State,
			Code:      payload.Code,
		})
		if err != nil {
			response.BadRequest(c, "Token exchange failed: "+err.Error())
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

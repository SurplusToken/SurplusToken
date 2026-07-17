package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type KimiOAuthHandler struct {
	kimiOAuthService *service.KimiOAuthService
	adminService     service.AdminService
}

func NewKimiOAuthHandler(kimiOAuthService *service.KimiOAuthService, adminService service.AdminService) *KimiOAuthHandler {
	return &KimiOAuthHandler{kimiOAuthService: kimiOAuthService, adminService: adminService}
}

func (h *KimiOAuthHandler) StartDeviceAuthorization(c *gin.Context) {
	var req struct {
		ProxyID *int64 `json:"proxy_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.ProxyID = nil
	}
	result, err := h.kimiOAuthService.StartDeviceAuthorization(c.Request.Context(), req.ProxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *KimiOAuthHandler) PollDeviceToken(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.kimiOAuthService.PollDeviceToken(c.Request.Context(), req.SessionID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *KimiOAuthHandler) RefreshAccountToken(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !account.IsKimiOAuth() {
		response.BadRequest(c, "Account is not a Kimi OAuth account")
		return
	}
	info, err := h.kimiOAuthService.RefreshAccountToken(c.Request.Context(), account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	credentials := service.MergeCredentials(account.Credentials, h.kimiOAuthService.BuildAccountCredentials(info))
	updated, err := h.adminService.UpdateAccount(c.Request.Context(), accountID, &service.UpdateAccountInput{Credentials: credentials})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(updated))
}

func (h *KimiOAuthHandler) CreateAccount(c *gin.Context) {
	var req struct {
		Name               string                 `json:"name"`
		Notes              *string                `json:"notes"`
		Token              *service.KimiTokenInfo `json:"token" binding:"required"`
		ProxyID            *int64                 `json:"proxy_id"`
		Concurrency        int                    `json:"concurrency"`
		Priority           int                    `json:"priority"`
		RateMultiplier     *float64               `json:"rate_multiplier"`
		LoadFactor         *int                   `json:"load_factor"`
		GroupIDs           []int64                `json:"group_ids"`
		ExpiresAt          *int64                 `json:"expires_at"`
		AutoPauseOnExpired *bool                  `json:"auto_pause_on_expired"`
		CredentialExtras   map[string]any         `json:"credential_extras"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Token == nil || strings.TrimSpace(req.Token.AccessToken) == "" ||
		strings.TrimSpace(req.Token.RefreshToken) == "" || req.Token.ExpiresAt <= 0 {
		response.BadRequest(c, "A complete OAuth token is required")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Kimi Coding Plan"
	}
	credentials := h.kimiOAuthService.BuildAccountCredentials(req.Token)
	for _, key := range []string{"model_mapping", "temp_unschedulable_enabled", "temp_unschedulable_rules"} {
		if value, ok := req.CredentialExtras[key]; ok {
			credentials[key] = value
		}
	}
	extra := map[string]any{
		"openai_compatible_provider": "kimi",
		"openai_responses_mode":      "force_chat_completions",
	}
	account, err := h.adminService.CreateAccount(c.Request.Context(), &service.CreateAccountInput{
		Name: name, Notes: req.Notes, Platform: service.PlatformKimi, Type: service.AccountTypeOAuth,
		Credentials: credentials,
		Extra:       extra,
		ProxyID:     req.ProxyID, Concurrency: req.Concurrency, Priority: req.Priority,
		RateMultiplier: req.RateMultiplier, LoadFactor: req.LoadFactor, GroupIDs: req.GroupIDs,
		ExpiresAt: req.ExpiresAt, AutoPauseOnExpired: req.AutoPauseOnExpired,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(account))
}

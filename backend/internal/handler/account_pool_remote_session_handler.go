package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// remoteSessionDisconnectPayload is the body for the disconnect endpoint.
type remoteSessionDisconnectPayload struct {
	KasmID string `json:"kasm_id"`
}

// remoteSessionReady maps a service result to the {status:"ready"|"queued", ...} shape.
func remoteSessionReady(result *service.RemoteSessionResult) gin.H {
	if result == nil {
		return gin.H{"status": service.RemoteSessionStatusQueued}
	}
	if result.Status == service.RemoteSessionStatusQueued {
		return gin.H{"status": "queued", "position": result.Position}
	}
	return gin.H{
		"status":      "ready",
		"connect_url": result.ConnectURL,
		"kasm_id":     result.KasmID,
	}
}

func (h *AccountPoolHandler) requireRemoteSessionService(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	if h.remoteSessionService == nil || !h.remoteSessionService.Enabled() {
		response.ErrorFrom(c, service.ErrRemoteSessionNotConfigured)
		return 0, false
	}
	return subject.UserID, true
}

// RemoteSessionSetup handles POST /accounts/pool/:id/remote-session/setup (owner only).
// It starts a MODE=setup Kasm session and marks the account seed-ready.
func (h *AccountPoolHandler) RemoteSessionSetup(c *gin.Context) {
	userID, ok := h.requireRemoteSessionService(c)
	if !ok {
		return
	}
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	result, err := h.remoteSessionService.Setup(c.Request.Context(), userID, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"connect_url": result.ConnectURL, "kasm_id": result.KasmID})
}

// RemoteSessionConnect handles POST /accounts/pool/:id/remote-session
// (owner + co-owner, Pro, requires remote_seed_ready).
func (h *AccountPoolHandler) RemoteSessionConnect(c *gin.Context) {
	userID, ok := h.requireRemoteSessionService(c)
	if !ok {
		return
	}
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	result, err := h.remoteSessionService.Connect(c.Request.Context(), userID, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, remoteSessionReady(result))
}

// RemoteSessionStatus handles GET /accounts/pool/:id/remote-session/status
// (poll for queued→ready).
func (h *AccountPoolHandler) RemoteSessionStatus(c *gin.Context) {
	userID, ok := h.requireRemoteSessionService(c)
	if !ok {
		return
	}
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	result, err := h.remoteSessionService.Poll(c.Request.Context(), userID, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, remoteSessionReady(result))
}

// RemoteSessionKeepalive handles POST /accounts/pool/:id/remote-session/keepalive.
// The frontend calls this every ~30s while its Kasm tab is open so the reconciler has a
// reliable "user is still here" signal (Kasm's own connection_info is empty here).
func (h *AccountPoolHandler) RemoteSessionKeepalive(c *gin.Context) {
	userID, ok := h.requireRemoteSessionService(c)
	if !ok {
		return
	}
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	if err := h.remoteSessionService.Keepalive(c.Request.Context(), userID, accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

// RemoteSessionDisconnect handles POST /accounts/pool/:id/remote-session/disconnect.
func (h *AccountPoolHandler) RemoteSessionDisconnect(c *gin.Context) {
	userID, ok := h.requireRemoteSessionService(c)
	if !ok {
		return
	}
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	var payload remoteSessionDisconnectPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(payload.KasmID) == "" {
		response.BadRequest(c, "kasm_id is required")
		return
	}
	if err := h.remoteSessionService.Disconnect(c.Request.Context(), userID, accountID, payload.KasmID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "disconnected"})
}

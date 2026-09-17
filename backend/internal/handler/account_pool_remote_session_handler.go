package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// requireRemoteSessionService resolves the caller's user id and fails with a stable
// code when the hand-off target is not configured.
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

// RemoteSessionConnect handles POST /accounts/pool/:id/remote-session
// (owner or co-owner, Pro plan, account linked to a Desktop2Web slot).
//
// It provisions the caller's Access on the gateway and returns a single-use SSO
// ticket plus the endpoint to redeem it at. The ticket is returned in the response
// body rather than embedded in a URL, and the frontend form-POSTs it immediately, so
// it never reaches browser history, a Referer header or the gateway's access log.
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
	response.Success(c, gin.H{
		"ticket":     result.Ticket,
		"redeem_url": result.RedeemURL,
	})
}

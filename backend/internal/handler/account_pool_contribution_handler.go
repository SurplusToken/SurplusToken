package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// distributePoolPayload is the body for the distribute endpoint. mode=="even" splits the
// whole pool equally across the owner set; otherwise allocations are explicit per-recipient.
type distributePoolPayload struct {
	Mode        string `json:"mode"`
	Allocations []struct {
		UserID int64   `json:"user_id"`
		Amount float64 `json:"amount"`
	} `json:"allocations"`
}

func (h *AccountPoolHandler) currentUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

// GetContributionPool handles GET /accounts/pool/:id/contribution-pool (primary owner only):
// the held reward pool + the eligible distribution recipients (owner + co-owners).
func (h *AccountPoolHandler) GetContributionPool(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		return
	}
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	view, err := h.contributionPoolService.GetAccountPool(c.Request.Context(), userID, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

// DistributeContributionPool handles POST /accounts/pool/:id/contribution-pool/distribute
// (primary owner only): distributes the held pool to the owner set, evenly or with custom
// per-recipient amounts. Credited contribution balance is available immediately (no freeze).
func (h *AccountPoolHandler) DistributeContributionPool(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		return
	}
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	var payload distributePoolPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	allocations := make([]service.PoolAllocation, 0, len(payload.Allocations))
	for _, a := range payload.Allocations {
		allocations = append(allocations, service.PoolAllocation{UserID: a.UserID, Amount: a.Amount})
	}
	if err := h.contributionPoolService.DistributeAccountPool(c.Request.Context(), userID, accountID, payload.Mode, allocations); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// Return the refreshed pool view (now reduced) for the UI.
	if view, err := h.contributionPoolService.GetAccountPool(c.Request.Context(), userID, accountID); err == nil {
		response.Success(c, view)
		return
	}
	response.Success(c, gin.H{"message": "distributed"})
}

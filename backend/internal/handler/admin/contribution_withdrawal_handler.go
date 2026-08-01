package admin

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ContributionWithdrawalHandler struct {
	service *service.ContributionService
}

func (h *ContributionWithdrawalHandler) GetQRCode(c *gin.Context) {
	if h == nil || h.service == nil {
		response.InternalError(c, "contribution service is not configured")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	withdrawalID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || withdrawalID <= 0 {
		response.BadRequest(c, "Invalid withdrawal ID")
		return
	}
	data, contentType, err := h.service.GetWithdrawalQRCode(c.Request.Context(), subject.UserID, withdrawalID, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, contentType, data)
}

func NewContributionWithdrawalHandler(contributionService *service.ContributionService) *ContributionWithdrawalHandler {
	return &ContributionWithdrawalHandler{service: contributionService}
}

func (h *ContributionWithdrawalHandler) List(c *gin.Context) {
	if h == nil || h.service == nil {
		response.InternalError(c, "contribution service is not configured")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.service.ListWithdrawalsAdmin(c.Request.Context(), c.Query("status"), c.Query("search"), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

type reviewContributionWithdrawalPayload struct {
	Status           string `json:"status"`
	ReviewNote       string `json:"review_note"`
	PaymentReference string `json:"payment_reference"`
}

func (h *ContributionWithdrawalHandler) Review(c *gin.Context) {
	if h == nil || h.service == nil {
		response.InternalError(c, "contribution service is not configured")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	withdrawalID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || withdrawalID <= 0 {
		response.BadRequest(c, "Invalid withdrawal ID")
		return
	}
	var payload reviewContributionWithdrawalPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.ReviewWithdrawal(c.Request.Context(), service.ReviewContributionWithdrawalRequest{
		WithdrawalID:     withdrawalID,
		AdminUserID:      subject.UserID,
		Status:           payload.Status,
		ReviewNote:       payload.ReviewNote,
		PaymentReference: payload.PaymentReference,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if result != nil && result.Status == service.ContributionWithdrawalStatusPaid {
		baseCtx := context.WithoutCancel(c.Request.Context())
		go func(withdrawal *service.ContributionWithdrawal) {
			ctx, cancel := context.WithTimeout(baseCtx, 30*time.Second)
			defer cancel()
			h.service.NotifyWithdrawalPaid(ctx, withdrawal)
		}(result)
	}
	response.Success(c, result)
}

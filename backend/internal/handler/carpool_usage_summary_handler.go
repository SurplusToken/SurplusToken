package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// CarpoolUsageSummaryHandler 暴露拼车本周用量摘要（我的用量/保底/预计结算 + 全车合计）。
// 权限与可见性在 service 层收口（与 upstream-quota 同一道闸），这里只做参数解析与响应。
type CarpoolUsageSummaryHandler struct {
	service *service.CarpoolUsageSummaryService
}

func NewCarpoolUsageSummaryHandler(usageSummaryService *service.CarpoolUsageSummaryService) *CarpoolUsageSummaryHandler {
	return &CarpoolUsageSummaryHandler{service: usageSummaryService}
}

// Get handles GET /carpools/:id/usage-summary（读自家库，无缓存）。
func (h *CarpoolUsageSummaryHandler) Get(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	result, err := h.service.Get(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"usage_summary": result})
}

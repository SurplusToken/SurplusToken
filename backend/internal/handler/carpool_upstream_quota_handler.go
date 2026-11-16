package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// CarpoolUpstreamQuotaHandler 暴露拼车绑定的上游 cpr 账号周额度（服务端代理 + Redis 缓存）。
// 权限与可见性在 service 层收口（与 GetRoster 同一道闸），这里只做参数解析与响应。
type CarpoolUpstreamQuotaHandler struct {
	service *service.CarpoolUpstreamQuotaService
}

func NewCarpoolUpstreamQuotaHandler(upstreamQuotaService *service.CarpoolUpstreamQuotaService) *CarpoolUpstreamQuotaHandler {
	return &CarpoolUpstreamQuotaHandler{service: upstreamQuotaService}
}

// Get handles GET /carpools/:id/upstream-quota（缓存优先，5 分钟 TTL）。
func (h *CarpoolUpstreamQuotaHandler) Get(c *gin.Context) {
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
	response.Success(c, gin.H{"upstream_quota": result})
}

// Refresh handles POST /carpools/:id/upstream-quota/refresh（强制刷新，60 秒冷却）。
func (h *CarpoolUpstreamQuotaHandler) Refresh(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	result, err := h.service.Refresh(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"upstream_quota": result})
}

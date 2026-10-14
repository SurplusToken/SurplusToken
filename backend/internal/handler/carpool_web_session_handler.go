package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// CarpoolWebSessionHandler 暴露拼车「打开网页版 ChatGPT」会话票据签发。
// 权限与可见性在 service 层收口（与 upstream-quota 同一道闸），这里只做参数解析与响应。
type CarpoolWebSessionHandler struct {
	service *service.CarpoolWebSessionService
}

func NewCarpoolWebSessionHandler(webSessionService *service.CarpoolWebSessionService) *CarpoolWebSessionHandler {
	return &CarpoolWebSessionHandler{service: webSessionService}
}

// Create handles POST /carpools/:id/web-session（幂等供给 Access + 签发 ES256 ticket）。
func (h *CarpoolWebSessionHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	result, err := h.service.Create(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"web_session": result})
}

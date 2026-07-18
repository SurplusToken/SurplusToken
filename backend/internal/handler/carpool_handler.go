package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CarpoolHandler struct {
	service *service.CarpoolService
}

func NewCarpoolHandler(carpoolService *service.CarpoolService) *CarpoolHandler {
	return &CarpoolHandler{service: carpoolService}
}

type createCarpoolRequest struct {
	Name             string `json:"name" binding:"required"`
	Description      string `json:"description"`
	CarType          string `json:"car_type" binding:"required"`
	Level            int    `json:"level" binding:"required"`
	Visibility       string `json:"visibility" binding:"required"`
	ScheduledStartAt string `json:"scheduled_start_at" binding:"required"`
}

type carpoolInviteRequest struct {
	Token string `json:"token" binding:"required"`
}

type carpoolJoinLockRequest struct {
	Locked *bool `json:"locked" binding:"required"`
}

func (h *CarpoolHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	items, err := h.service.List(c.Request.Context(), subject.UserID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, items)
}

func (h *CarpoolHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var req createCarpoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid carpool request")
		return
	}
	start, err := time.Parse("2006-01-02", strings.TrimSpace(req.ScheduledStartAt))
	if err != nil {
		response.BadRequest(c, "scheduled_start_at must use YYYY-MM-DD")
		return
	}
	result, err := h.service.Create(c.Request.Context(), subject.UserID, service.CreateCarpoolInput{
		Name:             req.Name,
		Description:      req.Description,
		CarType:          req.CarType,
		Level:            req.Level,
		Visibility:       req.Visibility,
		ScheduledStartAt: &start,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, result)
}

func (h *CarpoolHandler) ResolveInvite(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	item, err := h.service.ResolveInvite(c.Request.Context(), subject.UserID, c.Param("token"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *CarpoolHandler) CreateInvite(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	token, err := h.service.CreateInvite(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"token": token})
}

func (h *CarpoolHandler) Join(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	result, err := h.service.Join(c.Request.Context(), id, subject.UserID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *CarpoolHandler) JoinByInvite(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var req carpoolInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid carpool invite")
		return
	}
	result, err := h.service.JoinByInvite(c.Request.Context(), req.Token, subject.UserID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *CarpoolHandler) Cancel(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	if response.ErrorFrom(c, h.service.Cancel(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c))) {
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *CarpoolHandler) SetJoinLocked(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	var req carpoolJoinLockRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Locked == nil {
		response.BadRequest(c, "locked is required")
		return
	}
	if response.ErrorFrom(c, h.service.SetJoinLocked(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c), *req.Locked)) {
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func parseCarpoolID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid carpool ID")
		return 0, false
	}
	return id, true
}

func isCarpoolAdmin(c *gin.Context) bool {
	role, _ := middleware2.GetUserRoleFromContext(c)
	return role == service.RoleAdmin
}

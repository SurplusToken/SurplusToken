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
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	// car_type/level 已废弃（保留兼容），额度池参数为空时使用默认值（设计文档 §3）。
	CarType          string  `json:"car_type"`
	Level            int     `json:"level"`
	Visibility       string  `json:"visibility" binding:"required"`
	ScheduledStartAt string  `json:"scheduled_start_at" binding:"required"`
	WeeklyLimitUSD   float64 `json:"weekly_limit_usd"`
	SeatFeeCNY       float64 `json:"seat_fee_cny"`
	UsagePoolCNY     float64 `json:"usage_pool_cny"`
	ReserveRatio     float64 `json:"reserve_ratio"`
	LaunchMinRatio   float64 `json:"launch_min_ratio"`
	LaunchMaxRatio   float64 `json:"launch_max_ratio"`
	// DeclaredWeeklyQuotaUSD 是 owner 本人的申报（可选，0 = owner 仅发起不占额度）。
	DeclaredWeeklyQuotaUSD float64 `json:"declared_weekly_quota_usd"`
}

type carpoolInviteRequest struct {
	Token                  string  `json:"token" binding:"required"`
	DeclaredWeeklyQuotaUSD float64 `json:"declared_weekly_quota_usd" binding:"required"`
}

type carpoolJoinRequest struct {
	DeclaredWeeklyQuotaUSD float64 `json:"declared_weekly_quota_usd" binding:"required"`
}

type carpoolLaunchRequest struct {
	Force bool `json:"force"`
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
		Name:                   req.Name,
		Description:            req.Description,
		CarType:                req.CarType,
		Level:                  req.Level,
		Visibility:             req.Visibility,
		ScheduledStartAt:       &start,
		WeeklyLimitUSD:         req.WeeklyLimitUSD,
		SeatFeeCNY:             req.SeatFeeCNY,
		UsagePoolCNY:           req.UsagePoolCNY,
		ReserveRatio:           req.ReserveRatio,
		LaunchMinRatio:         req.LaunchMinRatio,
		LaunchMaxRatio:         req.LaunchMaxRatio,
		DeclaredWeeklyQuotaUSD: req.DeclaredWeeklyQuotaUSD,
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
	var req carpoolJoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "declared_weekly_quota_usd is required")
		return
	}
	result, err := h.service.Join(c.Request.Context(), id, subject.UserID, req.DeclaredWeeklyQuotaUSD)
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
		response.BadRequest(c, "token and declared_weekly_quota_usd are required")
		return
	}
	result, err := h.service.JoinByInvite(c.Request.Context(), req.Token, subject.UserID, req.DeclaredWeeklyQuotaUSD)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// Launch 手动发车（设计文档 §4.3）：Σ申报进入 [95%, 105%]×周限额 后由
// owner/admin 触发；force=true 时按降档发车放宽到 80% 下限。
func (h *CarpoolHandler) Launch(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	var req carpoolLaunchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body（等价于 force=false）。
		req = carpoolLaunchRequest{}
	}
	result, err := h.service.Launch(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c), req.Force)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// DeclarationRecommendation 申报推荐（设计文档 §4.1）：基于本人最近 7 天用量。
func (h *CarpoolHandler) DeclarationRecommendation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	rec, err := h.service.GetDeclarationRecommendation(c.Request.Context(), subject.UserID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, rec)
}

// Settlement 月度结算单（设计文档 §4.5）：成员仅见自己，owner/admin 见全车。
func (h *CarpoolHandler) Settlement(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	settlement, err := h.service.GetSettlement(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, settlement)
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

package handler

import (
	"net/http"
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
	// 两项强制确认：已添加管理员微信（true）+ 群二维码（base64/data URL，≤2MB）。
	AddedAdminWechat bool   `json:"added_admin_wechat"`
	GroupQRCode      string `json:"group_qr_code"`
}

type carpoolInviteRequest struct {
	Token                  string  `json:"token" binding:"required"`
	DeclaredWeeklyQuotaUSD float64 `json:"declared_weekly_quota_usd" binding:"required"`
	// JoinedWechatGroup 上车入群确认：必须 true，否则 400 CARPOOL_GROUP_JOIN_REQUIRED。
	JoinedWechatGroup bool `json:"joined_wechat_group"`
}

type carpoolJoinRequest struct {
	DeclaredWeeklyQuotaUSD float64 `json:"declared_weekly_quota_usd" binding:"required"`
	// JoinedWechatGroup 上车入群确认：必须 true，否则 400 CARPOOL_GROUP_JOIN_REQUIRED。
	JoinedWechatGroup bool `json:"joined_wechat_group"`
}

type carpoolLaunchRequest struct {
	Force bool `json:"force"`
}

type carpoolJoinLockRequest struct {
	Locked *bool `json:"locked" binding:"required"`
}

type carpoolCustomRuleInterestRequest struct {
	// Note 是可选的用户备注，随通知邮件一并发给 admin。
	Note string `json:"note"`
}

// CustomRuleInterest 自定义规则咨询入口：登录用户点击后给全部 admin 发提示邮件；
// SMTP 未配置或发送失败优雅降级，接口照常返回成功。
func (h *CarpoolHandler) CustomRuleInterest(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var req carpoolCustomRuleInterestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body（note 可选）。
		req = carpoolCustomRuleInterestRequest{}
	}
	h.service.NotifyCustomRuleInterest(c.Request.Context(), subject.UserID, req.Note)
	response.Success(c, gin.H{"message": "ok"})
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
		AddedAdminWechat:       req.AddedAdminWechat,
		GroupQRCode:            req.GroupQRCode,
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
	result, err := h.service.Join(c.Request.Context(), id, subject.UserID, req.DeclaredWeeklyQuotaUSD, req.JoinedWechatGroup)
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
	result, err := h.service.JoinByInvite(c.Request.Context(), req.Token, subject.UserID, req.DeclaredWeeklyQuotaUSD, req.JoinedWechatGroup)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// Leave 下车：仅 recruiting 状态的普通成员；申报额度即时释放，幂等。
func (h *CarpoolHandler) Leave(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	result, err := h.service.Leave(c.Request.Context(), id, subject.UserID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// Confirm 车主确认发车（两段确认第一段）：仅 owner、recruiting、Σ申报在发车区间内。
func (h *CarpoolHandler) Confirm(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	result, err := h.service.Confirm(c.Request.Context(), id, subject.UserID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// GroupQRCode 返回车辆微信群二维码图片（任何登录用户可读，含未上车者）。
func (h *CarpoolHandler) GroupQRCode(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	data, contentType, err := h.service.GetGroupQRCode(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, contentType, data)
}

// Launch 管理员启动发车（两段确认第二段）：仅 admin；正常发车要求车已 confirmed，
// force=true 为降档发车（要求 recruiting 且 Σ申报≥80%×周限额，跳过确认流程）。
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

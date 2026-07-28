package handler

import (
	"context"
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
//
// 每用户冷却在派发前同步检查（超频返回 429），邮件本身异步发送——这个入口会给
// 全部 admin 逐一发信，同步发会把 SMTP 往返挂在请求上，也把限流之外的放大面留给调用方。
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
	if response.ErrorFrom(c, h.service.ReserveInterestSlot(subject.UserID)) {
		return
	}
	// 脱离请求生命周期但保留 trace 等请求值：请求返回后邮件仍需发完。
	ctx := context.WithoutCancel(c.Request.Context())
	userID, note := subject.UserID, req.Note
	go h.service.NotifyCustomRuleInterest(ctx, userID, note)
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
	result, err := h.service.Create(c.Request.Context(), subject.UserID, isCarpoolAdmin(c), service.CreateCarpoolInput{
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

// GroupQRCode 返回车辆微信群二维码图片。可见性由 service 层判定：
// admin/车主/成员始终可读；招募中的 public 车对登录用户开放；招募中的
// invite_only 车必须带 ?token=<邀请码>。
func (h *CarpoolHandler) GroupQRCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	data, contentType, err := h.service.GetGroupQRCode(c.Request.Context(), id,
		subject.UserID, isCarpoolAdmin(c), c.Query("token"))
	if response.ErrorFrom(c, err) {
		return
	}
	// private：二维码是按调用者身份授权的，不能进共享缓存（CDN/代理）。
	// no-cache 而不是 max-age：换码后浏览器若继续吃 5 分钟旧缓存，
	// 「更换成功」就是个谎言——每次用前必须回源确认。
	c.Header("Cache-Control", "private, no-cache")
	c.Data(http.StatusOK, contentType, data)
}

// ReplaceGroupQRCode 车主或 admin 更换群二维码：二维码会过期、群也会换，
// 没有它车主只能解散重建整辆车。
func (h *CarpoolHandler) ReplaceGroupQRCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	var req struct {
		GroupQRCode string `json:"group_qr_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if err := h.service.ReplaceGroupQRCode(c.Request.Context(), id, subject.UserID,
		isCarpoolAdmin(c), req.GroupQRCode); response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"replaced": true})
}

// Roster 返回车上现有成员与各自申报额度，供上车弹窗展示：
// 让人在填申报额度之前就知道席位费要跟几个人分、别人分别报了多少。
func (h *CarpoolHandler) Roster(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	members, err := h.service.GetRoster(c.Request.Context(), id,
		subject.UserID, isCarpoolAdmin(c), c.Query("token"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, members)
}

// Unconfirm 撤回确认（confirmed → recruiting）：车主或 admin。
// 给"等 admin 启动"这段状态一个出口，避免车连人带钱无限挂起。
func (h *CarpoolHandler) Unconfirm(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	result, err := h.service.Unconfirm(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// PendingLaunch 列出全部等待管理员启动的车（仅 admin）。
func (h *CarpoolHandler) PendingLaunch(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	items, err := h.service.ListPendingLaunch(c.Request.Context(), isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, items)
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

// Settle 冻结结算单（车主或 admin）：把当下的金额写死，之后读到的就是这份快照。
func (h *CarpoolHandler) Settle(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	settlement, err := h.service.SettleCarpool(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, settlement)
}

// Unsettle 撤销结算（仅 admin）：结算算错了得有个受控的改回路径。
func (h *CarpoolHandler) Unsettle(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	if response.ErrorFrom(c, h.service.UnsettleCarpool(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c))) {
		return
	}
	response.Success(c, gin.H{"message": "ok"})
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

// ---------------------------------------------------------------------------
// 管理端
// ---------------------------------------------------------------------------

// AdminOverview 返回全部拼车（含私密、已取消、已结束），供管理总览页。
func (h *CarpoolHandler) AdminOverview(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	items, err := h.service.ListForAdmin(c.Request.Context(), subject.UserID, isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, items)
}

// RemoveMember 管理员在发车前把某位成员移出车。
func (h *CarpoolHandler) RemoveMember(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	memberID, ok := parseCarpoolMemberUserID(c)
	if !ok {
		return
	}
	result, err := h.service.RemoveMember(c.Request.Context(), id, memberID,
		subject.UserID, isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// UpdateMemberQuota 管理员代改成员申报额度。
func (h *CarpoolHandler) UpdateMemberQuota(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	memberID, ok := parseCarpoolMemberUserID(c)
	if !ok {
		return
	}
	var req struct {
		DeclaredWeeklyQuotaUSD float64 `json:"declared_weekly_quota_usd" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	result, err := h.service.UpdateMemberQuota(c.Request.Context(), id, memberID,
		subject.UserID, isCarpoolAdmin(c), req.DeclaredWeeklyQuotaUSD)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// UpdateCarpool 管理员改车的基本信息（未传的字段保持不变）。
func (h *CarpoolHandler) UpdateCarpool(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	var req struct {
		Name             *string `json:"name"`
		Description      *string `json:"description"`
		Visibility       *string `json:"visibility"`
		ScheduledStartAt *string `json:"scheduled_start_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	// 前端的日期控件给的是 YYYY-MM-DD（与 Create 同一口径）；直接绑 *time.Time
	// 只认 RFC3339，管理员每次保存编辑都会被它打成 400。
	var scheduledStartAt *time.Time
	if req.ScheduledStartAt != nil && strings.TrimSpace(*req.ScheduledStartAt) != "" {
		start, err := time.Parse("2006-01-02", strings.TrimSpace(*req.ScheduledStartAt))
		if err != nil {
			response.BadRequest(c, "scheduled_start_at must use YYYY-MM-DD")
			return
		}
		scheduledStartAt = &start
	}
	result, err := h.service.UpdateCarpool(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c),
		service.UpdateCarpoolInput{
			Name:             req.Name,
			Description:      req.Description,
			Visibility:       req.Visibility,
			ScheduledStartAt: scheduledStartAt,
		})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// TransferOwner 把车主转给车上另一位在册成员。
func (h *CarpoolHandler) TransferOwner(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	var req struct {
		NewOwnerUserID int64 `json:"new_owner_user_id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	result, err := h.service.TransferOwner(c.Request.Context(), id, req.NewOwnerUserID,
		subject.UserID, isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// parseCarpoolMemberUserID 解析路径里的成员用户 ID。
func parseCarpoolMemberUserID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid member user id")
		return 0, false
	}
	return id, true
}

func isCarpoolAdmin(c *gin.Context) bool {
	role, _ := middleware2.GetUserRoleFromContext(c)
	return role == service.RoleAdmin
}

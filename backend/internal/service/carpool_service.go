package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	CarpoolTypeSmall = "small"
	CarpoolTypeLarge = "large"

	CarpoolVisibilityPublic     = "public"
	CarpoolVisibilityInviteOnly = "invite_only"
)

var (
	ErrCarpoolNotFound       = infraerrors.NotFound("CARPOOL_NOT_FOUND", "carpool not found")
	ErrCarpoolUnavailable    = infraerrors.Conflict("CARPOOL_UNAVAILABLE", "carpool is not accepting members")
	ErrCarpoolAlreadyJoined  = infraerrors.Conflict("CARPOOL_ALREADY_JOINED", "user already joined this carpool")
	ErrCarpoolForbidden      = infraerrors.Forbidden("CARPOOL_FORBIDDEN", "operation is not allowed for this carpool")
	ErrCarpoolInviteInvalid  = infraerrors.NotFound("CARPOOL_INVITE_INVALID", "carpool invite is invalid or expired")
	ErrCarpoolNameConflict   = infraerrors.Conflict("CARPOOL_NAME_CONFLICT", "an active group already uses this carpool name")
	ErrCarpoolInvalidRequest = infraerrors.BadRequest("CARPOOL_INVALID_REQUEST", "invalid carpool request")
	ErrCarpoolQuotaExceeded  = infraerrors.Conflict("CARPOOL_QUOTA_EXCEEDED", "declared quota exceeds the carpool's remaining joinable quota")
	ErrCarpoolLaunchNotReady = infraerrors.Conflict("CARPOOL_LAUNCH_NOT_READY", "total declared quota is outside the launchable band")

	// 创建车辆两项强制确认（添加管理员微信 + 上传群二维码）。
	ErrCarpoolContactConfirmRequired = infraerrors.BadRequest("CARPOOL_CONTACT_CONFIRM_REQUIRED", "confirm that you have added the admin on WeChat before creating a carpool")
	ErrCarpoolGroupQRCodeRequired    = infraerrors.BadRequest("CARPOOL_GROUP_QR_CODE_REQUIRED", "group_qr_code is required")
	ErrCarpoolGroupQRCodeInvalid     = infraerrors.BadRequest("CARPOOL_GROUP_QR_CODE_INVALID", "group_qr_code must be a base64 png/jpeg/webp image within 2MB")
	ErrCarpoolQRCodeNotFound         = infraerrors.NotFound("CARPOOL_QR_CODE_NOT_FOUND", "carpool has no group qr code")
	// 上车入群确认。
	ErrCarpoolGroupJoinRequired = infraerrors.BadRequest("CARPOOL_GROUP_JOIN_REQUIRED", "confirm that you have joined the WeChat group before boarding")
	// 下车/两段确认发车。
	ErrCarpoolNotMember        = infraerrors.NotFound("CARPOOL_NOT_MEMBER", "user is not a member of this carpool")
	ErrCarpoolOwnerCannotLeave = infraerrors.Conflict("CARPOOL_OWNER_CANNOT_LEAVE", "owner cannot leave; cancel the carpool instead")
	ErrCarpoolNotConfirmed     = infraerrors.Conflict("CARPOOL_NOT_CONFIRMED", "carpool must be confirmed by its owner before launch")
)

const (
	// CarpoolAdminWechatID 是硬编码的管理员微信号，创建车辆前必须先添加。
	CarpoolAdminWechatID = "Charlemartingale"
	// CarpoolGroupQRCodeMaxBytes 是群二维码解码后的字节上限（2MB）。
	CarpoolGroupQRCodeMaxBytes = 2 << 20
)

type Carpool struct {
	ID                     int64      `json:"id"`
	Name                   string     `json:"name"`
	Description            string     `json:"description"`
	Organizer              string     `json:"organizer"`
	OwnerUserID            *int64     `json:"owner_user_id,omitempty"`
	Platform               string     `json:"platform"`
	PlanType               string     `json:"plan_type"`
	CarType                string     `json:"car_type"`
	Level                  int        `json:"level"`
	Capacity               int        `json:"capacity"`
	MemberCount            int        `json:"member_count"`
	BaseFeeCNY             float64    `json:"base_fee_cny"`
	UsagePoolPerAccountCNY float64    `json:"usage_pool_cny_per_account"`
	Visibility             string     `json:"visibility"`
	Status                 string     `json:"status"`
	JoinLocked             bool       `json:"join_locked"`
	ScheduledStartAt       *time.Time `json:"scheduled_start_at,omitempty"`
	LaunchedAt             *time.Time `json:"launched_at,omitempty"`
	GroupID                *int64     `json:"group_id,omitempty"`
	GroupName              *string    `json:"group_name,omitempty"`
	MemberRole             *string    `json:"member_role"`
	CreatedAt              time.Time  `json:"created_at"`

	// 两段确认发车（recruiting → confirmed → active）与创建确认展示字段。
	LaunchNotifiedAt *time.Time `json:"launch_notified_at,omitempty"`
	ConfirmedAt      *time.Time `json:"confirmed_at,omitempty"`
	HasGroupQRCode   bool       `json:"has_group_qr_code"`
	AdminWechat      string     `json:"admin_wechat"`

	// 额度预约制参数（设计文档 §3）
	WeeklyLimitUSD float64 `json:"weekly_limit_usd"`
	SeatFeeCNY     float64 `json:"seat_fee_cny"`
	UsagePoolCNY   float64 `json:"usage_pool_cny"`
	ReserveRatio   float64 `json:"reserve_ratio"`
	LaunchMinRatio float64 `json:"launch_min_ratio"`
	LaunchMaxRatio float64 `json:"launch_max_ratio"`

	// 车外展示指标（设计文档 §4.6），由 FillDerivedMetrics 计算
	DeclaredTotalUSD     float64 `json:"declared_total_usd"`
	RemainingJoinableUSD float64 `json:"remaining_joinable_usd"`
	PlusEquivalents      float64 `json:"plus_equivalents"`
	AvgPriceCNY          float64 `json:"avg_price_cny"`
}

// FillDerivedMetrics 由额度池参数与总申报推导展示指标：
// 剩余可预约额度 = launch_max×周限额 − Σ申报；Plus 等价数 = Σ申报/(周限额/20)；
// 均价 = [席位费 + 变动池×(Σ/周限额)] / Plus 等价数（Σ=0 时为 0）。
func (c *Carpool) FillDerivedMetrics() {
	remaining := c.LaunchMaxRatio*c.WeeklyLimitUSD - c.DeclaredTotalUSD
	if remaining < 0 {
		remaining = 0
	}
	c.RemainingJoinableUSD = remaining

	plusEquivUSD := carpoolPlusEquivalentUSD(c.WeeklyLimitUSD)
	if plusEquivUSD <= 0 {
		c.PlusEquivalents = 0
		c.AvgPriceCNY = 0
		return
	}
	c.PlusEquivalents = c.DeclaredTotalUSD / plusEquivUSD
	if c.PlusEquivalents > 0 {
		totalPrice := c.SeatFeeCNY
		if c.WeeklyLimitUSD > 0 {
			totalPrice += c.UsagePoolCNY * c.DeclaredTotalUSD / c.WeeklyLimitUSD
		}
		c.AvgPriceCNY = totalPrice / c.PlusEquivalents
	} else {
		c.AvgPriceCNY = 0
	}
}

type CreateCarpoolInput struct {
	Name             string
	Description      string
	CarType          string
	Level            int
	Visibility       string
	ScheduledStartAt *time.Time

	// 额度池/价格参数，零值表示使用默认（设计文档 §3）
	WeeklyLimitUSD float64
	SeatFeeCNY     float64
	UsagePoolCNY   float64
	ReserveRatio   float64
	LaunchMinRatio float64
	LaunchMaxRatio float64

	// DeclaredWeeklyQuotaUSD 是 owner 本人的申报（可选）：>0 表示 owner 也占额度拼车，
	// 写入 owner 成员记录并按 1 人记账预付；0 表示 owner 仅发起、不占用额度。
	DeclaredWeeklyQuotaUSD float64

	// AddedAdminWechat 创建车辆的强制确认：发起人已添加管理员微信（CarpoolAdminWechatID）。
	AddedAdminWechat bool
	// GroupQRCode 是微信群二维码（必填）：data URL 或纯 base64，png/jpeg/webp，≤2MB。
	GroupQRCode string

	// 解析后的二维码字节与内容类型，由 Create 校验后传给 repo 落库（不直接暴露给 JSON）。
	GroupQRCodeBytes       []byte `json:"-"`
	GroupQRCodeContentType string `json:"-"`
}

func (input *CreateCarpoolInput) applyQuotaDefaults() {
	if input.CarType == "" {
		input.CarType = CarpoolTypeSmall
	}
	if input.Level == 0 {
		input.Level = 1
	}
	if input.WeeklyLimitUSD <= 0 {
		input.WeeklyLimitUSD = CarpoolDefaultWeeklyLimitUSD
	}
	if input.SeatFeeCNY <= 0 {
		input.SeatFeeCNY = CarpoolDefaultSeatFeeCNY
	}
	if input.UsagePoolCNY <= 0 {
		input.UsagePoolCNY = CarpoolDefaultUsagePoolCNY
	}
	if input.ReserveRatio <= 0 {
		input.ReserveRatio = CarpoolDefaultReserveRatio
	}
	if input.LaunchMinRatio <= 0 {
		input.LaunchMinRatio = CarpoolDefaultLaunchMinRatio
	}
	if input.LaunchMaxRatio <= 0 {
		input.LaunchMaxRatio = CarpoolDefaultLaunchMaxRatio
	}
}

type CarpoolMutationResult struct {
	Carpool          *Carpool `json:"carpool"`
	InviteToken      string   `json:"invite_token,omitempty"`
	ActivatedUserIDs []int64  `json:"-"`
	ActivatedGroupID int64    `json:"-"`

	// 上车记账结果（设计文档 §4.4）
	DeclaredWeeklyQuotaUSD float64 `json:"declared_weekly_quota_usd,omitempty"`
	PrepaidAmountCNY       float64 `json:"prepaid_amount_cny,omitempty"`

	// LaunchBandEntered 表示本次上车使 Σ申报 首次进入发车区间（repo 已在同事务
	// 置 launch_notified_at），service 据此在提交后通知车主确认发车。
	LaunchBandEntered bool `json:"-"`
}

// CarpoolSettlementMemberRow 是结算用的成员数据行（repo 层查询结果）。
type CarpoolSettlementMemberRow struct {
	UserID                 int64
	Role                   string
	DeclaredWeeklyQuotaUSD float64
	PrepaidAmountCNY       float64
	ActualUsageUSD         float64
	PeriodStart            *time.Time
	PeriodEnd              *time.Time
}

// CarpoolSettlement 是月度结算单（设计文档 §4.5）。
type CarpoolSettlement struct {
	CarpoolID      int64                     `json:"carpool_id"`
	Status         string                    `json:"status"`
	WeeklyLimitUSD float64                   `json:"weekly_limit_usd"`
	SeatFeeCNY     float64                   `json:"seat_fee_cny"`
	UsagePoolCNY   float64                   `json:"usage_pool_cny"`
	ReserveRatio   float64                   `json:"reserve_ratio"`
	MemberCount    int                       `json:"member_count"`
	FullView       bool                      `json:"full_view"` // owner/admin 见全车，普通成员仅见自己
	PeriodStart    *time.Time                `json:"period_start,omitempty"`
	PeriodEnd      *time.Time                `json:"period_end,omitempty"`
	Members        []CarpoolSettlementMember `json:"members"`
}

type CarpoolRepository interface {
	List(ctx context.Context, userID int64) ([]Carpool, error)
	GetByID(ctx context.Context, carpoolID, userID int64) (*Carpool, error)
	GetByInvite(ctx context.Context, userID int64, tokenHash string) (*Carpool, error)
	Create(ctx context.Context, ownerUserID int64, input CreateCarpoolInput, inviteHash, inviteHint string) (*CarpoolMutationResult, error)
	CreateInvite(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool, inviteHash, inviteHint string) error
	Join(ctx context.Context, carpoolID, userID int64, declaredWeeklyQuotaUSD float64, joinedWechatGroup bool, inviteHash *string) (*CarpoolMutationResult, error)
	Leave(ctx context.Context, carpoolID, userID int64) (*CarpoolMutationResult, error)
	Confirm(ctx context.Context, carpoolID, ownerUserID int64) (*CarpoolMutationResult, error)
	Launch(ctx context.Context, carpoolID, actorUserID int64, isAdmin, force bool) (*CarpoolMutationResult, error)
	Cancel(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error
	SetJoinLocked(ctx context.Context, carpoolID, actorUserID int64, locked bool) error
	GetGroupQRCode(ctx context.Context, carpoolID int64) (data []byte, contentType string, err error)
	ListSettlementMembers(ctx context.Context, carpoolID int64) ([]CarpoolSettlementMemberRow, error)
	GetRecentWeeklyUsageStats(ctx context.Context, userID int64) (totalUSD float64, daysWithRecords int, err error)
}

// CarpoolEmailSender 是拼车通知邮件的最小发送接口（*EmailService 实现）。
// 发送失败只记日志，绝不影响主流程。
type CarpoolEmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// CarpoolUserDirectory 是拼车通知邮件所需的最小用户查询接口（UserRepository 实现）。
type CarpoolUserDirectory interface {
	GetByID(ctx context.Context, id int64) (*User, error)
	ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error)
}

type CarpoolService struct {
	repo                CarpoolRepository
	subscriptionService *SubscriptionService
	emailSender         CarpoolEmailSender
	userDirectory       CarpoolUserDirectory
}

func NewCarpoolService(repo CarpoolRepository, subscriptionService *SubscriptionService, emailService *EmailService, userRepo UserRepository) *CarpoolService {
	svc := &CarpoolService{repo: repo, subscriptionService: subscriptionService}
	if emailService != nil {
		svc.emailSender = emailService
	}
	if userRepo != nil {
		svc.userDirectory = userRepo
	}
	return svc
}

func (s *CarpoolService) List(ctx context.Context, userID int64) ([]Carpool, error) {
	items, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		fillCarpoolPresentation(&items[i])
	}
	return items, nil
}

func (s *CarpoolService) ResolveInvite(ctx context.Context, userID int64, token string) (*Carpool, error) {
	item, err := s.repo.GetByInvite(ctx, userID, hashInviteToken(token))
	if err != nil {
		return nil, err
	}
	fillCarpoolPresentation(item)
	return item, nil
}

// fillCarpoolPresentation 补充响应展示字段：派生指标 + 硬编码管理员微信号。
func fillCarpoolPresentation(c *Carpool) {
	if c == nil {
		return
	}
	c.FillDerivedMetrics()
	c.AdminWechat = CarpoolAdminWechatID
}

func (s *CarpoolService) Create(ctx context.Context, ownerUserID int64, input CreateCarpoolInput) (*CarpoolMutationResult, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.applyQuotaDefaults()
	if input.Name == "" || len(input.Name) > 100 || len(input.Description) > 300 || input.Level < 1 || input.Level > 10 {
		return nil, ErrCarpoolInvalidRequest
	}
	if input.CarType != CarpoolTypeSmall && input.CarType != CarpoolTypeLarge {
		return nil, ErrCarpoolInvalidRequest
	}
	if input.Visibility != CarpoolVisibilityPublic && input.Visibility != CarpoolVisibilityInviteOnly {
		return nil, ErrCarpoolInvalidRequest
	}
	if input.WeeklyLimitUSD <= 0 || input.WeeklyLimitUSD > 1e9 ||
		input.ReserveRatio <= 0 || input.ReserveRatio > 1 ||
		input.LaunchMinRatio <= 0 || input.LaunchMinRatio > input.LaunchMaxRatio ||
		input.DeclaredWeeklyQuotaUSD < 0 || input.DeclaredWeeklyQuotaUSD > 1e6 {
		return nil, ErrCarpoolInvalidRequest
	}
	// 两项强制确认：已添加管理员微信 + 上传微信群二维码。
	if !input.AddedAdminWechat {
		return nil, ErrCarpoolContactConfirmRequired
	}
	qrCode, qrCodeContentType, err := parseCarpoolGroupQRCode(input.GroupQRCode)
	if err != nil {
		return nil, err
	}
	input.GroupQRCodeBytes = qrCode
	input.GroupQRCodeContentType = qrCodeContentType
	token, hash, hint, err := newInviteToken()
	if err != nil {
		return nil, fmt.Errorf("generate carpool invite: %w", err)
	}
	result, err := s.repo.Create(ctx, ownerUserID, input, hash, hint)
	if err != nil {
		return nil, err
	}
	result.InviteToken = token
	fillCarpoolPresentation(result.Carpool)
	return result, nil
}

// parseCarpoolGroupQRCode 校验并解码群二维码：接受 data URL 或纯 base64，
// 解码后 ≤2MB，且 sniff 出的内容类型必须是 png/jpeg/webp。
func parseCarpoolGroupQRCode(raw string) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", ErrCarpoolGroupQRCodeRequired
	}
	encoded := raw
	if strings.HasPrefix(raw, "data:") {
		comma := strings.Index(raw, ",")
		if comma < 0 {
			return nil, "", ErrCarpoolGroupQRCodeInvalid
		}
		encoded = raw[comma+1:]
	}
	// base64 体积约为原始字节的 4/3，解码前先做上限预检。
	encoded = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, encoded)
	if len(encoded) > (CarpoolGroupQRCodeMaxBytes+2)/3*4+4 {
		return nil, "", ErrCarpoolGroupQRCodeInvalid
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data) > CarpoolGroupQRCodeMaxBytes {
		return nil, "", ErrCarpoolGroupQRCodeInvalid
	}
	contentType := http.DetectContentType(data)
	switch contentType {
	case "image/png", "image/jpeg", "image/webp":
		return data, contentType, nil
	default:
		return nil, "", ErrCarpoolGroupQRCodeInvalid
	}
}

func (s *CarpoolService) CreateInvite(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (string, error) {
	token, hash, hint, err := newInviteToken()
	if err != nil {
		return "", fmt.Errorf("generate carpool invite: %w", err)
	}
	if err := s.repo.CreateInvite(ctx, carpoolID, actorUserID, isAdmin, hash, hint); err != nil {
		return "", err
	}
	return token, nil
}

func (s *CarpoolService) Join(ctx context.Context, carpoolID, userID int64, declaredWeeklyQuotaUSD float64, joinedWechatGroup bool) (*CarpoolMutationResult, error) {
	if declaredWeeklyQuotaUSD <= 0 || declaredWeeklyQuotaUSD > 1e6 {
		return nil, ErrCarpoolInvalidRequest
	}
	if !joinedWechatGroup {
		return nil, ErrCarpoolGroupJoinRequired
	}
	result, err := s.repo.Join(ctx, carpoolID, userID, declaredWeeklyQuotaUSD, joinedWechatGroup, nil)
	if err != nil {
		return nil, err
	}
	s.invalidateLaunchedSubscriptions(result)
	s.notifyOwnerLaunchBandEntered(ctx, result)
	fillCarpoolPresentation(result.Carpool)
	return result, nil
}

func (s *CarpoolService) JoinByInvite(ctx context.Context, token string, userID int64, declaredWeeklyQuotaUSD float64, joinedWechatGroup bool) (*CarpoolMutationResult, error) {
	if declaredWeeklyQuotaUSD <= 0 || declaredWeeklyQuotaUSD > 1e6 {
		return nil, ErrCarpoolInvalidRequest
	}
	if !joinedWechatGroup {
		return nil, ErrCarpoolGroupJoinRequired
	}
	hash := hashInviteToken(token)
	result, err := s.repo.Join(ctx, 0, userID, declaredWeeklyQuotaUSD, joinedWechatGroup, &hash)
	if err != nil {
		return nil, err
	}
	s.invalidateLaunchedSubscriptions(result)
	s.notifyOwnerLaunchBandEntered(ctx, result)
	fillCarpoolPresentation(result.Carpool)
	return result, nil
}

// Leave 下车：仅 recruiting 状态；普通成员把成员行置为 left（幂等），申报额度
// 即时释放（Σ 统计只算 joined/active 成员），并可能触发 launch_notified_at 重置。
// 车主下车返回 409（只能 cancel 整车）。
func (s *CarpoolService) Leave(ctx context.Context, carpoolID, userID int64) (*CarpoolMutationResult, error) {
	result, err := s.repo.Leave(ctx, carpoolID, userID)
	if err != nil {
		return nil, err
	}
	fillCarpoolPresentation(result.Carpool)
	return result, nil
}

// Confirm 车主确认发车（两段确认第一段）：仅 owner、recruiting、Σ申报在
// [launch_min, launch_max]×周限额 区间内。提交后通知所有 admin 在 24 小时内启动。
func (s *CarpoolService) Confirm(ctx context.Context, carpoolID, ownerUserID int64) (*CarpoolMutationResult, error) {
	result, err := s.repo.Confirm(ctx, carpoolID, ownerUserID)
	if err != nil {
		return nil, err
	}
	s.notifyAdminsCarpoolConfirmed(ctx, result.Carpool)
	fillCarpoolPresentation(result.Carpool)
	return result, nil
}

// Launch 管理员启动发车（两段确认第二段）：仅 admin；正常发车要求车已 confirmed，
// force=true 用于招募不足的降档发车（要求 recruiting 且 Σ申报≥80%×周限额，跳过确认）。
// 启动成功后给每位成员发邮件（失败仅记日志）。
func (s *CarpoolService) Launch(ctx context.Context, carpoolID, actorUserID int64, isAdmin, force bool) (*CarpoolMutationResult, error) {
	result, err := s.repo.Launch(ctx, carpoolID, actorUserID, isAdmin, force)
	if err != nil {
		return nil, err
	}
	s.invalidateLaunchedSubscriptions(result)
	s.notifyMembersCarpoolLaunched(ctx, result)
	fillCarpoolPresentation(result.Carpool)
	return result, nil
}

func (s *CarpoolService) Cancel(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error {
	return s.repo.Cancel(ctx, carpoolID, actorUserID, isAdmin)
}

func (s *CarpoolService) SetJoinLocked(ctx context.Context, carpoolID, actorUserID int64, isAdmin, locked bool) error {
	if !isAdmin {
		return ErrCarpoolForbidden
	}
	return s.repo.SetJoinLocked(ctx, carpoolID, actorUserID, locked)
}

// GetGroupQRCode 返回车辆的微信群二维码字节与内容类型（任何登录用户可读，含未上车者）。
func (s *CarpoolService) GetGroupQRCode(ctx context.Context, carpoolID int64) ([]byte, string, error) {
	return s.repo.GetGroupQRCode(ctx, carpoolID)
}

// GetDeclarationRecommendation 按设计文档 §4.1 从 usage_logs 聚合本人最近 7 天
// 用量并给出推荐申报值（不足 7 天按日均外推 ×7，无记录返回锚点建议）。
func (s *CarpoolService) GetDeclarationRecommendation(ctx context.Context, userID int64) (*CarpoolDeclarationRecommendation, error) {
	totalUSD, days, err := s.repo.GetRecentWeeklyUsageStats(ctx, userID)
	if err != nil {
		return nil, err
	}
	rec := BuildDeclarationRecommendation(totalUSD, days, CarpoolDeclarationBufferRatio)
	return &rec, nil
}

// GetSettlement 按设计文档 §4.5 输出月度结算单（含 80% 地板规则）。
// 成员仅见自己的结算行，owner/admin 见全车。
func (s *CarpoolService) GetSettlement(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*CarpoolSettlement, error) {
	item, err := s.repo.GetByID(ctx, carpoolID, actorUserID)
	if err != nil {
		return nil, err
	}
	isOwner := item.OwnerUserID != nil && *item.OwnerUserID == actorUserID
	if !isAdmin && !isOwner && item.MemberRole == nil {
		return nil, ErrCarpoolForbidden
	}

	rows, err := s.repo.ListSettlementMembers(ctx, carpoolID)
	if err != nil {
		return nil, err
	}

	settlement := &CarpoolSettlement{
		CarpoolID:      item.ID,
		Status:         item.Status,
		WeeklyLimitUSD: item.WeeklyLimitUSD,
		SeatFeeCNY:     item.SeatFeeCNY,
		UsagePoolCNY:   item.UsagePoolCNY,
		ReserveRatio:   item.ReserveRatio,
		MemberCount:    len(rows),
		FullView:       isAdmin || isOwner,
	}

	inputs := make([]CarpoolSettlementMemberInput, 0, len(rows))
	for _, row := range rows {
		periodDays := 30.0
		if row.PeriodStart != nil && row.PeriodEnd != nil && row.PeriodEnd.After(*row.PeriodStart) {
			periodDays = row.PeriodEnd.Sub(*row.PeriodStart).Hours() / 24
			if settlement.PeriodStart == nil {
				start, end := *row.PeriodStart, *row.PeriodEnd
				settlement.PeriodStart = &start
				settlement.PeriodEnd = &end
			}
		}
		inputs = append(inputs, CarpoolSettlementMemberInput{
			UserID:                 row.UserID,
			Role:                   row.Role,
			DeclaredWeeklyQuotaUSD: row.DeclaredWeeklyQuotaUSD,
			PrepaidAmountCNY:       row.PrepaidAmountCNY,
			ActualUsageUSD:         row.ActualUsageUSD,
			PeriodDays:             periodDays,
		})
	}

	members := ComputeCarpoolSettlementMembers(item.WeeklyLimitUSD, item.SeatFeeCNY, item.UsagePoolCNY, item.ReserveRatio, inputs)
	if settlement.FullView {
		settlement.Members = members
	} else {
		settlement.Members = make([]CarpoolSettlementMember, 0, 1)
		for _, member := range members {
			if member.UserID == actorUserID {
				settlement.Members = append(settlement.Members, member)
			}
		}
	}
	return settlement, nil
}

func (s *CarpoolService) invalidateLaunchedSubscriptions(result *CarpoolMutationResult) {
	if result == nil || result.ActivatedGroupID <= 0 || s.subscriptionService == nil {
		return
	}
	for _, userID := range result.ActivatedUserIDs {
		_ = s.subscriptionService.invalidateSubscriptionCaches(userID, result.ActivatedGroupID)
	}
}

// sendCarpoolNotification 发送拼车通知邮件。SMTP 未配置或发送失败时仅记日志，
// 绝不影响主流程（优雅降级）。
func (s *CarpoolService) sendCarpoolNotification(ctx context.Context, to, subject, body string) {
	if s.emailSender == nil || strings.TrimSpace(to) == "" {
		return
	}
	if err := s.emailSender.SendEmail(ctx, to, subject, body); err != nil {
		slog.Warn("carpool notification email failed", "subject", subject, "error", err)
	}
}

func (s *CarpoolService) userEmail(ctx context.Context, userID int64) string {
	if s.userDirectory == nil {
		return ""
	}
	user, err := s.userDirectory.GetByID(ctx, userID)
	if err != nil {
		slog.Warn("carpool notification: load user failed", "user_id", userID, "error", err)
		return ""
	}
	return user.Email
}

// notifyOwnerLaunchBandEntered Σ申报 首次进入发车区间后通知车主登录确认发车。
func (s *CarpoolService) notifyOwnerLaunchBandEntered(ctx context.Context, result *CarpoolMutationResult) {
	if result == nil || !result.LaunchBandEntered || result.Carpool == nil || result.Carpool.OwnerUserID == nil {
		return
	}
	carpool := result.Carpool
	body := fmt.Sprintf(
		`<p>你发起的拼车「%s」总申报额度已达到 $%.2f，进入发车区间（$%.2f ~ $%.2f）。</p>`+
			`<p>请尽快登录平台确认发车；确认后管理员将在 24 小时内启动。</p>`,
		carpool.Name, carpool.DeclaredTotalUSD,
		carpool.LaunchMinRatio*carpool.WeeklyLimitUSD, carpool.LaunchMaxRatio*carpool.WeeklyLimitUSD)
	s.sendCarpoolNotification(ctx, s.userEmail(ctx, *carpool.OwnerUserID), "拼车已达发车区间，请登录确认发车", body)
}

// notifyAdminsCarpoolConfirmed 车主确认后通知所有 admin 在 24 小时内启动。
func (s *CarpoolService) notifyAdminsCarpoolConfirmed(ctx context.Context, carpool *Carpool) {
	if s.userDirectory == nil || carpool == nil {
		return
	}
	includeSubscriptions := false
	admins, _, err := s.userDirectory.ListWithFilters(ctx,
		pagination.PaginationParams{Page: 1, PageSize: 100},
		UserListFilters{Role: RoleAdmin, Status: StatusActive, IncludeSubscriptions: &includeSubscriptions})
	if err != nil {
		slog.Warn("carpool notification: list admins failed", "carpool_id", carpool.ID, "error", err)
		return
	}
	body := fmt.Sprintf(
		`<p>拼车「%s」（ID %d）已由车主确认发车，总申报额度 $%.2f。</p>`+
			`<p>请在 24 小时内登录管理后台执行启动。</p>`,
		carpool.Name, carpool.ID, carpool.DeclaredTotalUSD)
	for _, admin := range admins {
		s.sendCarpoolNotification(ctx, admin.Email, fmt.Sprintf("拼车「%s」已确认，请 24 小时内启动", carpool.Name), body)
	}
}

// notifyMembersCarpoolLaunched 启动成功后逐一通知成员拼车已发车。
func (s *CarpoolService) notifyMembersCarpoolLaunched(ctx context.Context, result *CarpoolMutationResult) {
	if result == nil || result.Carpool == nil || len(result.ActivatedUserIDs) == 0 {
		return
	}
	carpool := result.Carpool
	body := fmt.Sprintf(
		`<p>你参加的拼车「%s」已发车，订阅已开通并写入个人周限额。</p>`+
			`<p>请登录平台查看可用额度与结算规则。</p>`, carpool.Name)
	subject := fmt.Sprintf("你参加的拼车「%s」已发车", carpool.Name)
	for _, userID := range result.ActivatedUserIDs {
		s.sendCarpoolNotification(ctx, s.userEmail(ctx, userID), subject, body)
	}
}

func newInviteToken() (token, hash, hint string, err error) {
	raw := make([]byte, 24)
	if _, err = rand.Read(raw); err != nil {
		return "", "", "", err
	}
	token = hex.EncodeToString(raw)
	hash = hashInviteToken(token)
	hint = token[:8]
	return token, hash, hint, nil
}

func hashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

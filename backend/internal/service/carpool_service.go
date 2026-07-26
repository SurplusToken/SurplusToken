package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"sync"
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

	// CarpoolPricingQuota 是额度预约制（设计文档 §3/§4），新车默认。
	CarpoolPricingQuota = "quota"
	// CarpoolPricingCustom 是自定义规则车：不适用申报/保底/自动退补，
	// 按 rule_note 写明的规则人工结算。平台升级前建立的老车全部属于此类
	// （migration 192），管理员协商后开的特殊车也走这里。
	CarpoolPricingCustom = "custom"
)

// IsQuotaModel 报告本车是否走额度预约制。非额度制的车不参与申报、保底、
// 公共池与自动退补计算——那套数学对它们没有意义，硬算出来的是假账。
func (c *Carpool) IsQuotaModel() bool {
	return c == nil || c.PricingModel == "" || c.PricingModel == CarpoolPricingQuota
}

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

	// 额度池参数自助收口：非 admin 不得自定义整车额度/价格/保底参数，
	// 需要非默认规则时走 custom-rule-interest 与管理员协商（设计文档 §3）。
	ErrCarpoolCustomParamsForbidden = infraerrors.Forbidden("CARPOOL_CUSTOM_PARAMS_FORBIDDEN", "custom quota parameters require an administrator; use the custom-rule enquiry instead")
	// 申报下限与成员数硬上限（防止 $0.01 占席与单事务放大）。
	ErrCarpoolDeclarationTooSmall = infraerrors.BadRequest("CARPOOL_DECLARATION_TOO_SMALL", "declared weekly quota is below the minimum")
	ErrCarpoolFull                = infraerrors.Conflict("CARPOOL_FULL", "carpool has reached its member limit")
	// 自定义规则咨询限流。
	ErrCarpoolInterestTooFrequent = infraerrors.TooManyRequests("CARPOOL_INTEREST_TOO_FREQUENT", "custom rule enquiry was submitted recently; please wait before retrying")

	// 自定义规则车（migration 192）：不走申报/保底那套，也不接新成员。
	ErrCarpoolCustomRuleClosed = infraerrors.Conflict("CARPOOL_CUSTOM_RULE_CLOSED", "this carpool runs under custom rules and no longer accepts new members")

	// 结算落库（migration 191）。
	ErrCarpoolAlreadySettled = infraerrors.Conflict("CARPOOL_ALREADY_SETTLED", "carpool settlement is already frozen")
	ErrCarpoolNotSettled     = infraerrors.Conflict("CARPOOL_NOT_SETTLED", "carpool settlement has not been frozen yet")
	ErrCarpoolNotSettleable  = infraerrors.Conflict("CARPOOL_NOT_SETTLEABLE", "only a launched carpool can be settled")
)

const (
	// CarpoolAdminWechatID 是硬编码的管理员微信号，创建车辆前必须先添加。
	CarpoolAdminWechatID = "Charlemartingale"
	// CarpoolAdminEmail 是拼车运营联系人邮箱：所有"通知管理员"类的拼车邮件
	// （车主确认发车、自定义规则咨询）都只发到这里。
	//
	// 刻意不查 users 表里的 role=admin：那是**授权**角色，一个部署可能有若干个
	// 运维/客服管理员，他们不负责拼车运营；拼车的联系人是一个具体的人，与
	// CarpoolAdminWechatID 指的是同一个人。两者放在一起，换人时一并改这两行。
	// 平台其他模块（余额告警、运维告警等）的管理员通知不受此影响。
	CarpoolAdminEmail = "zhongshanhu@stu.pku.edu.cn"
	// CarpoolGroupQRCodeMaxBytes 是群二维码解码后的字节上限（2MB）。
	CarpoolGroupQRCodeMaxBytes = 2 << 20

	// CarpoolMaxMembers 是单车成员硬上限。发车是单事务、逐成员建订阅，
	// 成员数直接决定该事务的大小；同时也约束公共池的并发争抢面。
	CarpoolMaxMembers = 30
	// CarpoolMinDeclaredWeeklyQuotaUSD 是申报下限（≈1/6 个 Plus 等价）。
	// 没有下限时 $0.01 即可占一个席位、白拿公共池准入。
	CarpoolMinDeclaredWeeklyQuotaUSD = 20.0
	// CarpoolMaxDeclaredWeeklyQuotaUSD 是单人申报上限（防呆，实际还受
	// launch_max×周限额 的整车上限约束）。
	CarpoolMaxDeclaredWeeklyQuotaUSD = 1e6

	// CarpoolInterestCooldown 是自定义规则咨询的每用户冷却窗口。
	CarpoolInterestCooldown = 30 * time.Minute
	// CarpoolInterestNoteMaxRunes 是随咨询邮件转发的备注长度上限。
	CarpoolInterestNoteMaxRunes = 500
	// carpoolInterestTrackerMaxEntries 是冷却表的容量上限，超出后清理过期项。
	carpoolInterestTrackerMaxEntries = 4096
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
	// SettledAt 非空表示结算单已冻结（migration 191），金额不再随用量变化。
	SettledAt       *time.Time `json:"settled_at,omitempty"`
	SettledByUserID *int64     `json:"settled_by_user_id,omitempty"`
	HasGroupQRCode  bool       `json:"has_group_qr_code"`
	AdminWechat     string     `json:"admin_wechat"`

	// PricingModel 区分额度预约制（quota）与自定义规则车（custom，人工结算）。
	// RuleNote 是自定义规则车的规则说明，展示给成员看。
	PricingModel string `json:"pricing_model"`
	RuleNote     string `json:"rule_note,omitempty"`

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

// hasCustomQuotaParams 报告请求是否试图自定义额度池/价格参数（任一非零即为是）。
// 仅 admin 允许，普通用户走协商流程——否则任何登录用户都能造出"整车 $10 亿"的车，
// 而发车时这些参数会原样写进 group 与订阅限额。
func (input *CreateCarpoolInput) hasCustomQuotaParams() bool {
	return input.WeeklyLimitUSD != 0 || input.SeatFeeCNY != 0 || input.UsagePoolCNY != 0 ||
		input.ReserveRatio != 0 || input.LaunchMinRatio != 0 || input.LaunchMaxRatio != 0
}

// validateQuotaParams 校验额度池参数自洽（在 applyQuotaDefaults 之后调用）。
//
// 关键约束是 reserveRatio×launchMaxRatio < 1：公共池容量 C = 周限额 − reserve×Σ申报，
// 而 Σ申报 最大可到 launchMax×周限额，故 C ≥ (1 − reserve×launchMax)×周限额。
// 该乘积 ≥ 1 时 C 可能为 0 或负，此时 CarpoolSharedPoolCapacityUSD 返回 0，
// 组级公共池检查会被整体跳过——硬约束静默失效，比"负公共池"严重得多。
func (input *CreateCarpoolInput) validateQuotaParams() error {
	if input.WeeklyLimitUSD <= 0 || input.WeeklyLimitUSD > 1e9 {
		return ErrCarpoolInvalidRequest
	}
	if input.SeatFeeCNY <= 0 || input.SeatFeeCNY > 1e7 || input.UsagePoolCNY <= 0 || input.UsagePoolCNY > 1e7 {
		return ErrCarpoolInvalidRequest
	}
	if input.ReserveRatio <= 0 || input.ReserveRatio > 1 {
		return ErrCarpoolInvalidRequest
	}
	if input.LaunchMinRatio <= 0 || input.LaunchMaxRatio <= 0 ||
		input.LaunchMinRatio > input.LaunchMaxRatio || input.LaunchMaxRatio > 2 {
		return ErrCarpoolInvalidRequest
	}
	// 公共池必须严格为正，否则组级硬约束失效（见函数注释）。
	if input.ReserveRatio*input.LaunchMaxRatio >= 1 {
		return ErrCarpoolInvalidRequest
	}
	// owner 申报：0 表示仅发起不占额度；>0 时受申报下限与整车上限双重约束
	// （超过整车上限的车永远进不了发车区间，会一直卡在 recruiting）。
	if input.DeclaredWeeklyQuotaUSD < 0 || input.DeclaredWeeklyQuotaUSD > CarpoolMaxDeclaredWeeklyQuotaUSD {
		return ErrCarpoolInvalidRequest
	}
	if input.DeclaredWeeklyQuotaUSD > 0 {
		if input.DeclaredWeeklyQuotaUSD < CarpoolMinDeclaredWeeklyQuotaUSD {
			return ErrCarpoolDeclarationTooSmall
		}
		if input.DeclaredWeeklyQuotaUSD > input.LaunchMaxRatio*input.WeeklyLimitUSD {
			return ErrCarpoolQuotaExceeded
		}
	}
	return nil
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
	UserID int64
	// Email/Username 供车主把结算行对应到真人收款（仅 FullView 下输出）。
	Email                  string
	Username               string
	Role                   string
	DeclaredWeeklyQuotaUSD float64
	PrepaidAmountCNY       float64
	QuotedPrepaidCNY       float64
	ActualUsageUSD         float64
	PeriodStart            *time.Time
	PeriodEnd              *time.Time

	// Frozen 是已结算车辆的冻结快照；nil 表示该成员行尚未结算，按实时计算。
	Frozen *CarpoolSettlementFrozenRow
}

// CarpoolSettlementFrozenRow 是结算时冻结下来的金额快照（migration 191）。
// 实时重算会随 monthly_usage_usd 漂移——车主按 A 收的钱，第二天读到的是 B。
type CarpoolSettlementFrozenRow struct {
	FloorUsageUSD    float64
	ActualUsageUSD   float64
	BillableUsageUSD float64
	UsageShareCNY    float64
	SeatFeeCNY       float64
	TotalDeltaCNY    float64
}

// CarpoolPendingLaunch 是 admin"待启动"列表的一行：车主已确认、等待管理员启动的车。
// 有了这个列表，确认通知邮件丢失也不会让车连人带钱无限挂起。
type CarpoolPendingLaunch struct {
	CarpoolID        int64     `json:"carpool_id"`
	Name             string    `json:"name"`
	OwnerUserID      *int64    `json:"owner_user_id,omitempty"`
	OwnerEmail       string    `json:"owner_email,omitempty"`
	MemberCount      int       `json:"member_count"`
	DeclaredTotalUSD float64   `json:"declared_total_usd"`
	WeeklyLimitUSD   float64   `json:"weekly_limit_usd"`
	ConfirmedAt      time.Time `json:"confirmed_at"`
	// PendingHours 是已等待时长（小时），> CarpoolLaunchSLAHours 即超出 24 小时承诺。
	PendingHours float64 `json:"pending_hours"`
	Overdue      bool    `json:"overdue"`
}

// CarpoolLaunchSLAHours 是确认后承诺的启动时限（小时），与通知邮件里的措辞一致。
const CarpoolLaunchSLAHours = 24.0

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

	// Settled 为 true 时 Members 来自结算时冻结的快照，不随后续用量变化；
	// 为 false 时是实时预览（数字会随用量继续走）。
	Settled          bool       `json:"settled"`
	SettledAt        *time.Time `json:"settled_at,omitempty"`
	SettledByUserID  *int64     `json:"settled_by_user_id,omitempty"`
	CanSettle        bool       `json:"can_settle"`         // 调用者此刻能否执行结算
	SettleBlockedFor string     `json:"settle_blocked_for"` // 不能结算的原因（前端提示用）

	// ManualSettlement 为 true 时本车按 RuleNote 写明的规则人工结算：
	// Members 只带实际用量，全部金额字段为零（不是"算出来是 0"，是"不适用"）。
	ManualSettlement bool   `json:"manual_settlement"`
	PricingModel     string `json:"pricing_model"`
	RuleNote         string `json:"rule_note,omitempty"`
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
	Unconfirm(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*CarpoolMutationResult, error)
	ListPendingLaunch(ctx context.Context) ([]CarpoolPendingLaunch, error)
	Launch(ctx context.Context, carpoolID, actorUserID int64, isAdmin, force bool) (*CarpoolMutationResult, error)
	Cancel(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error
	SetJoinLocked(ctx context.Context, carpoolID, actorUserID int64, locked bool) error
	GetGroupQRCode(ctx context.Context, carpoolID int64) (data []byte, contentType string, err error)
	ListSettlementMembers(ctx context.Context, carpoolID int64) ([]CarpoolSettlementMemberRow, error)
	// PersistSettlement 冻结结算：仅当车未结算时写入（settled_at IS NULL 为幂等守卫），
	// 已结算返回 ErrCarpoolAlreadySettled。
	PersistSettlement(ctx context.Context, carpoolID, actorUserID int64, members []CarpoolSettlementMember) error
	// ClearSettlement 撤销结算（admin 出口），清空车与成员上的全部冻结字段。
	ClearSettlement(ctx context.Context, carpoolID, actorUserID int64) error
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

	// interestMu/interestLastAt 是自定义规则咨询的每用户冷却表（进程内）。
	// 多实例部署下每实例各自限流——对"给 admin 发提示邮件"这种低频入口足够，
	// 且不引入新的 Redis 硬依赖。
	interestMu     sync.Mutex
	interestLastAt map[int64]time.Time
}

func NewCarpoolService(repo CarpoolRepository, subscriptionService *SubscriptionService, emailService *EmailService, userRepo UserRepository) *CarpoolService {
	svc := &CarpoolService{repo: repo, subscriptionService: subscriptionService, interestLastAt: make(map[int64]time.Time)}
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
//
// 自定义规则车不算派生指标：剩余可预约额度 / Plus 等价 / 均价 都是额度预约制
// 的概念，对它们而言分母和分子都不成立（成员申报恒为 0），算出来只会在卡片上
// 显示 "0 / 2400"、"均价 ¥0" 这种误导性数字。
func fillCarpoolPresentation(c *Carpool) {
	if c == nil {
		return
	}
	if c.IsQuotaModel() {
		c.FillDerivedMetrics()
	} else {
		c.DeclaredTotalUSD = 0
		c.RemainingJoinableUSD = 0
		c.PlusEquivalents = 0
		c.AvgPriceCNY = 0
	}
	c.AdminWechat = CarpoolAdminWechatID
}

// Create 创建车辆。isAdmin 决定是否允许自定义额度池参数：普通用户一律使用
// 设计文档 §3 的默认参数（整车 $2400 / 席位费 ¥400 / 变动池 ¥1000 / 保底 80%），
// 需要别的规则请走 NotifyCustomRuleInterest 与管理员协商后由管理员开车。
func (s *CarpoolService) Create(ctx context.Context, ownerUserID int64, isAdmin bool, input CreateCarpoolInput) (*CarpoolMutationResult, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	// 额度参数自助收口：非 admin 传了任何非零额度参数直接拒绝（而非静默忽略，
	// 免得调用方以为参数生效了）。默认填充发生在这道校验之后。
	if !isAdmin && input.hasCustomQuotaParams() {
		return nil, ErrCarpoolCustomParamsForbidden
	}
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
	if err := input.validateQuotaParams(); err != nil {
		return nil, err
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

// validateJoinDeclaration 校验上车申报值：必须在 [下限, 上限] 内。
// 下限存在的理由见 CarpoolMinDeclaredWeeklyQuotaUSD——没有它，$0.01 就能占一个席位。
func validateJoinDeclaration(declaredWeeklyQuotaUSD float64) error {
	if declaredWeeklyQuotaUSD <= 0 || declaredWeeklyQuotaUSD > CarpoolMaxDeclaredWeeklyQuotaUSD {
		return ErrCarpoolInvalidRequest
	}
	if declaredWeeklyQuotaUSD < CarpoolMinDeclaredWeeklyQuotaUSD {
		return ErrCarpoolDeclarationTooSmall
	}
	return nil
}

func (s *CarpoolService) Join(ctx context.Context, carpoolID, userID int64, declaredWeeklyQuotaUSD float64, joinedWechatGroup bool) (*CarpoolMutationResult, error) {
	if err := validateJoinDeclaration(declaredWeeklyQuotaUSD); err != nil {
		return nil, err
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
	if err := validateJoinDeclaration(declaredWeeklyQuotaUSD); err != nil {
		return nil, err
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

// Unconfirm 撤回确认（confirmed → recruiting）：车主或 admin 均可。
//
// 没有这个出口时，confirmed 的车只能等 admin 手动 launch，而通知全靠一封可能丢失的
// 邮件——车会连人带钱无限期挂起，前端也没有任何入口（cancel 对 confirmed 只开放给
// admin）。撤回确认比取消整车温和：成员和申报都保留，重新开放上车。
func (s *CarpoolService) Unconfirm(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*CarpoolMutationResult, error) {
	result, err := s.repo.Unconfirm(ctx, carpoolID, actorUserID, isAdmin)
	if err != nil {
		return nil, err
	}
	fillCarpoolPresentation(result.Carpool)
	return result, nil
}

// ListPendingLaunch 返回全部等待管理员启动的车（仅 admin 可调用），按确认时间从早到晚。
func (s *CarpoolService) ListPendingLaunch(ctx context.Context, isAdmin bool) ([]CarpoolPendingLaunch, error) {
	if !isAdmin {
		return nil, ErrCarpoolForbidden
	}
	return s.repo.ListPendingLaunch(ctx)
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

// GetGroupQRCode 返回车辆的微信群二维码字节与内容类型，受可见性约束。
//
// 群二维码对 invite_only 车等同入场券（扫码即可进微信群），因此不能像原来那样
// 对任何登录用户开放——那等于绕过 List 的可见性控制。放行规则与 List 对齐：
//   - admin、车主、已上车成员：始终可读（含已发车/已结束的车，便于回群）；
//   - 招募中的 public 车：任何登录用户可读（上车前必须先入群，这是产品前提）；
//   - 招募中的 invite_only 车：必须随请求带上有效邀请 token；
//   - 其余一律 403。
func (s *CarpoolService) GetGroupQRCode(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool, inviteToken string) ([]byte, string, error) {
	item, err := s.repo.GetByID(ctx, carpoolID, actorUserID)
	if err != nil {
		return nil, "", err
	}
	if !s.canViewGroupQRCode(ctx, item, actorUserID, isAdmin, inviteToken) {
		return nil, "", ErrCarpoolForbidden
	}
	return s.repo.GetGroupQRCode(ctx, carpoolID)
}

func (s *CarpoolService) canViewGroupQRCode(ctx context.Context, item *Carpool, actorUserID int64, isAdmin bool, inviteToken string) bool {
	if item == nil {
		return false
	}
	if isAdmin || item.MemberRole != nil {
		return true
	}
	if item.OwnerUserID != nil && *item.OwnerUserID == actorUserID {
		return true
	}
	// 未上车者只在车还能上人的时候需要二维码。
	if item.Status != "recruiting" {
		return false
	}
	if item.Visibility == CarpoolVisibilityPublic {
		return true
	}
	inviteToken = strings.TrimSpace(inviteToken)
	if inviteToken == "" {
		return false
	}
	invited, err := s.repo.GetByInvite(ctx, actorUserID, hashInviteToken(inviteToken))
	if err != nil {
		return false
	}
	// 邀请必须指向这辆车——否则任意一张有效邀请都能解锁全部私密车的二维码。
	return invited != nil && invited.ID == item.ID
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
//
// 车已结算（settled_at 非空）时返回结算时冻结的快照；未结算时是实时预览，
// 金额会随用量继续变化——这个区别由 Settled 字段告诉调用方。
func (s *CarpoolService) GetSettlement(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*CarpoolSettlement, error) {
	item, err := s.repo.GetByID(ctx, carpoolID, actorUserID)
	if err != nil {
		return nil, err
	}
	isOwner := item.OwnerUserID != nil && *item.OwnerUserID == actorUserID
	if !isAdmin && !isOwner && item.MemberRole == nil {
		return nil, ErrCarpoolForbidden
	}
	settlement, _, err := s.buildSettlement(ctx, item, actorUserID, isAdmin, isOwner)
	return settlement, err
}

// SettleCarpool 冻结结算单：把当下算出来的每一行金额写进 carpool_members，
// 之后读到的就是这份快照，不再随用量漂移。owner 或 admin 可执行——owner 是
// 实际收退款的人，需要能把自己账本上的数字钉死。
//
// 幂等由 repo 的 `settled_at IS NULL` 守卫保证：重复提交返回 ErrCarpoolAlreadySettled，
// 不会覆盖已冻结的单子。
func (s *CarpoolService) SettleCarpool(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*CarpoolSettlement, error) {
	item, err := s.repo.GetByID(ctx, carpoolID, actorUserID)
	if err != nil {
		return nil, err
	}
	isOwner := item.OwnerUserID != nil && *item.OwnerUserID == actorUserID
	if !isAdmin && !isOwner {
		return nil, ErrCarpoolForbidden
	}
	if item.SettledAt != nil {
		return nil, ErrCarpoolAlreadySettled
	}
	if !carpoolSettleable(item.Status) {
		return nil, ErrCarpoolNotSettleable
	}

	// 全量计算（不受 FullView 裁剪影响）后整车落库。
	_, allMembers, err := s.buildSettlement(ctx, item, actorUserID, isAdmin, true)
	if err != nil {
		return nil, err
	}
	if len(allMembers) == 0 {
		return nil, ErrCarpoolNotSettleable
	}
	if err := s.repo.PersistSettlement(ctx, carpoolID, actorUserID, allMembers); err != nil {
		return nil, err
	}
	return s.GetSettlement(ctx, carpoolID, actorUserID, isAdmin)
}

// UnsettleCarpool 撤销结算（仅 admin）：清空冻结字段，结算单回到实时预览。
// 结算算错了却改不回来的话，账目就只能靠改数据库修——留一个受控出口。
func (s *CarpoolService) UnsettleCarpool(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error {
	if !isAdmin {
		return ErrCarpoolForbidden
	}
	return s.repo.ClearSettlement(ctx, carpoolID, actorUserID)
}

// carpoolSettleable 报告该状态的车能否结算：只有真正发过车的才有账可结。
func carpoolSettleable(status string) bool {
	return status == "active" || status == "ended"
}

// buildSettlement 组装结算单。第二个返回值是**未按可见性裁剪**的全量成员行，
// 供 SettleCarpool 落库使用（结算必须写全车，不能只写调用者看得见的那部分）。
func (s *CarpoolService) buildSettlement(ctx context.Context, item *Carpool, actorUserID int64, isAdmin, isOwner bool) (*CarpoolSettlement, []CarpoolSettlementMember, error) {
	rows, err := s.repo.ListSettlementMembers(ctx, item.ID)
	if err != nil {
		return nil, nil, err
	}

	settlement := &CarpoolSettlement{
		CarpoolID:       item.ID,
		Status:          item.Status,
		WeeklyLimitUSD:  item.WeeklyLimitUSD,
		SeatFeeCNY:      item.SeatFeeCNY,
		UsagePoolCNY:    item.UsagePoolCNY,
		ReserveRatio:    item.ReserveRatio,
		MemberCount:     len(rows),
		FullView:        isAdmin || isOwner,
		Settled:         item.SettledAt != nil,
		SettledAt:       item.SettledAt,
		SettledByUserID: item.SettledByUserID,
		PricingModel:    item.PricingModel,
		RuleNote:        item.RuleNote,
	}
	settlement.CanSettle, settlement.SettleBlockedFor = settleAvailability(item, isAdmin, isOwner, len(rows))

	// 自定义规则车只出用量，不出退补：地板/保底/变动池分摊这套数学是额度预约制
	// 的，套到成员申报恒为 0 的老车上，会凭空算出每人几百块的"补款"。用量本身
	// 仍然给——那正是车主按自己那套规则手工分账最需要的东西。
	if !item.IsQuotaModel() {
		settlement.ManualSettlement = true
		settlement.Members = manualSettlementMembers(rows, actorUserID, settlement.FullView)
		return settlement, nil, nil
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
			Email:                  row.Email,
			Username:               row.Username,
			Role:                   row.Role,
			DeclaredWeeklyQuotaUSD: row.DeclaredWeeklyQuotaUSD,
			PrepaidAmountCNY:       row.PrepaidAmountCNY,
			QuotedPrepaidCNY:       row.QuotedPrepaidCNY,
			ActualUsageUSD:         row.ActualUsageUSD,
			PeriodDays:             periodDays,
		})
	}

	members := ComputeCarpoolSettlementMembers(item.WeeklyLimitUSD, item.SeatFeeCNY, item.UsagePoolCNY, item.ReserveRatio, inputs)
	if settlement.Settled {
		applyFrozenSettlement(members, rows)
	}

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
	return settlement, members, nil
}

// manualSettlementMembers 为自定义规则车生成"只有用量"的结算行：
// 金额字段全部留零，避免前端把它们当成真实退补渲染出来。
func manualSettlementMembers(rows []CarpoolSettlementMemberRow, actorUserID int64, fullView bool) []CarpoolSettlementMember {
	members := make([]CarpoolSettlementMember, 0, len(rows))
	for _, row := range rows {
		if !fullView && row.UserID != actorUserID {
			continue
		}
		members = append(members, CarpoolSettlementMember{
			UserID:         row.UserID,
			Email:          row.Email,
			Username:       row.Username,
			Role:           row.Role,
			ActualUsageUSD: row.ActualUsageUSD,
		})
	}
	return members
}

// settleAvailability 返回调用者此刻能否结算，以及不能的原因（供前端直接提示）。
func settleAvailability(item *Carpool, isAdmin, isOwner bool, memberCount int) (bool, string) {
	switch {
	case !isAdmin && !isOwner:
		return false, "forbidden"
	case !item.IsQuotaModel():
		// 自定义规则车没有可冻结的自动结算结果，人工对账。
		return false, "manual_settlement"
	case item.SettledAt != nil:
		return false, "already_settled"
	case !carpoolSettleable(item.Status):
		return false, "not_launched"
	case memberCount == 0:
		return false, "no_members"
	default:
		return true, ""
	}
}

// applyFrozenSettlement 用冻结快照覆盖实时算出来的金额字段。
// 申报值、邮箱这些不会漂移的展示字段保留实时值（改了邮箱也应该看到新的）。
// 某一行缺快照（结算后手工插入的异常数据）时保留实时值，不伪造历史。
func applyFrozenSettlement(members []CarpoolSettlementMember, rows []CarpoolSettlementMemberRow) {
	frozen := make(map[int64]*CarpoolSettlementFrozenRow, len(rows))
	for _, row := range rows {
		if row.Frozen != nil {
			frozen[row.UserID] = row.Frozen
		}
	}
	for i := range members {
		snapshot, ok := frozen[members[i].UserID]
		if !ok {
			continue
		}
		members[i].FloorUsageUSD = snapshot.FloorUsageUSD
		members[i].ActualUsageUSD = snapshot.ActualUsageUSD
		members[i].BillableUsageUSD = snapshot.BillableUsageUSD
		members[i].FloorTriggered = snapshot.ActualUsageUSD <= snapshot.FloorUsageUSD
		members[i].UsageFinalShareCNY = snapshot.UsageShareCNY
		members[i].UsageDeltaCNY = members[i].UsagePrepaidCNY - snapshot.UsageShareCNY
		members[i].SeatFeeFinalCNY = snapshot.SeatFeeCNY
		members[i].SeatFeeDeltaCNY = members[i].SeatFeePrepaidCNY - snapshot.SeatFeeCNY
		members[i].TotalDeltaCNY = snapshot.TotalDeltaCNY
	}
}

// ReserveInterestSlot 预占一次自定义规则咨询的发送名额（每用户冷却
// CarpoolInterestCooldown）。冷却未过返回 ErrCarpoolInterestTooFrequent——
// 没有这道闸，任何登录用户都能脚本化地向全部 admin 邮箱灌邮件。
// 由 handler 在派发异步发送前同步调用。
func (s *CarpoolService) ReserveInterestSlot(userID int64) error {
	now := time.Now()
	s.interestMu.Lock()
	defer s.interestMu.Unlock()
	if s.interestLastAt == nil {
		s.interestLastAt = make(map[int64]time.Time)
	}
	if last, ok := s.interestLastAt[userID]; ok && now.Sub(last) < CarpoolInterestCooldown {
		return ErrCarpoolInterestTooFrequent
	}
	// 容量兜底：清掉所有已过冷却的条目（它们不再影响判定）。
	if len(s.interestLastAt) >= carpoolInterestTrackerMaxEntries {
		for id, last := range s.interestLastAt {
			if now.Sub(last) >= CarpoolInterestCooldown {
				delete(s.interestLastAt, id)
			}
		}
	}
	s.interestLastAt[userID] = now
	return nil
}

// NotifyCustomRuleInterest 自定义规则咨询入口：给全部 admin 发提示邮件（正文含发起人
// 用户 ID 与邮箱）。SMTP 未配置或发送失败仅记日志优雅降级（日志含发起人信息，便于管理员
// 跟进），接口照常成功；无 admin 或邮件链路未注入时不报错。
//
// 本函数同步发送（逐个 admin 一封），调用方（handler）负责限流与异步派发。
func (s *CarpoolService) NotifyCustomRuleInterest(ctx context.Context, userID int64, note string) {
	initiatorEmail := s.userEmail(ctx, userID)
	logAttrs := []any{"user_id", userID, "user_email", initiatorEmail}
	if s.emailSender == nil {
		slog.Warn("carpool custom rule interest: email pipeline unavailable", logAttrs...)
		return
	}
	displayEmail := initiatorEmail
	if displayEmail == "" {
		displayEmail = "未知"
	}
	subject := "有用户咨询自定义拼车规则"
	body := fmt.Sprintf(
		`<p>用户 #%d（邮箱 %s）希望协商自定义拼车规则（额度池 / 价格 / 保底比例等）。</p>`+
			`<p>请通过邮件或微信联系该用户确认需求，协商一致后为其人工调整车辆参数。</p>`,
		userID, html.EscapeString(displayEmail))
	if note = truncateRunes(strings.TrimSpace(note), CarpoolInterestNoteMaxRunes); note != "" {
		body += fmt.Sprintf(`<p>用户备注：%s</p>`, html.EscapeString(note))
	}
	s.sendCarpoolNotification(ctx, CarpoolAdminEmail, subject, body, logAttrs...)
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
// 绝不影响主流程（优雅降级）。attrs 为失败日志追加的上下文字段（如发起人信息）。
func (s *CarpoolService) sendCarpoolNotification(ctx context.Context, to, subject, body string, attrs ...any) {
	if s.emailSender == nil || strings.TrimSpace(to) == "" {
		return
	}
	if err := s.emailSender.SendEmail(ctx, to, subject, body); err != nil {
		args := append([]any{"subject", subject, "error", err}, attrs...)
		slog.Warn("carpool notification email failed", args...)
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

// carpoolDisplayName 返回可安全嵌入 HTML 邮件正文/标题的车名。
// 车名是车主自由输入的，未转义直接拼进 HTML 等于让车主以平台名义向 admin
// 和全体成员投递任意标记（钓鱼链接、伪造按钮）。
func carpoolDisplayName(name string) string {
	return html.EscapeString(name)
}

// truncateRunes 按字符（非字节）截断，避免把多字节字符切坏。
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// notifyOwnerLaunchBandEntered Σ申报 首次进入发车区间后通知车主登录确认发车。
func (s *CarpoolService) notifyOwnerLaunchBandEntered(ctx context.Context, result *CarpoolMutationResult) {
	if result == nil || !result.LaunchBandEntered || result.Carpool == nil || result.Carpool.OwnerUserID == nil {
		return
	}
	carpool := result.Carpool
	safeName := carpoolDisplayName(carpool.Name)
	body := fmt.Sprintf(
		`<p>你发起的拼车「%s」总申报额度已达到 $%.2f，进入发车区间（$%.2f ~ $%.2f）。</p>`+
			`<p>请尽快登录平台确认发车；确认后管理员将在 24 小时内启动。</p>`,
		safeName, carpool.DeclaredTotalUSD,
		carpool.LaunchMinRatio*carpool.WeeklyLimitUSD, carpool.LaunchMaxRatio*carpool.WeeklyLimitUSD)
	s.sendCarpoolNotification(ctx, s.userEmail(ctx, *carpool.OwnerUserID), "拼车已达发车区间，请登录确认发车", body)
}

// notifyAdminsCarpoolConfirmed 车主确认后通知拼车运营联系人在 24 小时内启动。
// 收件人固定为 CarpoolAdminEmail，不是 users 表里的 role=admin（见该常量注释）。
func (s *CarpoolService) notifyAdminsCarpoolConfirmed(ctx context.Context, carpool *Carpool) {
	if carpool == nil {
		return
	}
	safeName := carpoolDisplayName(carpool.Name)
	body := fmt.Sprintf(
		`<p>拼车「%s」（ID %d）已由车主确认发车，总申报额度 $%.2f。</p>`+
			`<p>请在 24 小时内登录管理后台执行启动。</p>`,
		safeName, carpool.ID, carpool.DeclaredTotalUSD)
	subject := fmt.Sprintf("拼车「%s」已确认，请 24 小时内启动", safeName)
	s.sendCarpoolNotification(ctx, CarpoolAdminEmail, subject, body, "carpool_id", carpool.ID)
}

// notifyMembersCarpoolLaunched 启动成功后逐一通知成员拼车已发车。
func (s *CarpoolService) notifyMembersCarpoolLaunched(ctx context.Context, result *CarpoolMutationResult) {
	if result == nil || result.Carpool == nil || len(result.ActivatedUserIDs) == 0 {
		return
	}
	carpool := result.Carpool
	safeName := carpoolDisplayName(carpool.Name)
	body := fmt.Sprintf(
		`<p>你参加的拼车「%s」已发车，订阅已开通并写入个人周限额。</p>`+
			`<p>请登录平台查看可用额度与结算规则。</p>`, safeName)
	subject := fmt.Sprintf("你参加的拼车「%s」已发车", safeName)
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

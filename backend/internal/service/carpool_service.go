package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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
	Join(ctx context.Context, carpoolID, userID int64, declaredWeeklyQuotaUSD float64, inviteHash *string) (*CarpoolMutationResult, error)
	Launch(ctx context.Context, carpoolID, actorUserID int64, isAdmin, force bool) (*CarpoolMutationResult, error)
	Cancel(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error
	SetJoinLocked(ctx context.Context, carpoolID, actorUserID int64, locked bool) error
	ListSettlementMembers(ctx context.Context, carpoolID int64) ([]CarpoolSettlementMemberRow, error)
	GetRecentWeeklyUsageStats(ctx context.Context, userID int64) (totalUSD float64, daysWithRecords int, err error)
}

type CarpoolService struct {
	repo                CarpoolRepository
	subscriptionService *SubscriptionService
}

func NewCarpoolService(repo CarpoolRepository, subscriptionService *SubscriptionService) *CarpoolService {
	return &CarpoolService{repo: repo, subscriptionService: subscriptionService}
}

func (s *CarpoolService) List(ctx context.Context, userID int64) ([]Carpool, error) {
	items, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].FillDerivedMetrics()
	}
	return items, nil
}

func (s *CarpoolService) ResolveInvite(ctx context.Context, userID int64, token string) (*Carpool, error) {
	item, err := s.repo.GetByInvite(ctx, userID, hashInviteToken(token))
	if err != nil {
		return nil, err
	}
	item.FillDerivedMetrics()
	return item, nil
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
	token, hash, hint, err := newInviteToken()
	if err != nil {
		return nil, fmt.Errorf("generate carpool invite: %w", err)
	}
	result, err := s.repo.Create(ctx, ownerUserID, input, hash, hint)
	if err != nil {
		return nil, err
	}
	result.InviteToken = token
	result.Carpool.FillDerivedMetrics()
	return result, nil
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

func (s *CarpoolService) Join(ctx context.Context, carpoolID, userID int64, declaredWeeklyQuotaUSD float64) (*CarpoolMutationResult, error) {
	if declaredWeeklyQuotaUSD <= 0 || declaredWeeklyQuotaUSD > 1e6 {
		return nil, ErrCarpoolInvalidRequest
	}
	result, err := s.repo.Join(ctx, carpoolID, userID, declaredWeeklyQuotaUSD, nil)
	if err != nil {
		return nil, err
	}
	s.invalidateLaunchedSubscriptions(result)
	return result, nil
}

func (s *CarpoolService) JoinByInvite(ctx context.Context, token string, userID int64, declaredWeeklyQuotaUSD float64) (*CarpoolMutationResult, error) {
	if declaredWeeklyQuotaUSD <= 0 || declaredWeeklyQuotaUSD > 1e6 {
		return nil, ErrCarpoolInvalidRequest
	}
	hash := hashInviteToken(token)
	result, err := s.repo.Join(ctx, 0, userID, declaredWeeklyQuotaUSD, &hash)
	if err != nil {
		return nil, err
	}
	s.invalidateLaunchedSubscriptions(result)
	return result, nil
}

// Launch 手动发车（设计文档 §4.3）：Σ申报进入 [launch_min, launch_max]×周限额 区间，
// 由 owner/admin 触发；force=true 时放宽到 [80%, launch_max]（降档发车，公共池变大）。
func (s *CarpoolService) Launch(ctx context.Context, carpoolID, actorUserID int64, isAdmin, force bool) (*CarpoolMutationResult, error) {
	result, err := s.repo.Launch(ctx, carpoolID, actorUserID, isAdmin, force)
	if err != nil {
		return nil, err
	}
	s.invalidateLaunchedSubscriptions(result)
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

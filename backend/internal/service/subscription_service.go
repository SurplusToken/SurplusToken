package service

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/dgraph-io/ristretto"
	"golang.org/x/sync/singleflight"
)

// MaxExpiresAt is the maximum allowed expiration date (year 2099)
// This prevents time.Time JSON serialization errors (RFC 3339 requires year <= 9999)
var MaxExpiresAt = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)

// MaxValidityDays is the maximum allowed validity days for subscriptions (100 years)
const MaxValidityDays = 36500

var (
	ErrSubscriptionNotFound        = infraerrors.NotFound("SUBSCRIPTION_NOT_FOUND", "subscription not found")
	ErrSubscriptionExpired         = infraerrors.Forbidden("SUBSCRIPTION_EXPIRED", "subscription has expired")
	ErrSubscriptionSuspended       = infraerrors.Forbidden("SUBSCRIPTION_SUSPENDED", "subscription is suspended")
	ErrSubscriptionAlreadyExists   = infraerrors.Conflict("SUBSCRIPTION_ALREADY_EXISTS", "subscription already exists for this user and group")
	ErrSubscriptionAssignConflict  = infraerrors.Conflict("SUBSCRIPTION_ASSIGN_CONFLICT", "subscription exists but request conflicts with existing assignment semantics")
	ErrSubscriptionNotRevoked      = infraerrors.Conflict("SUBSCRIPTION_NOT_REVOKED", "subscription is not revoked")
	ErrSubscriptionRestoreConflict = infraerrors.Conflict("SUBSCRIPTION_RESTORE_CONFLICT", "subscription already exists for this user and group")
	ErrGroupNotSubscriptionType    = infraerrors.BadRequest("GROUP_NOT_SUBSCRIPTION_TYPE", "group is not a subscription type")
	ErrInvalidInput                = infraerrors.BadRequest("INVALID_INPUT", "at least one of resetDaily, resetWeekly, or resetMonthly must be true")
	ErrDailyLimitExceeded          = infraerrors.TooManyRequests("DAILY_LIMIT_EXCEEDED", "daily usage limit exceeded")
	ErrWeeklyLimitExceeded         = infraerrors.TooManyRequests("WEEKLY_LIMIT_EXCEEDED", "weekly usage limit exceeded")
	ErrMonthlyLimitExceeded        = infraerrors.TooManyRequests("MONTHLY_LIMIT_EXCEEDED", "monthly usage limit exceeded")
	ErrSubscriptionNilInput        = infraerrors.BadRequest("SUBSCRIPTION_NIL_INPUT", "subscription input cannot be nil")
	ErrAdjustWouldExpire           = infraerrors.BadRequest("ADJUST_WOULD_EXPIRE", "adjustment would result in expired subscription (remaining days must be > 0)")
)

// SubscriptionService 订阅服务
type SubscriptionService struct {
	groupRepo           GroupRepository
	userSubRepo         UserSubscriptionRepository
	billingCacheService *BillingCacheService
	entClient           *dbent.Client

	// L1 缓存：加速中间件热路径的订阅查询
	subCacheL1     *ristretto.Cache
	subCacheGroup  singleflight.Group
	subCacheTTL    time.Duration
	subCacheJitter int // 抖动百分比

	maintenanceQueue *SubscriptionMaintenanceQueue
	now              func() time.Time

	// upstreamWindows 提供拼车组绑定账号的上游周窗口（可选注入）。
	// 未注入时拼车周窗口退回本地 7 天网格，行为与注入前一致。
	upstreamWindows CarpoolUpstreamWindowSource

	// cycleRecorder 落库已关闭的拼车计费周期（可选注入）。
	// 未注入时不记台账，结算退回按月整体算地板。
	cycleRecorder CarpoolBillingCycleRecorder
}

// SetCarpoolBillingCycleRecorder 注入计费周期台账（wire 组装时调用）。
func (s *SubscriptionService) SetCarpoolBillingCycleRecorder(rec CarpoolBillingCycleRecorder) {
	if s == nil {
		return
	}
	s.cycleRecorder = rec
}

// recordClosedCarpoolCycle 在周窗口推进前，把即将被清零的那一周落成台账。
//
// 这是唯一能拿到"这一周实际用了多少"的时刻——weekly_usage_usd 下一行就被置零。
// 不落库，月末就算不出 Σ max(周实际, 周保底)，只能退回按月整体算地板，而那会让
// 超用的周补贴没用满的周（见 migration 193 的说明）。
//
// 失败只记日志不阻断：窗口推进关系到用户能不能继续用，不能因为台账写失败就卡住。
// 漏记的周期在结算时会被识别（周期数对不上覆盖天数），单据上会标出来。
func (s *SubscriptionService) recordClosedCarpoolCycle(ctx context.Context, sub *UserSubscription, cycleEnd time.Time) {
	if s == nil || sub == nil {
		return
	}
	if !sub.HasWeeklyReserve() || sub.WeeklyWindowStart == nil {
		return // 非拼车订阅不记台账
	}
	if s.cycleRecorder == nil {
		// 这一周的流水就此丢失且无法补记（用量马上会被清零），必须报出来。
		warnMissingCycleRecorder()
		return
	}
	// 只传订阅 ID：用量等数值由仓储层直接读订阅行，绝不用这里的内存快照——
	// ValidateAndCheckLimits 已经把 sub.WeeklyUsageUSD 在内存里清零了。
	if err := s.cycleRecorder.RecordCycle(ctx, sub.ID, cycleEnd); err != nil {
		log.Printf("[subscription] ALERT: record carpool billing cycle failed sub=%d start=%s: %v",
			sub.ID, sub.WeeklyWindowStart.Format(time.RFC3339), err)
	}
}

// CarpoolWiringReady 报告拼车相关的可选注入是否齐备。
//
// 这两个注入曾经在一次 wire 重新生成中被整块抹掉，而缺失是完全静默的：
// 上游窗口缺失时拼车的"一周"永远不跟随 OpenAI 重置（成员被自己人挡在门外），
// 台账记录器缺失时每周期的计费流水根本不落库（月底按 80% 地板结算直接失真）。
// 两者都不会报错、日志里也看不出来，只能靠这里主动暴露。
func (s *SubscriptionService) CarpoolWiringReady() (upstreamWindows bool, cycleRecorder bool) {
	if s == nil {
		return false, false
	}
	return s.upstreamWindows != nil, s.cycleRecorder != nil
}

// 惰性告警：只在真的碰到拼车订阅、却发现注入缺失时打一次。
//
// 刻意不放在启动流程里——启动处的调用点在 wire 生成文件中，正是上次被整块
// 抹掉的地方；放在服务自己身上则与装配方式无关，怎么组装都躲不掉。
var (
	warnMissingUpstreamWindowsOnce sync.Once
	warnMissingCycleRecorderOnce   sync.Once
)

func warnMissingUpstreamWindows() {
	warnMissingUpstreamWindowsOnce.Do(func() {
		log.Printf("[subscription] ALERT: carpool upstream window source is NOT wired; " +
			"carpool weekly windows stay on the local 7-day grid and will NOT follow the " +
			"upstream reset — after upstream resets, members get blocked by our own counter")
	})
}

func warnMissingCycleRecorder() {
	warnMissingCycleRecorderOnce.Do(func() {
		log.Printf("[subscription] ALERT: carpool billing cycle recorder is NOT wired; " +
			"closed weekly cycles are NOT persisted and month-end settlement loses the " +
			"per-cycle 80%% floor")
	})
}

// SetCarpoolUpstreamWindowSource 注入上游周窗口查询器（wire 组装时调用）。
func (s *SubscriptionService) SetCarpoolUpstreamWindowSource(src CarpoolUpstreamWindowSource) {
	if s == nil {
		return
	}
	s.upstreamWindows = src
}

// carpoolWeeklyWindowTarget 返回拼车订阅此刻应处的周窗口起点，以及是否采信了上游。
// 上游不可用/陈旧/查询失败时退回本地 7 天网格（降级后全车依然一致）。
func (s *SubscriptionService) carpoolWeeklyWindowTarget(ctx context.Context, sub *UserSubscription) (time.Time, bool) {
	if sub == nil || sub.WeeklyWindowStart == nil {
		return time.Time{}, false
	}
	var upstream *CarpoolUpstreamWindow
	if s.upstreamWindows == nil {
		// 走到这里说明这是一张拼车订阅，却没有上游窗口可查——降级是真实发生的，
		// 必须在日志里留下痕迹（见 warnMissingUpstreamWindows 的注释）。
		warnMissingUpstreamWindows()
	}
	if s.upstreamWindows != nil {
		w, err := s.upstreamWindows.GroupUpstreamWeeklyWindow(ctx, sub.GroupID)
		if err != nil {
			log.Printf("[subscription] ALERT: read upstream carpool window failed group=%d: %v (falling back to local grid)", sub.GroupID, err)
		} else {
			upstream = w
		}
	}
	return CarpoolWeeklyWindowTarget(*sub.WeeklyWindowStart, upstream, time.Now())
}

// resetCarpoolGroupWindow 尝试把整辆车一次性重锚到 target。
//
// 返回 true 表示整组重锚已经落库（含台账与清零），调用方无需再走单订阅路径。
// 返回 false 有两种情形，都退回原路径：
//   - 上游窗口源没有实现整组重锚（未注入/旧实现）；
//   - 本次无需写入（并发下已有人重锚过，或 target 并未前移）。
//
// 失败只记日志不阻塞：窗口重置是请求路径上的旁路维护，让它挡住用户请求
// 是本末倒置——退回单订阅路径至少还能把本人重置掉。
// carpoolGroupResetOutcome 区分整组重锚的三种结局。
//
// 把"拒绝"和"不支持"分开是必须的：前者是整组重锚看过全组状态后的决定，
// 调用方必须尊重；后者只是能力缺失，才可以退回单订阅路径。早先两者都返回
// false，于是单订阅路径去做了整组重锚刚刚拒绝的事——生产上因此写出了一批
// cycle_end < cycle_start 的倒挂周期。
type carpoolGroupResetOutcome int

const (
	carpoolGroupResetUnavailable carpoolGroupResetOutcome = iota // 未注入/旧实现/出错
	carpoolGroupResetApplied                                     // 已落库
	carpoolGroupResetDeclined                                    // 看过全组后判定不该写
)

func (s *SubscriptionService) resetCarpoolGroupWindow(ctx context.Context, sub *UserSubscription, target time.Time) carpoolGroupResetOutcome {
	if s == nil || sub == nil || s.upstreamWindows == nil {
		return carpoolGroupResetUnavailable
	}
	resetter, ok := s.upstreamWindows.(CarpoolGroupWindowResetter)
	if !ok {
		return carpoolGroupResetUnavailable
	}
	result, err := resetter.ResetGroupWeeklyWindow(ctx, sub.GroupID, target, CarpoolUpstreamWindowMinAdvance)
	if err != nil {
		log.Printf("[subscription] ALERT: carpool group window reset failed group=%d target=%s: %v (falling back to per-subscription reset)",
			sub.GroupID, target.Format(time.RFC3339), err)
		return carpoolGroupResetUnavailable
	}
	if result == nil {
		return carpoolGroupResetUnavailable
	}
	if !result.Applied {
		return carpoolGroupResetDeclined
	}
	log.Printf("[subscription] carpool group window reanchored group=%d %s -> %s members=%d cycles=%d",
		sub.GroupID, result.From.Format(time.RFC3339), result.To.Format(time.RFC3339),
		len(result.UserIDs), result.Cycles)
	// 整组重锚必须整组失效缓存，否则别人还拿着旧窗口和旧用量。
	for _, userID := range result.UserIDs {
		s.InvalidateSubCache(userID, sub.GroupID)
		if s.billingCacheService != nil {
			_ = s.billingCacheService.InvalidateSubscription(ctx, userID, sub.GroupID)
		}
	}
	return carpoolGroupResetApplied
}

// carpoolWeeklyWindowDrifted 报告拼车订阅的周窗口是否已经偏离目标窗口。
//
// 这是"上游在我们的 7 天到点之前/之后重置了"的检测点：只看时间差是发现不了的，
// 必须拿上游那条外部事实来比。
func (s *SubscriptionService) carpoolWeeklyWindowDrifted(ctx context.Context, sub *UserSubscription) bool {
	if sub == nil || !sub.HasWeeklyReserve() || sub.WeeklyWindowStart == nil {
		return false
	}
	target, fromUpstream := s.carpoolWeeklyWindowTarget(ctx, sub)
	if !fromUpstream {
		return false
	}
	return CarpoolWeeklyWindowDrifted(*sub.WeeklyWindowStart, target)
}

// NewSubscriptionService 创建订阅服务
func NewSubscriptionService(groupRepo GroupRepository, userSubRepo UserSubscriptionRepository, billingCacheService *BillingCacheService, entClient *dbent.Client, cfg *config.Config) *SubscriptionService {
	svc := &SubscriptionService{
		groupRepo:           groupRepo,
		userSubRepo:         userSubRepo,
		billingCacheService: billingCacheService,
		entClient:           entClient,
		now:                 time.Now,
	}
	svc.initSubCache(cfg)
	svc.initMaintenanceQueue(cfg)
	svc.StartSubCacheInvalidationSubscriber(context.Background())
	return svc
}

func (s *SubscriptionService) initMaintenanceQueue(cfg *config.Config) {
	if cfg == nil {
		return
	}
	mc := cfg.SubscriptionMaintenance
	if mc.WorkerCount <= 0 || mc.QueueSize <= 0 {
		return
	}
	s.maintenanceQueue = NewSubscriptionMaintenanceQueue(mc.WorkerCount, mc.QueueSize)
}

// Stop stops the maintenance worker pool.
func (s *SubscriptionService) Stop() {
	if s == nil {
		return
	}
	if s.maintenanceQueue != nil {
		s.maintenanceQueue.Stop()
	}
}

// initSubCache 初始化订阅 L1 缓存
func (s *SubscriptionService) initSubCache(cfg *config.Config) {
	if cfg == nil {
		return
	}
	sc := cfg.SubscriptionCache
	if sc.L1Size <= 0 || sc.L1TTLSeconds <= 0 {
		return
	}
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: int64(sc.L1Size) * 10,
		MaxCost:     int64(sc.L1Size),
		BufferItems: 64,
	})
	if err != nil {
		log.Printf("Warning: failed to init subscription L1 cache: %v", err)
		return
	}
	s.subCacheL1 = cache
	s.subCacheTTL = time.Duration(sc.L1TTLSeconds) * time.Second
	s.subCacheJitter = sc.JitterPercent
}

// subCacheKey 生成订阅缓存 key（热路径，避免 fmt.Sprintf 开销）
func subCacheKey(userID, groupID int64) string {
	return "sub:" + strconv.FormatInt(userID, 10) + ":" + strconv.FormatInt(groupID, 10)
}

// jitteredTTL 为 TTL 添加抖动，避免集中过期
func (s *SubscriptionService) jitteredTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 || s.subCacheJitter <= 0 {
		return ttl
	}
	pct := s.subCacheJitter
	if pct > 100 {
		pct = 100
	}
	delta := float64(pct) / 100
	factor := 1 - delta + rand.Float64()*(2*delta)
	if factor <= 0 {
		return ttl
	}
	return time.Duration(float64(ttl) * factor)
}

// InvalidateSubCache 失效指定用户+分组的订阅 L1 缓存
func (s *SubscriptionService) InvalidateSubCache(userID, groupID int64) {
	if s.subCacheL1 == nil {
		return
	}
	s.subCacheL1.Del(subCacheKey(userID, groupID))
}

// InvalidateSubCacheSync 失效订阅 L1 缓存并等待 Ristretto 删除操作生效。
func (s *SubscriptionService) InvalidateSubCacheSync(userID, groupID int64) {
	s.invalidateSubCacheKeySync(subCacheKey(userID, groupID))
}

func (s *SubscriptionService) invalidateSubCacheKeySync(key string) {
	if s.subCacheL1 == nil {
		return
	}
	s.subCacheL1.Del(key)
	s.subCacheL1.Wait()
}

// StartSubCacheInvalidationSubscriber 启动跨实例订阅 L1 缓存失效订阅。
func (s *SubscriptionService) StartSubCacheInvalidationSubscriber(ctx context.Context) {
	if s.billingCacheService == nil || s.subCacheL1 == nil {
		return
	}
	if err := s.billingCacheService.SubscribeSubscriptionCacheInvalidation(ctx, func(cacheKey string) {
		s.invalidateSubCacheKeySync(cacheKey)
	}); err != nil {
		log.Printf("Warning: failed to start subscription cache invalidation subscriber: %v", err)
	}
}

func (s *SubscriptionService) invalidateSubscriptionCaches(userID, groupID int64) error {
	s.InvalidateSubCacheSync(userID, groupID)
	if s.billingCacheService == nil {
		return nil
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.billingCacheService.InvalidateSubscription(cacheCtx, userID, groupID); err != nil {
		return fmt.Errorf("invalidate billing subscription cache: %w", err)
	}
	if err := s.billingCacheService.PublishSubscriptionCacheInvalidation(cacheCtx, subCacheKey(userID, groupID)); err != nil {
		return fmt.Errorf("publish subscription cache invalidation: %w", err)
	}
	return nil
}

// AssignSubscriptionInput 分配订阅输入
type AssignSubscriptionInput struct {
	UserID       int64
	GroupID      int64
	ValidityDays int
	AssignedBy   int64
	Notes        string
	// WeeklyLimitUSD 是可选的订阅级周限额覆盖（拼车手动车代加成员用）；
	// nil 表示未设置，限额检查回退到分组级 group.WeeklyLimitUSD。
	WeeklyLimitUSD *float64
	// WeeklyWindowStart 是可选的周窗口锚点（拼车手动车对齐当日 UTC 零点）；
	// nil 表示不锚定，首次窗口检查时按既有逻辑自动初始化。
	WeeklyWindowStart *time.Time
}

// AssignSubscription 分配订阅给用户（不允许重复分配）
func (s *SubscriptionService) AssignSubscription(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, error) {
	sub, _, err := s.assignSubscriptionWithReuse(ctx, input)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// AssignOrExtendSubscription 分配或续期订阅（用于兑换码等场景）
// 如果用户已有同分组的订阅：
//   - 未过期：从当前过期时间累加天数
//   - 已过期：从当前时间开始计算新的过期时间，并激活订阅
//
// 如果没有订阅：创建新订阅
func (s *SubscriptionService) AssignOrExtendSubscription(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, bool, error) {
	return s.assignOrExtendSubscription(ctx, input, false)
}

func (s *SubscriptionService) assignOrExtendSubscription(ctx context.Context, input *AssignSubscriptionInput, deferCacheInvalidation bool) (*UserSubscription, bool, error) {
	// 检查分组是否存在且为订阅类型
	group, err := s.groupRepo.GetByID(ctx, input.GroupID)
	if err != nil {
		return nil, false, fmt.Errorf("group not found: %w", err)
	}
	if !group.IsSubscriptionType() {
		return nil, false, ErrGroupNotSubscriptionType
	}

	// 查询是否已有订阅
	existingSub, err := s.userSubRepo.GetByUserIDAndGroupID(ctx, input.UserID, input.GroupID)
	if err != nil {
		// 不存在记录是正常情况，其他错误需要返回
		existingSub = nil
	}

	validityDays := input.ValidityDays
	if validityDays <= 0 {
		validityDays = 30
	}
	if validityDays > MaxValidityDays {
		validityDays = MaxValidityDays
	}

	// 已有订阅，执行续期（在事务中完成所有更新）
	if existingSub != nil {
		if err := s.updateExistingSubscriptionTerm(ctx, existingSub.ID, validityDays, input.Notes, false); err != nil {
			return nil, false, err
		}

		// 失效订阅缓存
		s.maybeInvalidateAssignmentCaches(input.UserID, input.GroupID, deferCacheInvalidation)

		// 返回更新后的订阅
		sub, err := s.userSubRepo.GetByID(ctx, existingSub.ID)
		return sub, true, err // true 表示是续期
	}

	// 没有订阅，创建新订阅
	sub, err := s.createSubscription(ctx, input)
	if err != nil {
		return nil, false, err
	}

	// 失效订阅缓存
	s.maybeInvalidateAssignmentCaches(input.UserID, input.GroupID, deferCacheInvalidation)

	return sub, false, nil // false 表示是新建
}

func (s *SubscriptionService) maybeInvalidateAssignmentCaches(userID, groupID int64, deferred bool) {
	// Payment fulfillment owns an outer transaction and performs a synchronous
	// invalidation after commit. Invalidating inside that transaction can reload
	// the pre-commit subscription into cache.
	if deferred {
		return
	}

	s.InvalidateSubCache(userID, groupID)
	if s.billingCacheService != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.billingCacheService.InvalidateSubscription(cacheCtx, userID, groupID)
		}()
	}
}

func (s *SubscriptionService) updateExistingSubscriptionTerm(
	ctx context.Context,
	subscriptionID int64,
	validityDays int,
	notes string,
	assignmentSemantics bool,
) error {
	return s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		existingSub, err := s.userSubRepo.GetByIDForUpdate(txCtx, subscriptionID)
		if err != nil {
			return fmt.Errorf("lock subscription for renewal: %w", err)
		}
		if assignmentSemantics && existingSub.Status == SubscriptionStatusSuspended {
			return nil
		}

		now := time.Now()
		if s.now != nil {
			now = s.now()
		}
		isExpired := !existingSub.ExpiresAt.After(now)
		if assignmentSemantics {
			isExpired = existingSub.Status == SubscriptionStatusExpired ||
				(existingSub.Status != SubscriptionStatusSuspended && !existingSub.ExpiresAt.After(now))
		}
		newExpiresAt := existingSub.ExpiresAt.AddDate(0, 0, validityDays)
		if isExpired {
			newExpiresAt = now.AddDate(0, 0, validityDays)
		}
		if newExpiresAt.After(MaxExpiresAt) {
			newExpiresAt = MaxExpiresAt
		}
		if assignmentSemantics && strings.TrimSpace(existingSub.Notes) == strings.TrimSpace(notes) {
			notes = ""
		}

		if isExpired {
			renewed := renewedSubscriptionTerm(existingSub, notes, now, newExpiresAt)
			if err := s.userSubRepo.Update(txCtx, renewed); err != nil {
				return fmt.Errorf("renew expired subscription: %w", err)
			}
			return nil
		}

		// 更新过期时间
		if err := s.userSubRepo.ExtendExpiry(txCtx, existingSub.ID, newExpiresAt); err != nil {
			return fmt.Errorf("extend subscription: %w", err)
		}

		// 如果订阅被暂停，恢复为 active 状态
		if existingSub.Status != SubscriptionStatusActive {
			if err := s.userSubRepo.UpdateStatus(txCtx, existingSub.ID, SubscriptionStatusActive); err != nil {
				return fmt.Errorf("update subscription status: %w", err)
			}
		}

		// 追加备注
		if notes != "" {
			if err := s.userSubRepo.UpdateNotes(txCtx, existingSub.ID, appendSubscriptionNotes(existingSub.Notes, notes)); err != nil {
				return fmt.Errorf("update subscription notes: %w", err)
			}
		}

		return nil
	})
}

func (s *SubscriptionService) withSubscriptionUpdateTx(ctx context.Context, fn func(context.Context) error) error {
	if dbent.TxFromContext(ctx) != nil {
		return fn(ctx)
	}
	if s.entClient == nil {
		return fn(ctx)
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func renewedSubscriptionTerm(existingSub *UserSubscription, notes string, startsAt, expiresAt time.Time) *UserSubscription {
	renewed := *existingSub
	// 日窗口按日历日对齐（0 点刷新）；周/月窗口按订阅期限对齐（锚点为新周期起点）。
	dailyWindowStart := timezone.StartOfDay(startsAt)
	periodicWindowStart := startsAt
	renewed.StartsAt = startsAt
	renewed.ExpiresAt = expiresAt
	renewed.Status = SubscriptionStatusActive
	renewed.DailyWindowStart = &dailyWindowStart
	renewed.WeeklyWindowStart = &periodicWindowStart
	renewed.MonthlyWindowStart = &periodicWindowStart
	renewed.DailyUsageUSD = 0
	renewed.WeeklyUsageUSD = 0
	renewed.MonthlyUsageUSD = 0
	renewed.Notes = appendSubscriptionNotes(existingSub.Notes, notes)
	return &renewed
}

func appendSubscriptionNotes(existingNotes, newNotes string) string {
	if newNotes == "" {
		return existingNotes
	}
	if existingNotes == "" {
		return newNotes
	}
	return existingNotes + "\n" + newNotes
}

// createSubscription 创建新订阅（内部方法）
// subscriptionHistoryLookup 是可选能力：查出该 (user, group) 最近一条订阅（含已撤销的）。
// 做成可选接口而不是塞进 UserSubscriptionRepository，是因为后者被大量测试桩实现，
// 而这里只需要生产仓储具备该能力；不具备时继承逻辑安全地退化为不做任何事。
type subscriptionHistoryLookup interface {
	GetLatestIncludingDeletedByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error)
}

// inheritQuotaOverrides 从同一 (user, group) 最近一条订阅（含已撤销的）继承订阅级
// 限额覆盖字段。
//
// 撤销订阅是软删除，而 ExistsByUserIDAndGroupID 看不到软删行，所以"撤销 + 重新分配"
// 会走 Create 新建一行。对拼车成员来说，这一行没有 weekly_reserved_usd 就意味着：
// 保底额度消失、用量不再计入组级公共池计数器，而个人上限回落到分组级的整车周限额
// ——一个人可以独占全车额度，且对公共池完全隐形。继承是就地可做、不跨层的修法。
// 查询失败或没有历史行时保持原样（新订阅按分组级限额），不阻塞分配。
func (s *SubscriptionService) inheritQuotaOverrides(ctx context.Context, sub *UserSubscription) {
	if sub == nil || s.userSubRepo == nil {
		return
	}
	lookup, ok := s.userSubRepo.(subscriptionHistoryLookup)
	if !ok {
		return
	}
	previous, err := lookup.GetLatestIncludingDeletedByUserIDAndGroupID(ctx, sub.UserID, sub.GroupID)
	if err != nil || previous == nil {
		return
	}
	if sub.WeeklyLimitUSD == nil && previous.WeeklyLimitUSD != nil {
		limit := *previous.WeeklyLimitUSD
		sub.WeeklyLimitUSD = &limit
	}
	if sub.WeeklyReservedUSD == nil && previous.WeeklyReservedUSD != nil {
		reserved := *previous.WeeklyReservedUSD
		sub.WeeklyReservedUSD = &reserved
	}
}

func (s *SubscriptionService) createSubscription(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, error) {
	validityDays := input.ValidityDays
	if validityDays <= 0 {
		validityDays = 30
	}
	if validityDays > MaxValidityDays {
		validityDays = MaxValidityDays
	}

	now := time.Now()
	expiresAt := now.AddDate(0, 0, validityDays)
	if expiresAt.After(MaxExpiresAt) {
		expiresAt = MaxExpiresAt
	}

	sub := &UserSubscription{
		UserID:     input.UserID,
		GroupID:    input.GroupID,
		StartsAt:   now,
		ExpiresAt:  expiresAt,
		Status:     SubscriptionStatusActive,
		AssignedAt: now,
		Notes:      input.Notes,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	// 可选的订阅级周限额覆盖与周窗口锚点（拼车手动车代加成员）；先于此设置，
	// inheritQuotaOverrides 只填 nil 字段，不会覆盖显式传入的值。
	sub.WeeklyLimitUSD = input.WeeklyLimitUSD
	sub.WeeklyWindowStart = input.WeeklyWindowStart
	// 只有当 AssignedBy > 0 时才设置（0 表示系统分配，如兑换码）
	if input.AssignedBy > 0 {
		sub.AssignedBy = &input.AssignedBy
	}
	s.inheritQuotaOverrides(ctx, sub)

	if err := s.userSubRepo.Create(ctx, sub); err != nil {
		return nil, err
	}

	// 重新获取完整订阅信息（包含关联）
	return s.userSubRepo.GetByID(ctx, sub.ID)
}

// BulkAssignSubscriptionInput 批量分配订阅输入
type BulkAssignSubscriptionInput struct {
	UserIDs      []int64
	GroupID      int64
	ValidityDays int
	AssignedBy   int64
	Notes        string
}

// BulkAssignResult 批量分配结果
type BulkAssignResult struct {
	SuccessCount  int
	CreatedCount  int
	ReusedCount   int
	FailedCount   int
	Subscriptions []UserSubscription
	Errors        []string
	Statuses      map[int64]string
}

// BulkAssignSubscription 批量分配订阅
func (s *SubscriptionService) BulkAssignSubscription(ctx context.Context, input *BulkAssignSubscriptionInput) (*BulkAssignResult, error) {
	result := &BulkAssignResult{
		Subscriptions: make([]UserSubscription, 0),
		Errors:        make([]string, 0),
		Statuses:      make(map[int64]string),
	}

	for _, userID := range input.UserIDs {
		sub, reused, err := s.assignSubscriptionWithReuse(ctx, &AssignSubscriptionInput{
			UserID:       userID,
			GroupID:      input.GroupID,
			ValidityDays: input.ValidityDays,
			AssignedBy:   input.AssignedBy,
			Notes:        input.Notes,
		})
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Sprintf("user %d: %v", userID, err))
			result.Statuses[userID] = "failed"
		} else {
			result.SuccessCount++
			result.Subscriptions = append(result.Subscriptions, *sub)
			if reused {
				result.ReusedCount++
				result.Statuses[userID] = "reused"
			} else {
				result.CreatedCount++
				result.Statuses[userID] = "created"
			}
		}
	}

	return result, nil
}

func (s *SubscriptionService) assignSubscriptionWithReuse(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, bool, error) {
	// 检查分组是否存在且为订阅类型
	group, err := s.groupRepo.GetByID(ctx, input.GroupID)
	if err != nil {
		return nil, false, fmt.Errorf("group not found: %w", err)
	}
	if !group.IsSubscriptionType() {
		return nil, false, ErrGroupNotSubscriptionType
	}

	// 检查是否已存在订阅；若已存在，则按幂等成功返回现有订阅
	exists, err := s.userSubRepo.ExistsByUserIDAndGroupID(ctx, input.UserID, input.GroupID)
	if err != nil {
		return nil, false, err
	}
	if exists {
		sub, getErr := s.userSubRepo.GetByUserIDAndGroupID(ctx, input.UserID, input.GroupID)
		if getErr != nil {
			return nil, false, getErr
		}
		now := time.Now()
		if sub.Status == SubscriptionStatusExpired ||
			(sub.Status != SubscriptionStatusSuspended && !sub.ExpiresAt.After(now)) {
			validityDays := normalizeAssignValidityDays(input.ValidityDays)
			if err := s.updateExistingSubscriptionTerm(ctx, sub.ID, validityDays, input.Notes, true); err != nil {
				return nil, false, err
			}
			s.maybeInvalidateAssignmentCaches(input.UserID, input.GroupID, false)
			renewed, getErr := s.userSubRepo.GetByID(ctx, sub.ID)
			return renewed, true, getErr
		}
		if conflictReason, conflict := detectAssignSemanticConflict(sub, input); conflict {
			return nil, false, ErrSubscriptionAssignConflict.WithMetadata(map[string]string{
				"conflict_reason": conflictReason,
			})
		}
		return sub, true, nil
	}

	sub, err := s.createSubscription(ctx, input)
	if err != nil {
		return nil, false, err
	}

	// 失效订阅缓存
	s.InvalidateSubCache(input.UserID, input.GroupID)
	if s.billingCacheService != nil {
		userID, groupID := input.UserID, input.GroupID
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.billingCacheService.InvalidateSubscription(cacheCtx, userID, groupID)
		}()
	}

	return sub, false, nil
}

func detectAssignSemanticConflict(existing *UserSubscription, input *AssignSubscriptionInput) (string, bool) {
	if existing == nil || input == nil {
		return "", false
	}

	normalizedDays := normalizeAssignValidityDays(input.ValidityDays)
	if !existing.StartsAt.IsZero() {
		expectedExpiresAt := existing.StartsAt.AddDate(0, 0, normalizedDays)
		if expectedExpiresAt.After(MaxExpiresAt) {
			expectedExpiresAt = MaxExpiresAt
		}
		if !existing.ExpiresAt.Equal(expectedExpiresAt) {
			return "validity_days_mismatch", true
		}
	}

	existingNotes := strings.TrimSpace(existing.Notes)
	inputNotes := strings.TrimSpace(input.Notes)
	if existingNotes != inputNotes {
		return "notes_mismatch", true
	}

	return "", false
}

func normalizeAssignValidityDays(days int) int {
	if days <= 0 {
		days = 30
	}
	if days > MaxValidityDays {
		days = MaxValidityDays
	}
	return days
}

// RevokeSubscription 撤销订阅
func (s *SubscriptionService) RevokeSubscription(ctx context.Context, subscriptionID int64) error {
	// 先获取订阅信息用于失效缓存
	sub, err := s.userSubRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return err
	}

	if err := s.userSubRepo.Delete(ctx, subscriptionID); err != nil {
		return err
	}

	if err := s.invalidateSubscriptionCaches(sub.UserID, sub.GroupID); err != nil {
		return err
	}

	return nil
}

// RestoreSubscription 恢复已撤销订阅
func (s *SubscriptionService) RestoreSubscription(ctx context.Context, subscriptionID int64) (*UserSubscription, error) {
	sub, err := s.userSubRepo.GetByIDIncludeDeleted(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if sub.DeletedAt == nil {
		return nil, ErrSubscriptionNotRevoked
	}

	exists, err := s.userSubRepo.ExistsActiveByUserIDAndGroupID(ctx, sub.UserID, sub.GroupID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrSubscriptionRestoreConflict
	}

	restoredStatus := sub.Status
	now := time.Now()
	if restoredStatus == SubscriptionStatusActive && !sub.ExpiresAt.After(now) {
		restoredStatus = SubscriptionStatusExpired
	}

	restored, err := s.userSubRepo.Restore(ctx, subscriptionID, restoredStatus)
	if err != nil {
		return nil, err
	}

	if err := s.invalidateSubscriptionCaches(restored.UserID, restored.GroupID); err != nil {
		return nil, err
	}
	return restored, nil
}

// ExtendSubscription 调整订阅时长（正数延长，负数缩短）
func (s *SubscriptionService) ExtendSubscription(ctx context.Context, subscriptionID int64, days int) (*UserSubscription, error) {
	sub, err := s.userSubRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, ErrSubscriptionNotFound
	}

	// 限制调整天数范围
	if days > MaxValidityDays {
		days = MaxValidityDays
	}
	if days < -MaxValidityDays {
		days = -MaxValidityDays
	}

	now := time.Now()
	isExpired := !sub.ExpiresAt.After(now)

	// 如果订阅已过期，不允许负向调整
	if isExpired && days < 0 {
		return nil, infraerrors.BadRequest("CANNOT_SHORTEN_EXPIRED", "cannot shorten an expired subscription")
	}

	// 计算新的过期时间
	var newExpiresAt time.Time
	if isExpired {
		// 已过期：从当前时间开始增加天数
		newExpiresAt = now.AddDate(0, 0, days)
	} else {
		// 未过期：从原过期时间增加/减少天数
		newExpiresAt = sub.ExpiresAt.AddDate(0, 0, days)
	}

	if newExpiresAt.After(MaxExpiresAt) {
		newExpiresAt = MaxExpiresAt
	}

	// 检查新的过期时间必须大于当前时间
	if !newExpiresAt.After(now) {
		return nil, ErrAdjustWouldExpire
	}

	if err := s.userSubRepo.ExtendExpiry(ctx, subscriptionID, newExpiresAt); err != nil {
		return nil, err
	}

	// 如果订阅已过期，恢复为active状态
	if sub.Status == SubscriptionStatusExpired {
		if err := s.userSubRepo.UpdateStatus(ctx, subscriptionID, SubscriptionStatusActive); err != nil {
			return nil, err
		}
	}

	// 失效订阅缓存
	s.InvalidateSubCache(sub.UserID, sub.GroupID)
	if s.billingCacheService != nil {
		userID, groupID := sub.UserID, sub.GroupID
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.billingCacheService.InvalidateSubscription(cacheCtx, userID, groupID)
		}()
	}

	return s.userSubRepo.GetByID(ctx, subscriptionID)
}

// GetByID 根据ID获取订阅
func (s *SubscriptionService) GetByID(ctx context.Context, id int64) (*UserSubscription, error) {
	return s.userSubRepo.GetByID(ctx, id)
}

// GetActiveSubscription 获取用户对特定分组的有效订阅
// 使用 L1 缓存 + singleflight 加速中间件热路径。
// 返回缓存对象的浅拷贝，调用方可安全修改字段而不会污染缓存或触发 data race。
func (s *SubscriptionService) GetActiveSubscription(ctx context.Context, userID, groupID int64) (*UserSubscription, error) {
	key := subCacheKey(userID, groupID)

	// L1 缓存命中：返回浅拷贝
	if s.subCacheL1 != nil {
		if v, ok := s.subCacheL1.Get(key); ok {
			if sub, ok := v.(*UserSubscription); ok {
				cp := *sub
				return &cp, nil
			}
		}
	}

	// singleflight 防止并发击穿
	value, err, _ := s.subCacheGroup.Do(key, func() (any, error) {
		sub, err := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, userID, groupID)
		if err != nil {
			return nil, err // 直接透传 repo 已翻译的错误（NotFound → ErrSubscriptionNotFound，其他错误原样返回）
		}
		// 写入 L1 缓存
		if s.subCacheL1 != nil {
			_ = s.subCacheL1.SetWithTTL(key, sub, 1, s.jitteredTTL(s.subCacheTTL))
		}
		return sub, nil
	})
	if err != nil {
		return nil, err
	}
	// singleflight 返回的也是缓存指针，需要浅拷贝
	sub, ok := value.(*UserSubscription)
	if !ok || sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	cp := *sub
	return &cp, nil
}

// ListUserSubscriptions 获取用户的所有订阅
func (s *SubscriptionService) ListUserSubscriptions(ctx context.Context, userID int64) ([]UserSubscription, error) {
	subs, err := s.userSubRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	normalizeExpiredWindows(subs)
	normalizeSubscriptionStatus(subs)
	return subs, nil
}

// ListActiveUserSubscriptions 获取用户的所有有效订阅
func (s *SubscriptionService) ListActiveUserSubscriptions(ctx context.Context, userID int64) ([]UserSubscription, error) {
	subs, err := s.userSubRepo.ListActiveByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	normalizeExpiredWindows(subs)
	return subs, nil
}

// ListGroupSubscriptions 获取分组的所有订阅
func (s *SubscriptionService) ListGroupSubscriptions(ctx context.Context, groupID int64, page, pageSize int) ([]UserSubscription, *pagination.PaginationResult, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	subs, pag, err := s.userSubRepo.ListByGroupID(ctx, groupID, params)
	if err != nil {
		return nil, nil, err
	}
	normalizeExpiredWindows(subs)
	normalizeSubscriptionStatus(subs)
	return subs, pag, nil
}

// List 获取所有订阅（分页，支持筛选和排序）
func (s *SubscriptionService) List(ctx context.Context, page, pageSize int, userID, groupID *int64, status, platform, sortBy, sortOrder string) ([]UserSubscription, *pagination.PaginationResult, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	subs, pag, err := s.userSubRepo.List(ctx, params, userID, groupID, status, platform, sortBy, sortOrder)
	if err != nil {
		return nil, nil, err
	}
	normalizeExpiredWindows(subs)
	normalizeSubscriptionStatus(subs)
	return subs, pag, nil
}

// normalizeExpiredWindows 将已过期窗口的数据清零（仅影响返回数据，不影响数据库）
// 这确保前端显示正确的当前窗口状态，而不是过期窗口的历史数据
func normalizeExpiredWindows(subs []UserSubscription) {
	normalizeExpiredWindowsAt(subs, time.Now())
}

func normalizeExpiredWindowsAt(subs []UserSubscription, now time.Time) {
	for i := range subs {
		sub := &subs[i]
		// 日窗口过期：清零展示数据
		if sub.canAutomaticallyResetDailyAt(now) {
			sub.DailyWindowStart = nil
			sub.DailyUsageUSD = 0
		}
		// 周窗口过期：清零展示数据
		if sub.canAutomaticallyResetWeeklyAt(now) {
			sub.WeeklyWindowStart = nil
			sub.WeeklyUsageUSD = 0
		}
		// 月窗口过期：清零展示数据
		if sub.canAutomaticallyResetMonthlyAt(now) {
			sub.MonthlyWindowStart = nil
			sub.MonthlyUsageUSD = 0
		}
	}
}

// normalizeSubscriptionStatus 根据实际过期时间修正状态（仅影响返回数据，不影响数据库）
// 这确保前端显示正确的状态，即使定时任务尚未更新数据库
func normalizeSubscriptionStatus(subs []UserSubscription) {
	now := time.Now()
	for i := range subs {
		sub := &subs[i]
		if sub.Status == SubscriptionStatusActive && !sub.ExpiresAt.After(now) {
			sub.Status = SubscriptionStatusExpired
		}
	}
}

// startOfDay 返回给定时间所在日期的零点（保持原时区）
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// CheckAndActivateWindow 检查并激活窗口（首次使用时）
func (s *SubscriptionService) CheckAndActivateWindow(ctx context.Context, sub *UserSubscription) error {
	return s.checkAndActivateWindowAt(ctx, sub, s.now())
}

func (s *SubscriptionService) checkAndActivateWindowAt(ctx context.Context, sub *UserSubscription, now time.Time) error {
	if sub.IsWindowActivated() {
		return nil
	}

	// 日窗口锚定当天 0 点（日历日语义）；周/月窗口锚定首次使用时刻（期限对齐语义，
	// 锚点不得早于 StartsAt，否则最后一个不完整周期会重复发放额度，见 issue #5051）。
	return s.userSubRepo.ActivateWindows(ctx, sub.ID, timezone.StartOfDay(now), now)
}

// AdminResetQuota manually resets the daily, weekly, and/or monthly usage windows.
func (s *SubscriptionService) AdminResetQuota(ctx context.Context, subscriptionID int64, resetDaily, resetWeekly, resetMonthly bool) (*UserSubscription, error) {
	if !resetDaily && !resetWeekly && !resetMonthly {
		return nil, ErrInvalidInput
	}
	sub, err := s.userSubRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	// 日窗口锚点取当天 0 点：手动重置只清空用量，不改变“每天 0 点刷新”的节奏。
	// 周/月窗口保持锚定重置时刻（期限对齐滚动窗口语义）。
	if err := s.userSubRepo.ResetUsageWindows(ctx, sub.ID, resetDaily, resetWeekly, resetMonthly, timezone.StartOfDay(now), now); err != nil {
		return nil, err
	}
	// Invalidate L1 ristretto cache. Ristretto's Del() is asynchronous by design,
	// so call Wait() immediately after to flush pending operations and guarantee
	// the deleted key is not returned on the very next Get() call.
	s.InvalidateSubCacheSync(sub.UserID, sub.GroupID)
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateSubscription(ctx, sub.UserID, sub.GroupID)
	}
	// Return the refreshed subscription from DB
	return s.userSubRepo.GetByID(ctx, subscriptionID)
}

// CheckAndResetWindows 检查并重置过期的窗口
func (s *SubscriptionService) CheckAndResetWindows(ctx context.Context, sub *UserSubscription) error {
	now := s.now()
	needsInvalidateCache := false

	// 日窗口重置（每天 0 点刷新，按日历日对齐）
	if windowStart, ok := sub.automaticDailyWindowStartAt(now); ok {
		expectedWindowStart := sub.DailyWindowStart
		if err := s.userSubRepo.ResetDailyUsage(ctx, sub.ID, expectedWindowStart, windowStart); err != nil {
			return err
		}
		sub.DailyWindowStart = &windowStart
		sub.DailyUsageUSD = 0
		needsInvalidateCache = true
	}
	// 周窗口重置。
	//
	// 拼车订阅的"一周"应当等于上游 OpenAI 的一周：上游按自己的节奏重置，
	// 与我们发车那天锚定的 7 天网格未必对得上。所以这里不再只看"过了 7 天"，
	// 而是算出目标窗口起点再比对——两个方向的漂移都能纠正：
	//   上游提前重置 → 本地计时器没响，但目标窗口已前移，跟着重置；
	//   上游推后重置 → 本地 7 天到点，但目标窗口没动，保持不变，避免全车
	//                  拿着新保底去撞一个尚未重置的上游账号。
	// 上游数据缺失或陈旧时目标退回本地 7 天网格，行为与原来一致。
	if sub.HasWeeklyReserve() && sub.WeeklyWindowStart != nil {
		target, fromUpstream := s.carpoolWeeklyWindowTarget(ctx, sub)
		// 判据只有一个：窗口确实前移了足够多（见 CarpoolWeeklyWindowAdvanced）。
		// 早先用的是"偏离超过容差"，双向都算——上游一抖就被误判成新窗口，
		// 抖回去又把成员往回搬，生产上把一整周的用量清成了伪周期。
		shouldReset := CarpoolWeeklyWindowAdvanced(*sub.WeeklyWindowStart, target)
		if !fromUpstream {
			// 降级路径：维持原语义，只在本地 7 天到点后吸附网格。
			shouldReset = sub.NeedsWeeklyReset() && target.After(*sub.WeeklyWindowStart)
		}
		if shouldReset {
			// 优先整组一次性重锚：全车写同一个窗口起点，公共池计数器的 key
			// 才不会因为各人各存一份而裂开（见 CarpoolGroupWindowResetter）。
			// 顺带把不发请求、否则永远不会被重锚的成员也带上。
			outcome := s.resetCarpoolGroupWindow(ctx, sub, target)
			switch outcome {
			case carpoolGroupResetApplied:
				sub.WeeklyWindowStart = &target
				sub.WeeklyUsageUSD = 0
				needsInvalidateCache = true
			case carpoolGroupResetDeclined:
				// 整组重锚看过全组状态后判定"不该写"（并发下已有人重锚过，或目标
				// 并未真正前移）。这是一个决定，不是能力缺失——绝不能退回单订阅
				// 路径去做它刚拒绝的事，那正是倒挂周期的来源。
			case carpoolGroupResetUnavailable:
				// 未注入或出错才退回单订阅路径；这里再自查一次方向，
				// 降级路径同样不允许把窗口往回搬。
				if !target.After(*sub.WeeklyWindowStart) {
					break
				}
				expectedWindowStart := sub.WeeklyWindowStart
				// 先把这一周落成台账，再清零——顺序反了就永远拿不到该周用量。
				s.recordClosedCarpoolCycle(ctx, sub, target)
				if err := s.userSubRepo.ResetWeeklyUsage(ctx, sub.ID, expectedWindowStart, target); err != nil {
					return err
				}
				sub.WeeklyWindowStart = &target
				sub.WeeklyUsageUSD = 0
				needsInvalidateCache = true
			}
		}
	} else if windowStart, ok := sub.automaticWindowStartAt(sub.WeeklyWindowStart, 7*24*time.Hour, now); ok {
		expectedWindowStart := sub.WeeklyWindowStart
		if err := s.userSubRepo.ResetWeeklyUsage(ctx, sub.ID, expectedWindowStart, windowStart); err != nil {
			return err
		}
		sub.WeeklyWindowStart = &windowStart
		sub.WeeklyUsageUSD = 0
		needsInvalidateCache = true
	}

	// 月窗口重置（30天）
	if windowStart, ok := sub.automaticWindowStartAt(sub.MonthlyWindowStart, 30*24*time.Hour, now); ok {
		expectedWindowStart := sub.MonthlyWindowStart
		if err := s.userSubRepo.ResetMonthlyUsage(ctx, sub.ID, expectedWindowStart, windowStart); err != nil {
			return err
		}
		sub.MonthlyWindowStart = &windowStart
		sub.MonthlyUsageUSD = 0
		needsInvalidateCache = true
	}

	// 如果有窗口被重置，失效缓存以保持一致性
	if needsInvalidateCache {
		s.InvalidateSubCache(sub.UserID, sub.GroupID)
		if s.billingCacheService != nil {
			_ = s.billingCacheService.InvalidateSubscription(ctx, sub.UserID, sub.GroupID)
		}
	}

	return nil
}

// EnsureWindowMaintenance advances expired usage windows before a request is
// allowed to proceed. It returns a fresh database snapshot because a competing
// request may have won one of the conditional resets.
func (s *SubscriptionService) EnsureWindowMaintenance(ctx context.Context, sub *UserSubscription) (*UserSubscription, error) {
	if sub == nil {
		return nil, ErrSubscriptionNilInput
	}
	if !sub.IsWindowActivated() {
		if err := s.CheckAndActivateWindow(ctx, sub); err != nil {
			return nil, err
		}
	}
	if err := s.CheckAndResetWindows(ctx, sub); err != nil {
		return nil, err
	}

	// GetByID bypasses the service caches. This prevents a stale loser of the
	// CAS from validating limits against zeroed in-memory usage.
	refreshed, err := s.userSubRepo.GetByID(ctx, sub.ID)
	if err != nil {
		return nil, err
	}
	s.InvalidateSubCacheSync(sub.UserID, sub.GroupID)
	return refreshed, nil
}

// CheckUsageLimits 检查使用限额（返回错误如果超限）
// 用于中间件的快速预检查，additionalCost 通常为 0
func (s *SubscriptionService) CheckUsageLimits(ctx context.Context, sub *UserSubscription, group *Group, additionalCost float64) error {
	if !sub.CheckDailyLimit(group, additionalCost) {
		return ErrDailyLimitExceeded
	}
	if !sub.CheckWeeklyLimit(group, additionalCost) {
		return ErrWeeklyLimitExceeded
	}
	if !sub.CheckMonthlyLimit(group, additionalCost) {
		return ErrMonthlyLimitExceeded
	}
	return nil
}

// ValidateAndCheckLimits 合并验证+限额检查（中间件热路径专用）
// 除拼车公共池计数器读取（Redis GET）外仅做内存检查，不触发 DB 写入。
// 调用方必须在放行请求前同步完成窗口维护。
// 返回 needsMaintenance 表示是否需要执行窗口维护并回读数据库快照。
func (s *SubscriptionService) ValidateAndCheckLimits(ctx context.Context, sub *UserSubscription, group *Group) (needsMaintenance bool, err error) {
	now := s.now()
	// 1. 验证订阅状态
	if sub.Status == SubscriptionStatusExpired {
		return false, ErrSubscriptionExpired
	}
	if sub.Status == SubscriptionStatusSuspended {
		return false, ErrSubscriptionSuspended
	}
	if !sub.ExpiresAt.After(now) {
		return false, ErrSubscriptionExpired
	}

	// 2. 内存中修正过期窗口的用量，确保预检查不会误拒绝用户。
	//    调用方随后同步推进 DB 窗口，并用回读快照重新校验。
	if sub.canAutomaticallyResetDailyAt(now) {
		sub.DailyUsageUSD = 0
		needsMaintenance = true
	}
	if sub.canAutomaticallyResetWeeklyAt(now) {
		sub.WeeklyUsageUSD = 0
		needsMaintenance = true
	} else if s.carpoolWeeklyWindowDrifted(ctx, sub) {
		// 上游在我们的 7 天到点之前就重置了：本地计时器还没响，但那一周
		// 事实上已经翻篇。不在这里跟上就会拿上一周的用量继续挡用户。
		sub.WeeklyUsageUSD = 0
		needsMaintenance = true
	}
	if sub.canAutomaticallyResetMonthlyAt(now) {
		sub.MonthlyUsageUSD = 0
		needsMaintenance = true
	}
	if !sub.IsWindowActivated() {
		needsMaintenance = true
	}

	// 3. 检查用量限额
	if !sub.CheckDailyLimit(group, 0) {
		return needsMaintenance, ErrDailyLimitExceeded
	}
	if !sub.CheckWeeklyLimit(group, 0) {
		return needsMaintenance, ErrWeeklyLimitExceeded
	}
	if !sub.CheckMonthlyLimit(group, 0) {
		return needsMaintenance, ErrMonthlyLimitExceeded
	}

	// 4. 拼车组级公共池硬约束（设计文档 §4.2 v3.2）：
	//    周用量 < 保底 r → 无条件放行（保底硬保证，不读计数器）；
	//    周用量 ≥ r → 全车超额之和（组级计数器）必须 < 公共池容量 C。
	if err := s.checkCarpoolCommons(ctx, sub); err != nil {
		return needsMaintenance, err
	}

	return needsMaintenance, nil
}

// checkCarpoolCommons 通过 BillingCacheService 注入的计数器执行公共池预检查。
// 计数器未注入（billingCacheService/counter 为 nil）或读取失败时 fail-open：
// 保底硬保证不依赖计数器，公共池强制降级不阻塞保底内用量。
func (s *SubscriptionService) checkCarpoolCommons(ctx context.Context, sub *UserSubscription) error {
	if s == nil || s.billingCacheService == nil {
		return nil
	}
	return s.billingCacheService.checkCarpoolCommonsEligibility(ctx, sub, sub.WeeklyUsageUSD)
}

// DoWindowMaintenance 异步执行窗口维护（激活+重置）
// 使用独立 context，不受请求取消影响。
// 注意：此方法仅在 ValidateAndCheckLimits 返回 needsMaintenance=true 时调用，
// 而 IsExpired()=true 的订阅在 ValidateAndCheckLimits 中已被拦截返回错误，
// 因此进入此方法的订阅一定未过期，无需处理过期状态同步。
func (s *SubscriptionService) DoWindowMaintenance(sub *UserSubscription) {
	if s == nil {
		return
	}
	if s.maintenanceQueue != nil {
		err := s.maintenanceQueue.TryEnqueue(func() {
			s.doWindowMaintenance(sub)
		})
		if err != nil {
			log.Printf("Subscription maintenance enqueue failed: %v", err)
		}
		return
	}

	s.doWindowMaintenance(sub)
}

func (s *SubscriptionService) doWindowMaintenance(sub *UserSubscription) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 激活窗口（首次使用时）
	if !sub.IsWindowActivated() {
		if err := s.CheckAndActivateWindow(ctx, sub); err != nil {
			log.Printf("Failed to activate subscription windows: %v", err)
		}
	}

	// 重置过期窗口
	if err := s.CheckAndResetWindows(ctx, sub); err != nil {
		log.Printf("Failed to reset subscription windows: %v", err)
	}

	// 失效 L1 缓存，确保后续请求拿到更新后的数据
	s.InvalidateSubCache(sub.UserID, sub.GroupID)
}

// RecordUsage 记录使用量到订阅
func (s *SubscriptionService) RecordUsage(ctx context.Context, subscriptionID int64, costUSD float64) error {
	return s.userSubRepo.IncrementUsage(ctx, subscriptionID, costUSD)
}

// SubscriptionProgress 订阅进度
type SubscriptionProgress struct {
	ID            int64                `json:"id"`
	GroupName     string               `json:"group_name"`
	ExpiresAt     time.Time            `json:"expires_at"`
	ExpiresInDays int                  `json:"expires_in_days"`
	Daily         *UsageWindowProgress `json:"daily,omitempty"`
	Weekly        *UsageWindowProgress `json:"weekly,omitempty"`
	Monthly       *UsageWindowProgress `json:"monthly,omitempty"`
}

// UsageWindowProgress 使用窗口进度
type UsageWindowProgress struct {
	LimitUSD        float64   `json:"limit_usd"`
	UsedUSD         float64   `json:"used_usd"`
	RemainingUSD    float64   `json:"remaining_usd"`
	Percentage      float64   `json:"percentage"`
	WindowStart     time.Time `json:"window_start"`
	ResetsAt        time.Time `json:"resets_at"`
	ResetsInSeconds int64     `json:"resets_in_seconds"`
}

// GetSubscriptionProgress 获取订阅使用进度
func (s *SubscriptionService) GetSubscriptionProgress(ctx context.Context, subscriptionID int64) (*SubscriptionProgress, error) {
	sub, err := s.userSubRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, ErrSubscriptionNotFound
	}

	group := sub.Group
	if group == nil {
		group, err = s.groupRepo.GetByID(ctx, sub.GroupID)
		if err != nil {
			return nil, err
		}
	}

	return s.calculateProgress(sub, group), nil
}

// calculateProgress 根据已加载的订阅和分组数据计算使用进度（纯内存计算，无 DB 查询）
func (s *SubscriptionService) calculateProgress(sub *UserSubscription, group *Group) *SubscriptionProgress {
	progress := &SubscriptionProgress{
		ID:            sub.ID,
		GroupName:     group.Name,
		ExpiresAt:     sub.ExpiresAt,
		ExpiresInDays: sub.DaysRemaining(),
	}

	// 日进度
	if group.HasDailyLimit() && sub.DailyWindowStart != nil {
		limit := *group.DailyLimitUSD
		resetsAt := sub.DailyWindowStart.Add(24 * time.Hour)
		if dailyResetTime := sub.DailyResetTime(); dailyResetTime != nil {
			resetsAt = *dailyResetTime
		}
		progress.Daily = &UsageWindowProgress{
			LimitUSD:        limit,
			UsedUSD:         sub.DailyUsageUSD,
			RemainingUSD:    limit - sub.DailyUsageUSD,
			Percentage:      (sub.DailyUsageUSD / limit) * 100,
			WindowStart:     *sub.DailyWindowStart,
			ResetsAt:        resetsAt,
			ResetsInSeconds: int64(time.Until(resetsAt).Seconds()),
		}
		if progress.Daily.RemainingUSD < 0 {
			progress.Daily.RemainingUSD = 0
		}
		if progress.Daily.Percentage > 100 {
			progress.Daily.Percentage = 100
		}
		if progress.Daily.ResetsInSeconds < 0 {
			progress.Daily.ResetsInSeconds = 0
		}
	}

	// 周进度（订阅级限额覆盖优先于分组级限额）
	if weeklyLimit := sub.EffectiveWeeklyLimit(group); weeklyLimit != nil && *weeklyLimit > 0 && sub.WeeklyWindowStart != nil {
		limit := *weeklyLimit
		resetsAt := sub.WeeklyWindowStart.Add(7 * 24 * time.Hour)
		if weeklyResetTime := sub.WeeklyResetTime(); weeklyResetTime != nil {
			resetsAt = *weeklyResetTime
		}
		progress.Weekly = &UsageWindowProgress{
			LimitUSD:        limit,
			UsedUSD:         sub.WeeklyUsageUSD,
			RemainingUSD:    limit - sub.WeeklyUsageUSD,
			Percentage:      (sub.WeeklyUsageUSD / limit) * 100,
			WindowStart:     *sub.WeeklyWindowStart,
			ResetsAt:        resetsAt,
			ResetsInSeconds: int64(time.Until(resetsAt).Seconds()),
		}
		if progress.Weekly.RemainingUSD < 0 {
			progress.Weekly.RemainingUSD = 0
		}
		if progress.Weekly.Percentage > 100 {
			progress.Weekly.Percentage = 100
		}
		if progress.Weekly.ResetsInSeconds < 0 {
			progress.Weekly.ResetsInSeconds = 0
		}
	}

	// 月进度
	if group.HasMonthlyLimit() && sub.MonthlyWindowStart != nil {
		limit := *group.MonthlyLimitUSD
		resetsAt := sub.MonthlyWindowStart.Add(30 * 24 * time.Hour)
		if monthlyResetTime := sub.MonthlyResetTime(); monthlyResetTime != nil {
			resetsAt = *monthlyResetTime
		}
		progress.Monthly = &UsageWindowProgress{
			LimitUSD:        limit,
			UsedUSD:         sub.MonthlyUsageUSD,
			RemainingUSD:    limit - sub.MonthlyUsageUSD,
			Percentage:      (sub.MonthlyUsageUSD / limit) * 100,
			WindowStart:     *sub.MonthlyWindowStart,
			ResetsAt:        resetsAt,
			ResetsInSeconds: int64(time.Until(resetsAt).Seconds()),
		}
		if progress.Monthly.RemainingUSD < 0 {
			progress.Monthly.RemainingUSD = 0
		}
		if progress.Monthly.Percentage > 100 {
			progress.Monthly.Percentage = 100
		}
		if progress.Monthly.ResetsInSeconds < 0 {
			progress.Monthly.ResetsInSeconds = 0
		}
	}

	return progress
}

// GetUserSubscriptionsWithProgress 获取用户所有订阅及进度
func (s *SubscriptionService) GetUserSubscriptionsWithProgress(ctx context.Context, userID int64) ([]SubscriptionProgress, error) {
	// ListActiveByUserID 已使用 .WithGroup() eager-load Group 关联，1 次查询获取所有数据
	subs, err := s.userSubRepo.ListActiveByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	progresses := make([]SubscriptionProgress, 0, len(subs))
	for i := range subs {
		sub := &subs[i]
		group := sub.Group
		if group == nil {
			continue
		}
		progresses = append(progresses, *s.calculateProgress(sub, group))
	}

	return progresses, nil
}

// ValidateSubscription 验证订阅是否有效
func (s *SubscriptionService) ValidateSubscription(ctx context.Context, sub *UserSubscription) error {
	if sub.Status == SubscriptionStatusExpired {
		return ErrSubscriptionExpired
	}
	if sub.Status == SubscriptionStatusSuspended {
		return ErrSubscriptionSuspended
	}
	if sub.IsExpired() {
		// 更新状态
		_ = s.userSubRepo.UpdateStatus(ctx, sub.ID, SubscriptionStatusExpired)
		return ErrSubscriptionExpired
	}
	return nil
}

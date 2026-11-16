package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	cprForecastPeriodWeekly       = "weekly"
	cprResetCreditStatusAvailable = "available"
)

var (
	// cpr 未配置（cpr.admin_base_url 为空）：整个拼车周额度功能禁用，
	// 惯例是答 503 而不是 404。
	ErrCarpoolUpstreamQuotaNotConfigured = infraerrors.New(503, "CARPOOL_UPSTREAM_QUOTA_NOT_CONFIGURED", "upstream quota is not configured")
	// 服务端刷新冷却中（carpool:upstream_quota:refresh:<id> 窗口未过期）。
	ErrCarpoolUpstreamQuotaRefreshTooFrequent = infraerrors.TooManyRequests("CARPOOL_UPSTREAM_QUOTA_REFRESH_TOO_FREQUENT", "upstream quota refresh was requested recently; please wait before retrying")
)

// CarpoolUpstreamQuotaRepository 是 CarpoolUpstreamQuotaService 的最小依赖面。
// 由 *carpoolRepository 实现；窄接口让 service 测试免于 stub 整个 CarpoolRepository。
type CarpoolUpstreamQuotaRepository interface {
	// GetByID 加载车辆并填充查看者视角的 MemberRole（可见性判定与 GetRoster 同源）。
	GetByID(ctx context.Context, carpoolID, userID int64) (*Carpool, error)
	// GetCprAccountID 解析本车绑定的上游 cpr 账号 id（未配置返回 ""）。
	GetCprAccountID(ctx context.Context, carpoolID int64) (string, error)
	// ListCprAccountIDs 批量解析一组车的 cpr 账号 id（GetCprAccountID 的列表版，
	// 一条 SQL 完成主账号 + 组内回退解析）。map 里不存在的 id 表示未配置。
	ListCprAccountIDs(ctx context.Context, carpoolIDs []int64) (map[int64]string, error)
}

// CarpoolUpstreamQuotaCache 是周额度 Redis 缓存的最小抽象（repository 实现）。
// 实现为 nil 时整体跳过：GET 每次直查 cpr，refresh 无服务端冷却。
type CarpoolUpstreamQuotaCache interface {
	// Get 读快照；ok=false 表示 miss 或实现为 nil。
	Get(ctx context.Context, carpoolID int64) (entry *CarpoolUpstreamQuotaCacheEntry, ok bool, err error)
	// Set 写快照，ttl 由调用方从 cpr.cache_ttl_seconds 解析。
	Set(ctx context.Context, carpoolID int64, entry *CarpoolUpstreamQuotaCacheEntry, ttl time.Duration) error
	// TryAcquireRefreshCooldown 用 SET NX EX 抢占刷新冷却；false 表示冷却中。
	TryAcquireRefreshCooldown(ctx context.Context, carpoolID int64, cooldown time.Duration) (bool, error)
	// ReleaseRefreshCooldown 释放刷新冷却窗口（刷新失败时归还，避免失败也占冷却）。
	ReleaseRefreshCooldown(ctx context.Context, carpoolID int64) error
}

// CarpoolUpstreamQuotaCacheEntry 是写进 Redis 的周额度快照
// （key carpool:upstream_quota:<carpoolID>）。只缓存上游载荷，
// cached 标记等响应期字段在组装响应时现填。reset_credits / account_expiry
// 为 null 只表示拉取失败或无数据（best-effort），不代表账号没有重置额度/订阅。
type CarpoolUpstreamQuotaCacheEntry struct {
	CprAccountID  string                      `json:"cpr_account_id"`
	GeneratedAt   string                      `json:"generated_at"`
	Weekly        *UpstreamQuotaWeekly        `json:"weekly"`
	ResetCredits  *UpstreamQuotaResetCredits  `json:"reset_credits"`
	AccountExpiry *UpstreamQuotaAccountExpiry `json:"account_expiry"`
}

// UpstreamQuotaWeekly 是 cpr quota-forecast 里 period=="weekly" 那一项的展示形态。
// Available = source != null && source.usedPercent != null（cpr 尚无观测时 false）；
// 各 Display 字段是 cpr 的预格式化字符串，原样透出。
type UpstreamQuotaWeekly struct {
	Available              bool     `json:"available"`
	UnavailableReason      *string  `json:"unavailable_reason"`
	LowSample              bool     `json:"low_sample"`
	UsedPercent            *float64 `json:"used_percent"`
	UsedPercentDisplay     string   `json:"used_percent_display"`
	ObservedAt             string   `json:"observed_at"`
	ResetAt                string   `json:"reset_at"`
	TokensDisplay          string   `json:"tokens_display"`
	USDDisplay             string   `json:"usd_display"`
	EstimatedTokens        *uint64  `json:"estimated_tokens"`
	EstimatedTokensDisplay string   `json:"estimated_tokens_display"`
	EstimatedUSD           *float64 `json:"estimated_usd"`
	EstimatedUSDDisplay    string   `json:"estimated_usd_display"`
}

// UpstreamQuotaResetCredits 是 cpr reset-credits 的展示形态：
// AvailableCount 是 status=="available" 的重置券张数（直接取 cpr 的 availableCount），
// NextExpiresAt 是 available 券里最近的 expiresAt（RFC3339 原样字符串），没有则为 null。
type UpstreamQuotaResetCredits struct {
	AvailableCount int     `json:"available_count"`
	NextExpiresAt  *string `json:"next_expires_at"`
}

// UpstreamQuotaAccountExpiry 是 cpr personal-info 里 subscription 的展示形态：
// ExpiresAt 是 RFC3339 原样字符串；WillRenew 为 null 表示 cpr 未观测到续费状态。
type UpstreamQuotaAccountExpiry struct {
	ExpiresAt string `json:"expires_at"`
	WillRenew *bool  `json:"will_renew"`
}

// CarpoolUpstreamSummary 是列表接口随车返回的上游摘要（仅 GET /carpools 填充）：
// Configured=false 表示未配置 cpr 账号映射（或功能未启用），此时其余字段均为 null。
type CarpoolUpstreamSummary struct {
	Configured  bool     `json:"configured"`
	UsedPercent *float64 `json:"used_percent"`
	ResetAt     *string  `json:"reset_at"`
	ExpiresAt   *string  `json:"expires_at"`
}

// UpstreamQuotaResult 是 GET / POST 共用的响应载荷（handler 包一层 {"upstream_quota": ...}）。
type UpstreamQuotaResult struct {
	Configured    bool                        `json:"configured"`
	CarpoolID     int64                       `json:"carpool_id"`
	CprAccountID  string                      `json:"cpr_account_id,omitempty"`
	GeneratedAt   string                      `json:"generated_at,omitempty"`
	Cached        bool                        `json:"cached"`
	Weekly        *UpstreamQuotaWeekly        `json:"weekly"`
	ResetCredits  *UpstreamQuotaResetCredits  `json:"reset_credits"`
	AccountExpiry *UpstreamQuotaAccountExpiry `json:"account_expiry"`
	Refresh       UpstreamQuotaRefresh        `json:"refresh"`
}

// UpstreamQuotaRefresh 描述 POST upstream-quota/refresh 的服务端冷却参数。
type UpstreamQuotaRefresh struct {
	CooldownSeconds int `json:"cooldown_seconds"`
}

// CarpoolUpstreamQuotaService 查询拼车绑定的上游 cpr 账号周额度：
// 服务端代理 cpr admin API，Redis 快照缓存（默认 5 分钟），POST refresh 强制刷新
// （默认 60 秒冷却）。可见性口径与 GetRoster 完全一致（成员/车主/admin，
// 未上车者仅 public 且招募中可读）。
type CarpoolUpstreamQuotaService struct {
	repo            CarpoolUpstreamQuotaRepository
	cpr             *CprClient
	cache           CarpoolUpstreamQuotaCache
	cacheTTL        time.Duration
	refreshCooldown time.Duration
}

func NewCarpoolUpstreamQuotaService(repo CarpoolUpstreamQuotaRepository, cpr *CprClient, cache CarpoolUpstreamQuotaCache, cfg *config.Config) *CarpoolUpstreamQuotaService {
	cacheTTL := 5 * time.Minute
	refreshCooldown := time.Minute
	if cfg != nil {
		if cfg.Cpr.CacheTTLSeconds > 0 {
			cacheTTL = time.Duration(cfg.Cpr.CacheTTLSeconds) * time.Second
		}
		if cfg.Cpr.RefreshCooldownSeconds > 0 {
			refreshCooldown = time.Duration(cfg.Cpr.RefreshCooldownSeconds) * time.Second
		}
	}
	return &CarpoolUpstreamQuotaService{
		repo:            repo,
		cpr:             cpr,
		cache:           cache,
		cacheTTL:        cacheTTL,
		refreshCooldown: refreshCooldown,
	}
}

// Get 返回周额度：可见性校验 → 解析 cpr 账号映射（未配置 → configured:false）→
// 缓存命中直返（cached:true）→ 未命中直查 cpr 并回填缓存（cached:false）。
func (s *CarpoolUpstreamQuotaService) Get(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*UpstreamQuotaResult, error) {
	if s.cpr == nil {
		return nil, ErrCarpoolUpstreamQuotaNotConfigured
	}
	if err := s.checkVisibility(ctx, carpoolID, actorUserID, isAdmin); err != nil {
		return nil, err
	}
	accountID, err := s.repo.GetCprAccountID(ctx, carpoolID)
	if err != nil {
		return nil, err
	}
	if accountID == "" {
		return s.unconfiguredResult(carpoolID), nil
	}
	if s.cache != nil {
		entry, ok, err := s.cache.Get(ctx, carpoolID)
		if err != nil {
			// 缓存坏了不影响主流程：落到 cpr 直查。
			slog.Warn("carpool upstream quota cache read failed; querying cpr directly",
				"carpool_id", carpoolID, "error", err)
		} else if ok && entry != nil {
			return s.resultFromEntry(carpoolID, accountID, entry, true), nil
		}
	}
	entry, err := s.fetchAndCache(ctx, carpoolID, accountID)
	if err != nil {
		return nil, err
	}
	return s.resultFromEntry(carpoolID, accountID, entry, false), nil
}

// Refresh 强制刷新：可见性校验 → 解析映射（未配置 → configured:false，不消耗冷却）
// → 抢占冷却窗口（失败 429）→ cpr 触发重观测 → 拉新快照回填缓存并返回。
func (s *CarpoolUpstreamQuotaService) Refresh(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*UpstreamQuotaResult, error) {
	if s.cpr == nil {
		return nil, ErrCarpoolUpstreamQuotaNotConfigured
	}
	if err := s.checkVisibility(ctx, carpoolID, actorUserID, isAdmin); err != nil {
		return nil, err
	}
	accountID, err := s.repo.GetCprAccountID(ctx, carpoolID)
	if err != nil {
		return nil, err
	}
	if accountID == "" {
		return s.unconfiguredResult(carpoolID), nil
	}
	acquired := false
	if s.cache != nil {
		acquired, err = s.cache.TryAcquireRefreshCooldown(ctx, carpoolID, s.refreshCooldown)
		if err != nil {
			// 冷却判断本身失败不阻塞刷新（fail-open，与缓存读取降级同一口径）。
			slog.Warn("carpool upstream quota refresh cooldown check failed; allowing refresh",
				"carpool_id", carpoolID, "error", err)
			acquired = false
		} else if !acquired {
			return nil, ErrCarpoolUpstreamQuotaRefreshTooFrequent
		}
	}
	// 刷新失败归还冷却窗口，避免一次失败把用户锁在冷却里。
	release := func() {
		if !acquired || s.cache == nil {
			return
		}
		if relErr := s.cache.ReleaseRefreshCooldown(ctx, carpoolID); relErr != nil {
			slog.Warn("carpool upstream quota refresh cooldown release failed",
				"carpool_id", carpoolID, "error", relErr)
		}
	}
	if err := s.cpr.RefreshQuota(ctx, accountID); err != nil {
		release()
		return nil, err
	}
	entry, err := s.fetchAndCache(ctx, carpoolID, accountID)
	if err != nil {
		release()
		return nil, err
	}
	return s.resultFromEntry(carpoolID, accountID, entry, false), nil
}

// checkVisibility 与 GetRoster 同一道闸：能看 roster 就能看周额度（同一个信任边界）。
// 本接口不接受邀请 token，invite_only 的未上车者一律 403。
func (s *CarpoolUpstreamQuotaService) checkVisibility(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error {
	item, err := s.repo.GetByID(ctx, carpoolID, actorUserID)
	if err != nil {
		return err
	}
	if !canViewCarpoolUpstreamQuota(item, actorUserID, isAdmin) {
		return ErrCarpoolForbidden
	}
	return nil
}

// canViewCarpoolUpstreamQuota 复刻 CarpoolService.canViewGroupQRCode 的放行规则
// （去掉邀请 token 分支——该分支只服务二维码入口，这里恒为空串等价于 false）。
func canViewCarpoolUpstreamQuota(item *Carpool, actorUserID int64, isAdmin bool) bool {
	if item == nil {
		return false
	}
	if isAdmin || item.MemberRole != nil {
		return true
	}
	if item.OwnerUserID != nil && *item.OwnerUserID == actorUserID {
		return true
	}
	// 未上车者只在车还招募中时需要额度信息（上车决策参考）。
	if item.Status != "recruiting" {
		return false
	}
	return item.Visibility == CarpoolVisibilityPublic
}

// fetchAndCache 直查 cpr quota-forecast、reset-credits 与 personal-info(subscription)，
// 组装快照并回填缓存（缓存写失败只记日志）。拉取顺序 forecast → reset-credits →
// subscription，三段全部进同一快照、同一 TTL。后两段是展示增强，拉挂了不拖垮主数据：
// 只记 WARN，快照里对应字段留 null（响应同样为 null）。
func (s *CarpoolUpstreamQuotaService) fetchAndCache(ctx context.Context, carpoolID int64, accountID string) (*CarpoolUpstreamQuotaCacheEntry, error) {
	data, err := s.cpr.QuotaForecast(ctx, accountID)
	if err != nil {
		return nil, err
	}
	entry := &CarpoolUpstreamQuotaCacheEntry{
		CprAccountID: accountID,
		GeneratedAt:  data.GeneratedAt,
		Weekly:       buildWeeklyForecast(data),
	}
	resetCredits, err := s.cpr.ResetCredits(ctx, accountID)
	if err != nil {
		slog.Warn("carpool upstream quota reset credits fetch failed; continuing without it",
			"carpool_id", carpoolID, "error", err)
	} else {
		entry.ResetCredits = buildResetCredits(resetCredits)
	}
	subscription, err := s.cpr.Subscription(ctx, accountID)
	if err != nil {
		slog.Warn("carpool upstream quota subscription fetch failed; continuing without it",
			"carpool_id", carpoolID, "error", err)
	} else if subscription != nil {
		entry.AccountExpiry = &UpstreamQuotaAccountExpiry{
			ExpiresAt: subscription.ExpiresAt,
			WillRenew: subscription.WillRenew,
		}
	}
	if s.cache != nil {
		if err := s.cache.Set(ctx, carpoolID, entry, s.cacheTTL); err != nil {
			slog.Warn("carpool upstream quota cache write failed",
				"carpool_id", carpoolID, "error", err)
		}
	}
	return entry, nil
}

// ListSummaries 给列表车批量补上游摘要（GET /carpools 的 enrichment 入口）。
// 每车先读快照缓存（命中直接用，不打 cpr）；miss 的车串行 best-effort fetchAndCache
// 并回填缓存（该车失败只记 WARN、字段留 null，不阻塞列表）。串行是故意的：miss 是
// 少数路径（缓存 TTL 5 分钟，详情页浏览持续回填），避免并发打爆 cpr。
// cpr 未配置（功能禁用）时返回空 map，调用方按全部 configured:false 处理。
func (s *CarpoolUpstreamQuotaService) ListSummaries(ctx context.Context, carpoolIDs []int64) (map[int64]*CarpoolUpstreamSummary, error) {
	out := make(map[int64]*CarpoolUpstreamSummary, len(carpoolIDs))
	if s.cpr == nil {
		return out, nil
	}
	accountIDs, err := s.repo.ListCprAccountIDs(ctx, carpoolIDs)
	if err != nil {
		return nil, err
	}
	for _, id := range carpoolIDs {
		summary := &CarpoolUpstreamSummary{}
		out[id] = summary
		accountID := accountIDs[id]
		if accountID == "" {
			continue
		}
		summary.Configured = true
		entry, ok, cacheErr := s.cachedEntry(ctx, id)
		if cacheErr != nil {
			slog.Warn("carpool upstream quota cache read failed; querying cpr directly",
				"carpool_id", id, "error", cacheErr)
		}
		if !ok {
			entry, err = s.fetchAndCache(ctx, id, accountID)
			if err != nil {
				slog.Warn("carpool upstream quota list summary fetch failed; leaving fields null",
					"carpool_id", id, "error", err)
				continue
			}
		}
		if entry.Weekly != nil {
			summary.UsedPercent = entry.Weekly.UsedPercent
			if entry.Weekly.ResetAt != "" {
				resetAt := entry.Weekly.ResetAt
				summary.ResetAt = &resetAt
			}
		}
		if entry.AccountExpiry != nil && entry.AccountExpiry.ExpiresAt != "" {
			expiresAt := entry.AccountExpiry.ExpiresAt
			summary.ExpiresAt = &expiresAt
		}
	}
	return out, nil
}

// cachedEntry 读快照；缓存未注入时恒 miss。
func (s *CarpoolUpstreamQuotaService) cachedEntry(ctx context.Context, carpoolID int64) (*CarpoolUpstreamQuotaCacheEntry, bool, error) {
	if s.cache == nil {
		return nil, false, nil
	}
	return s.cache.Get(ctx, carpoolID)
}

// buildResetCredits 把 cpr reset-credits 收敛成展示形态。NextExpiresAt 取
// status=="available" 里最近的 expiresAt（按 RFC3339 解析比较，保留原始字符串），
// 没有可用券或全部解析失败则为 null。
func buildResetCredits(data *CprResetCreditsData) *UpstreamQuotaResetCredits {
	if data == nil {
		return nil
	}
	out := &UpstreamQuotaResetCredits{AvailableCount: data.AvailableCount}
	var nextAt time.Time
	nextRaw := ""
	found := false
	for i := range data.Credits {
		c := &data.Credits[i]
		if c.Status != cprResetCreditStatusAvailable {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, c.ExpiresAt)
		if err != nil {
			continue
		}
		if !found || parsed.Before(nextAt) {
			found = true
			nextAt = parsed
			nextRaw = c.ExpiresAt
		}
	}
	if found {
		out.NextExpiresAt = &nextRaw
	}
	return out
}

// buildWeeklyForecast 从 quota-forecast 里挑 period=="weekly" 那一项。没有 weekly 项
// （cpr 异常或只产出 monthly）返回 nil——configured:true 但 weekly:null，前端按无数据显示。
func buildWeeklyForecast(data *CprQuotaForecastData) *UpstreamQuotaWeekly {
	if data == nil {
		return nil
	}
	for i := range data.Forecasts {
		f := &data.Forecasts[i]
		if f.Period != cprForecastPeriodWeekly {
			continue
		}
		w := &UpstreamQuotaWeekly{
			UnavailableReason:      f.UnavailableReason,
			LowSample:              f.LowSample,
			EstimatedTokens:        f.EstimatedTokens,
			EstimatedTokensDisplay: f.EstimatedTokensDisplay,
			EstimatedUSD:           f.EstimatedUSD,
			EstimatedUSDDisplay:    f.EstimatedUSDDisplay,
		}
		if f.Source != nil {
			w.UsedPercent = f.Source.UsedPercent
			w.UsedPercentDisplay = f.Source.UsedPercentDisplay
			w.ObservedAt = f.Source.ObservedAt
			w.ResetAt = f.Source.ResetAt
			w.TokensDisplay = f.Source.TokensDisplay
			w.USDDisplay = f.Source.USDDisplay
			w.Available = f.Source.UsedPercent != nil
		}
		return w
	}
	return nil
}

func (s *CarpoolUpstreamQuotaService) resultFromEntry(carpoolID int64, accountID string, entry *CarpoolUpstreamQuotaCacheEntry, cached bool) *UpstreamQuotaResult {
	return &UpstreamQuotaResult{
		Configured:    true,
		CarpoolID:     carpoolID,
		CprAccountID:  accountID,
		GeneratedAt:   entry.GeneratedAt,
		Cached:        cached,
		Weekly:        entry.Weekly,
		ResetCredits:  entry.ResetCredits,
		AccountExpiry: entry.AccountExpiry,
		Refresh:       UpstreamQuotaRefresh{CooldownSeconds: int(s.refreshCooldown / time.Second)},
	}
}

func (s *CarpoolUpstreamQuotaService) unconfiguredResult(carpoolID int64) *UpstreamQuotaResult {
	return &UpstreamQuotaResult{
		Configured: false,
		CarpoolID:  carpoolID,
		Refresh:    UpstreamQuotaRefresh{CooldownSeconds: int(s.refreshCooldown / time.Second)},
	}
}

package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const cprForecastPeriodWeekly = "weekly"

var (
	// cpr 未配置（cpr.admin_base_url 为空）：整个拼车周额度功能禁用。
	// 与 Kasm 禁用同一惯例（REMOTE_SESSION_NOT_CONFIGURED），答 503 而不是 404。
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
}

// CarpoolUpstreamQuotaCacheEntry 是写进 Redis 的周额度快照
// （key carpool:upstream_quota:<carpoolID>）。只缓存上游载荷，
// cached 标记等响应期字段在组装响应时现填。
type CarpoolUpstreamQuotaCacheEntry struct {
	CprAccountID string               `json:"cpr_account_id"`
	GeneratedAt  string               `json:"generated_at"`
	Weekly       *UpstreamQuotaWeekly `json:"weekly"`
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

// UpstreamQuotaResult 是 GET / POST 共用的响应载荷（handler 包一层 {"upstream_quota": ...}）。
type UpstreamQuotaResult struct {
	Configured   bool                 `json:"configured"`
	CarpoolID    int64                `json:"carpool_id"`
	CprAccountID string               `json:"cpr_account_id,omitempty"`
	GeneratedAt  string               `json:"generated_at,omitempty"`
	Cached       bool                 `json:"cached"`
	Weekly       *UpstreamQuotaWeekly `json:"weekly"`
	Refresh      UpstreamQuotaRefresh `json:"refresh"`
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
	if s.cache != nil {
		acquired, err := s.cache.TryAcquireRefreshCooldown(ctx, carpoolID, s.refreshCooldown)
		if err != nil {
			// 冷却判断本身失败不阻塞刷新（fail-open，与缓存读取降级同一口径）。
			slog.Warn("carpool upstream quota refresh cooldown check failed; allowing refresh",
				"carpool_id", carpoolID, "error", err)
		} else if !acquired {
			return nil, ErrCarpoolUpstreamQuotaRefreshTooFrequent
		}
	}
	if err := s.cpr.RefreshQuota(ctx, accountID); err != nil {
		return nil, err
	}
	entry, err := s.fetchAndCache(ctx, carpoolID, accountID)
	if err != nil {
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

// fetchAndCache 直查 cpr quota-forecast、组装快照并回填缓存（缓存写失败只记日志）。
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
	if s.cache != nil {
		if err := s.cache.Set(ctx, carpoolID, entry, s.cacheTTL); err != nil {
			slog.Warn("carpool upstream quota cache write failed",
				"carpool_id", carpoolID, "error", err)
		}
	}
	return entry, nil
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
		Configured:   true,
		CarpoolID:    carpoolID,
		CprAccountID: accountID,
		GeneratedAt:  entry.GeneratedAt,
		Cached:       cached,
		Weekly:       entry.Weekly,
		Refresh:      UpstreamQuotaRefresh{CooldownSeconds: int(s.refreshCooldown / time.Second)},
	}
}

func (s *CarpoolUpstreamQuotaService) unconfiguredResult(carpoolID int64) *UpstreamQuotaResult {
	return &UpstreamQuotaResult{
		Configured: false,
		CarpoolID:  carpoolID,
		Refresh:    UpstreamQuotaRefresh{CooldownSeconds: int(s.refreshCooldown / time.Second)},
	}
}

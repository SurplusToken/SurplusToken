package service

import (
	"context"
	"log/slog"
	"time"
)

// CarpoolUsageSummaryMe 是查看者本人在本车的本周用量与预估账单。
// DeclaredUSD/ReservedUSD/EstimatedBillUSD 仅额度预约制（car_type 2/3）车有值：
// 老车（car_type 0/1 无保底）这三项为 null，前端按「按量计费」展示。
// ReservedUSD 来自订阅 weekly_reserved_usd（= 申报 × reserve_ratio），
// EstimatedBillUSD = max(reserved, actual)，与结算单的 billable 同公式
// （CarpoolCycleBillableUSD）。
type CarpoolUsageSummaryMe struct {
	WeekUSD          float64  `json:"week_usd"`
	DeclaredUSD      *float64 `json:"declared_usd"`
	ReservedUSD      *float64 `json:"reserved_usd"`
	EstimatedBillUSD *float64 `json:"estimated_bill_usd"`
}

// CarpoolUsageSummaryTotal 是全车成员（status active/joined）的本周用量合计。
type CarpoolUsageSummaryTotal struct {
	WeekUSD     float64 `json:"week_usd"`
	MemberCount int     `json:"member_count"`
}

// CarpoolUsageSummary 是 GET /carpools/:id/usage-summary 的载荷
// （handler 包一层 {"usage_summary": ...}）。所有统计都是 best-effort：
// 成员用量查询失败时 Total/Me/WeekStart 为对应零值或 null，不 500。
type CarpoolUsageSummary struct {
	WeekStart *time.Time `json:"week_start"`
	WeekEnd   *time.Time `json:"week_end"`
	// Me 是查看者本人的行；非成员（public+recruiting 可见但未上车）为 null。
	Me *CarpoolUsageSummaryMe `json:"me"`
	// Total 恒非 null（查询失败时是零值）。
	Total          CarpoolUsageSummaryTotal `json:"total"`
	WeeklyLimitUSD *float64                 `json:"weekly_limit_usd"`
}

// CarpoolMemberWeekUsageRow 是车上一位成员（carpool_members status active/joined）
// 的本周用量行，字段与订阅页用量面板（ListCarpoolUsageSnapshots）同源：
// user_subscriptions.weekly_usage_usd / weekly_reserved_usd + 成员申报。
type CarpoolMemberWeekUsageRow struct {
	UserID                 int64
	SubscriptionID         int64
	DeclaredWeeklyQuotaUSD float64
	WeeklyUsageUSD         float64
	WeeklyReservedUSD      *float64
	WeeklyWindowStart      *time.Time
}

// CarpoolUsageSummaryRepository 是 CarpoolUsageSummaryService 的最小依赖面，
// 由 *carpoolRepository 实现；窄接口让 service 测试免于 stub 整个 CarpoolRepository。
type CarpoolUsageSummaryRepository interface {
	// GetByID 加载车辆并填充查看者视角的 MemberRole（可见性判定与 GetRoster 同源）。
	GetByID(ctx context.Context, carpoolID, userID int64) (*Carpool, error)
	// ListMemberWeekUsage 返回车上 active/joined 成员的本周用量行（含无订阅绑定的行）。
	ListMemberWeekUsage(ctx context.Context, carpoolID int64) ([]CarpoolMemberWeekUsageRow, error)
}

// CarpoolUsageSummaryMaintainer 是查看者订阅窗口维护的最小依赖面（*SubscriptionService
// 实现）。与订阅页同一道维护：读数前先做 EnsureWindowMaintenance，保证过期窗口
// 被推进、清零后的「本周」口径与面板一致。
type CarpoolUsageSummaryMaintainer interface {
	GetByID(ctx context.Context, subscriptionID int64) (*UserSubscription, error)
	EnsureWindowMaintenance(ctx context.Context, sub *UserSubscription) (*UserSubscription, error)
}

// CarpoolUsageSummaryService 汇总一辆车本周用量：成员估算自己最后付多少
// （我的本周/保底/预计结算）+ 全车合计。数据全部读自家库（订阅行），秒回，
// 不做 Redis 缓存。可见性与 upstream-quota 完全同一道闸（成员/admin，
// 未上车者仅 public 且招募中可读）。
type CarpoolUsageSummaryService struct {
	repo          CarpoolUsageSummaryRepository
	subscriptions CarpoolUsageSummaryMaintainer
}

func NewCarpoolUsageSummaryService(repo CarpoolUsageSummaryRepository, subscriptions CarpoolUsageSummaryMaintainer) *CarpoolUsageSummaryService {
	return &CarpoolUsageSummaryService{repo: repo, subscriptions: subscriptions}
}

// Get 返回本车本周用量摘要。可见性校验失败/车不存在照常返回错误；
// 成员用量统计失败只记 WARN，返回仅带 weekly_limit_usd 的部分摘要。
func (s *CarpoolUsageSummaryService) Get(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*CarpoolUsageSummary, error) {
	item, err := s.repo.GetByID(ctx, carpoolID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !canViewCarpoolUpstreamQuota(item, actorUserID, isAdmin) {
		return nil, ErrCarpoolForbidden
	}

	weeklyLimitUSD := item.WeeklyLimitUSD
	summary := &CarpoolUsageSummary{WeeklyLimitUSD: &weeklyLimitUSD}

	rows, err := s.repo.ListMemberWeekUsage(ctx, carpoolID)
	if err != nil {
		slog.Warn("carpool usage summary member usage query failed; returning partial summary",
			"carpool_id", carpoolID, "error", err)
		return summary, nil
	}
	summary.Total.MemberCount = len(rows)

	// 查看者本人的行 + 全车最新的窗口起点（非成员查看时作为本周锚点）。
	var viewer *CarpoolMemberWeekUsageRow
	var carWindow *time.Time
	for i := range rows {
		row := &rows[i]
		if row.UserID == actorUserID && viewer == nil {
			viewer = row
		}
		if row.WeeklyWindowStart != nil &&
			(carWindow == nil || row.WeeklyWindowStart.After(*carWindow)) {
			window := *row.WeeklyWindowStart
			carWindow = &window
		}
	}

	viewerUsage, viewerReserved, viewerWindow := 0.0, (*float64)(nil), carWindow
	if viewer != nil {
		viewerUsage = viewer.WeeklyUsageUSD
		viewerReserved = viewer.WeeklyReservedUSD
		if viewer.WeeklyWindowStart != nil {
			window := *viewer.WeeklyWindowStart
			viewerWindow = &window
		}
		if maintained, ok := s.maintainViewerSubscription(ctx, viewer.SubscriptionID); ok {
			viewerUsage = maintained.WeeklyUsageUSD
			viewerReserved = maintained.WeeklyReservedUSD
			if maintained.WeeklyWindowStart != nil {
				window := *maintained.WeeklyWindowStart
				viewerWindow = &window
			}
		}
	}
	summary.WeekStart = viewerWindow
	if viewerWindow != nil {
		weekEnd := viewerWindow.Add(7 * 24 * time.Hour)
		summary.WeekEnd = &weekEnd
	}

	// 全车合计：Σ 成员 weekly_usage_usd。查看者的量用维护后的值替换，
	// 与订阅页「先维护 viewer 再聚合」的口径一致；其余成员按行读（面板同款）。
	total := 0.0
	for i := range rows {
		total += rows[i].WeeklyUsageUSD
	}
	if viewer != nil {
		total += viewerUsage - viewer.WeeklyUsageUSD
	}
	summary.Total.WeekUSD = total

	if viewer == nil {
		return summary, nil
	}
	me := &CarpoolUsageSummaryMe{WeekUSD: viewerUsage}
	if item.CarType == CarpoolCarTypeQuota || item.CarType == CarpoolCarTypeQuotaV2 {
		declared := viewer.DeclaredWeeklyQuotaUSD
		me.DeclaredUSD = &declared
		me.ReservedUSD = viewerReserved
		if viewerReserved != nil {
			bill := CarpoolCycleBillableUSD(viewerUsage, *viewerReserved)
			me.EstimatedBillUSD = &bill
		}
	}
	summary.Me = me
	return summary, nil
}

// maintainViewerSubscription 对查看者订阅做窗口维护（订阅页 ListCarpoolUsageSnapshots
// 同款），返回维护后的订阅行。加载或维护失败只记 WARN，调用方退回成员行快照——
// 统计块是 best-effort，不为维护失败 500。
func (s *CarpoolUsageSummaryService) maintainViewerSubscription(ctx context.Context, subscriptionID int64) (*UserSubscription, bool) {
	if s.subscriptions == nil {
		return nil, false
	}
	sub, err := s.subscriptions.GetByID(ctx, subscriptionID)
	if err != nil {
		slog.Warn("carpool usage summary load viewer subscription failed; using member row snapshot",
			"subscription_id", subscriptionID, "error", err)
		return nil, false
	}
	maintained, err := s.subscriptions.EnsureWindowMaintenance(ctx, sub)
	if err != nil {
		slog.Warn("carpool usage summary window maintenance failed; using member row snapshot",
			"subscription_id", subscriptionID, "error", err)
		return nil, false
	}
	if maintained == nil {
		return sub, true
	}
	return maintained, true
}

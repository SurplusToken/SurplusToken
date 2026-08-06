package service

import (
	"context"
	"log/slog"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// ErrCarpoolSharedPoolUnavailable 表示公共池快照所需的订阅或计数器暂时不可用。
// 这里不能返回一个看似为 0 的快照：把“读不到”展示成“额度已耗尽”会误导成员。
var ErrCarpoolSharedPoolUnavailable = infraerrors.ServiceUnavailable(
	"CARPOOL_SHARED_POOL_UNAVAILABLE",
	"carpool shared pool snapshot is temporarily unavailable",
)

// CarpoolSharedPoolSnapshot 是当前拼车周窗口内的公共池实时快照。
// 所有数值均为 USD；RemainingUSD 始终按剩余额度口径输出。
type CarpoolSharedPoolSnapshot struct {
	CapacityUSD  float64   `json:"capacity_usd"`
	UsedUSD      float64   `json:"used_usd"`
	RemainingUSD float64   `json:"remaining_usd"`
	WindowStart  time.Time `json:"window_start"`
	ResetsAt     time.Time `json:"resets_at"`
}

// GetCarpoolSharedPoolSnapshot 读取一个组当前周窗口的公共池快照。
//
// 订阅行只用于取得发车时锁定的保底/个人上限与窗口锚点；当前窗口会优先跟随
// 上游窗口，缺失时退回本地 7 天网格，与实际限额检查使用同一套算法。
func (s *SubscriptionService) GetCarpoolSharedPoolSnapshot(ctx context.Context, groupID int64) (*CarpoolSharedPoolSnapshot, error) {
	if s == nil || s.userSubRepo == nil || s.billingCacheService == nil {
		return nil, ErrCarpoolSharedPoolUnavailable
	}
	// 仓储查询包含历史软删除行；一辆车当前虽最多 30 人，但长期换人后历史行可能
	// 超过 30，不能让它们把仍有效的订阅挤出这一页。
	subs, _, err := s.userSubRepo.ListByGroupID(ctx, groupID, pagination.PaginationParams{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, ErrCarpoolSharedPoolUnavailable.WithCause(err)
	}
	sub := selectCarpoolSharedPoolSubscription(subs, time.Now())
	if sub == nil {
		return nil, ErrCarpoolSharedPoolUnavailable
	}

	// 使用当前应处的窗口查询计数器，不把只读详情接口变成订阅维护入口。
	windowStart, _ := s.carpoolWeeklyWindowTarget(ctx, sub)
	if windowStart.IsZero() {
		return nil, ErrCarpoolSharedPoolUnavailable
	}
	current := *sub
	current.WeeklyWindowStart = &windowStart
	return s.billingCacheService.carpoolSharedPoolSnapshot(ctx, &current)
}

func selectCarpoolSharedPoolSubscription(subs []UserSubscription, now time.Time) *UserSubscription {
	var selected *UserSubscription
	for i := range subs {
		sub := &subs[i]
		if sub.DeletedAt != nil || sub.Status != SubscriptionStatusActive || !sub.ExpiresAt.After(now) ||
			!sub.HasWeeklyReserve() || sub.WeeklyWindowStart == nil {
			continue
		}
		if selected == nil || sub.WeeklyWindowStart.After(*selected.WeeklyWindowStart) {
			selected = sub
		}
	}
	return selected
}

func (s *BillingCacheService) carpoolSharedPoolSnapshot(ctx context.Context, sub *UserSubscription) (*CarpoolSharedPoolSnapshot, error) {
	if s == nil || sub == nil || sub.WeeklyWindowStart == nil || !sub.HasWeeklyReserve() {
		return nil, ErrCarpoolSharedPoolUnavailable
	}
	capacity := s.carpoolCommonsCapacity(ctx, sub)
	if capacity < 0 {
		capacity = 0
	}
	used, ok, err := s.GetCarpoolCommonsUsage(ctx, sub.GroupID, *sub.WeeklyWindowStart)
	if err != nil {
		slog.Warn("carpool shared pool: read usage failed", "group_id", sub.GroupID, "error", err)
		return nil, ErrCarpoolSharedPoolUnavailable.WithCause(err)
	}
	if !ok {
		return nil, ErrCarpoolSharedPoolUnavailable
	}
	if used < 0 {
		used = 0
	}
	remaining := capacity - used
	if remaining < 0 {
		remaining = 0
	}
	return &CarpoolSharedPoolSnapshot{
		CapacityUSD:  capacity,
		UsedUSD:      used,
		RemainingUSD: remaining,
		WindowStart:  *sub.WeeklyWindowStart,
		ResetsAt:     sub.WeeklyWindowStart.Add(carpoolWeeklyWindowDuration),
	}, nil
}

// GetSharedPool 对外提供车内公共池快照。公开车在“车外”也不能读取；只有本车
// 成员、车主或管理员能看到实时消耗，避免把组级运行数据暴露给任意登录用户。
func (s *CarpoolService) GetSharedPool(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*CarpoolSharedPoolSnapshot, error) {
	item, err := s.repo.GetByID(ctx, carpoolID, actorUserID)
	if err != nil {
		return nil, err
	}
	isOwner := item.OwnerUserID != nil && *item.OwnerUserID == actorUserID
	if !isAdmin && !isOwner && item.MemberRole == nil {
		return nil, ErrCarpoolForbidden
	}
	if !item.IsQuotaModel() || item.Status != "active" || item.GroupID == nil || s.subscriptionService == nil {
		return nil, ErrCarpoolUnavailable
	}
	return s.subscriptionService.GetCarpoolSharedPoolSnapshot(ctx, *item.GroupID)
}

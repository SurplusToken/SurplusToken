package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type usageSummaryRepoStub struct {
	carpool    *Carpool
	getByIDErr error
	rows       []CarpoolMemberWeekUsageRow
	rowsErr    error
}

func (s *usageSummaryRepoStub) GetByID(ctx context.Context, carpoolID, userID int64) (*Carpool, error) {
	if s.getByIDErr != nil {
		return nil, s.getByIDErr
	}
	return s.carpool, nil
}

func (s *usageSummaryRepoStub) ListMemberWeekUsage(ctx context.Context, carpoolID int64) ([]CarpoolMemberWeekUsageRow, error) {
	if s.rowsErr != nil {
		return nil, s.rowsErr
	}
	return s.rows, nil
}

// usageSummaryMaintainerStub 模拟 *SubscriptionService 的「加载 + 窗口维护」：
// maintenanceHook 在维护阶段改写订阅（如模拟过期窗口被重置），返回维护后的行。
type usageSummaryMaintainerStub struct {
	subs            map[int64]*UserSubscription
	getByIDErr      error
	maintenanceErr  error
	maintenanceHook func(sub *UserSubscription)
	maintenanceCalls int
}

func (m *usageSummaryMaintainerStub) GetByID(ctx context.Context, subscriptionID int64) (*UserSubscription, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	sub := m.subs[subscriptionID]
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	copy := *sub
	return &copy, nil
}

func (m *usageSummaryMaintainerStub) EnsureWindowMaintenance(ctx context.Context, sub *UserSubscription) (*UserSubscription, error) {
	m.maintenanceCalls++
	if m.maintenanceErr != nil {
		return nil, m.maintenanceErr
	}
	if m.maintenanceHook != nil {
		m.maintenanceHook(sub)
	}
	if refreshed := m.subs[sub.ID]; refreshed != nil {
		return refreshed, nil
	}
	return sub, nil
}

func usageSummaryWindow() time.Time {
	return time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
}

// quota 车成员：me 带申报/保底/预计结算（estimated = max(reserved, actual)），
// total 为全车 Σ weekly_usage_usd。
func TestCarpoolUsageSummaryMemberQuotaCar(t *testing.T) {
	window := usageSummaryWindow()
	repo := &usageSummaryRepoStub{
		carpool: &Carpool{
			ID:             7,
			CarType:        CarpoolCarTypeQuotaV2,
			Status:         "active",
			Visibility:     CarpoolVisibilityInviteOnly,
			MemberRole:     strPtr("member"),
			WeeklyLimitUSD: 2400,
		},
		rows: []CarpoolMemberWeekUsageRow{
			{UserID: 8, SubscriptionID: 102, DeclaredWeeklyQuotaUSD: 700, WeeklyUsageUSD: 588.0, WeeklyReservedUSD: float64Ptr(560), WeeklyWindowStart: &window},
			{UserID: 7, SubscriptionID: 101, DeclaredWeeklyQuotaUSD: 600, WeeklyUsageUSD: 500.0, WeeklyReservedUSD: float64Ptr(480), WeeklyWindowStart: &window},
			{UserID: 9, SubscriptionID: 103, DeclaredWeeklyQuotaUSD: 600, WeeklyUsageUSD: 352.7, WeeklyReservedUSD: float64Ptr(480), WeeklyWindowStart: &window},
			{UserID: 10, SubscriptionID: 104, DeclaredWeeklyQuotaUSD: 500, WeeklyUsageUSD: 330.1, WeeklyReservedUSD: float64Ptr(400), WeeklyWindowStart: &window},
		},
	}
	maintainer := &usageSummaryMaintainerStub{subs: map[int64]*UserSubscription{
		101: {
			ID: 101, UserID: 7, GroupID: 9, Status: SubscriptionStatusActive,
			ExpiresAt:         time.Now().Add(24 * time.Hour),
			WeeklyWindowStart: &window, WeeklyUsageUSD: 500.0, WeeklyReservedUSD: float64Ptr(480),
		},
	}}
	svc := NewCarpoolUsageSummaryService(repo, maintainer)

	summary, err := svc.Get(context.Background(), 7, 7, false)
	require.NoError(t, err)
	require.Equal(t, 1, maintainer.maintenanceCalls)

	require.NotNil(t, summary.WeekStart)
	require.Equal(t, window, *summary.WeekStart)
	require.NotNil(t, summary.WeekEnd)
	require.Equal(t, window.Add(7*24*time.Hour), *summary.WeekEnd)

	require.NotNil(t, summary.Me)
	require.InDelta(t, 500.0, summary.Me.WeekUSD, 1e-9)
	require.NotNil(t, summary.Me.DeclaredUSD)
	require.InDelta(t, 600.0, *summary.Me.DeclaredUSD, 1e-9)
	require.NotNil(t, summary.Me.ReservedUSD)
	require.InDelta(t, 480.0, *summary.Me.ReservedUSD, 1e-9)
	// actual(500) > reserved(480) → 预计结算按 actual，与结算 billable 同公式。
	require.NotNil(t, summary.Me.EstimatedBillUSD)
	require.InDelta(t, 500.0, *summary.Me.EstimatedBillUSD, 1e-9)

	require.InDelta(t, 1770.8, summary.Total.WeekUSD, 1e-9)
	require.Equal(t, 4, summary.Total.MemberCount)

	require.NotNil(t, summary.WeeklyLimitUSD)
	require.InDelta(t, 2400.0, *summary.WeeklyLimitUSD, 1e-9)

	payload, err := json.Marshal(summary)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"week_start":"2026-10-12T00:00:00Z"`)
}

// 保底高于实际：estimated_bill 落到 reserved（地板）。
func TestCarpoolUsageSummaryEstimatedBillFloorsAtReserved(t *testing.T) {
	window := usageSummaryWindow()
	repo := &usageSummaryRepoStub{
		carpool: &Carpool{
			ID: 7, CarType: CarpoolCarTypeQuota, Status: "active",
			Visibility: CarpoolVisibilityInviteOnly, MemberRole: strPtr("member"), WeeklyLimitUSD: 2400,
		},
		rows: []CarpoolMemberWeekUsageRow{
			{UserID: 7, SubscriptionID: 101, DeclaredWeeklyQuotaUSD: 20, WeeklyUsageUSD: 12.34, WeeklyReservedUSD: float64Ptr(16.96), WeeklyWindowStart: &window},
		},
	}
	maintainer := &usageSummaryMaintainerStub{subs: map[int64]*UserSubscription{
		101: {ID: 101, WeeklyWindowStart: &window, WeeklyUsageUSD: 12.34, WeeklyReservedUSD: float64Ptr(16.96)},
	}}
	svc := NewCarpoolUsageSummaryService(repo, maintainer)

	summary, err := svc.Get(context.Background(), 7, 7, false)
	require.NoError(t, err)
	require.InDelta(t, 12.34, summary.Me.WeekUSD, 1e-9)
	require.NotNil(t, summary.Me.EstimatedBillUSD)
	require.InDelta(t, 16.96, *summary.Me.EstimatedBillUSD, 1e-9)
}

// 非成员（public + recruiting 可见）打开详情：me 为 null，total/week 用全车最新窗口。
func TestCarpoolUsageSummaryNonMemberViewer(t *testing.T) {
	window := usageSummaryWindow()
	repo := &usageSummaryRepoStub{
		carpool: &Carpool{
			ID: 8, CarType: CarpoolCarTypeQuotaV2, Status: "recruiting",
			Visibility: CarpoolVisibilityPublic, WeeklyLimitUSD: 2400,
		},
		rows: []CarpoolMemberWeekUsageRow{
			{UserID: 7, SubscriptionID: 101, DeclaredWeeklyQuotaUSD: 600, WeeklyUsageUSD: 100, WeeklyReservedUSD: float64Ptr(480), WeeklyWindowStart: &window},
			{UserID: 9, SubscriptionID: 103, DeclaredWeeklyQuotaUSD: 600, WeeklyUsageUSD: 200, WeeklyReservedUSD: float64Ptr(480), WeeklyWindowStart: &window},
		},
	}
	svc := NewCarpoolUsageSummaryService(repo, &usageSummaryMaintainerStub{})

	summary, err := svc.Get(context.Background(), 8, 99, false)
	require.NoError(t, err)
	require.Nil(t, summary.Me, "non-member viewer must not get a me block")
	require.Equal(t, 2, summary.Total.MemberCount)
	require.InDelta(t, 300.0, summary.Total.WeekUSD, 1e-9)
	require.NotNil(t, summary.WeekStart)
	require.Equal(t, window, *summary.WeekStart)
}

// 老车（car_type 1 无保底）：me 只有实际用量，declared/reserved/estimated 全 null。
func TestCarpoolUsageSummaryLegacyCarWithoutReserve(t *testing.T) {
	window := usageSummaryWindow()
	repo := &usageSummaryRepoStub{
		carpool: &Carpool{
			ID: 9, CarType: CarpoolCarTypeQuotaLegacy, Status: "active",
			Visibility: CarpoolVisibilityInviteOnly, MemberRole: strPtr("member"), WeeklyLimitUSD: 2400,
		},
		rows: []CarpoolMemberWeekUsageRow{
			{UserID: 7, SubscriptionID: 101, WeeklyUsageUSD: 45.6, WeeklyWindowStart: &window},
			{UserID: 9, SubscriptionID: 105, WeeklyUsageUSD: 10.0, WeeklyWindowStart: &window},
		},
	}
	maintainer := &usageSummaryMaintainerStub{subs: map[int64]*UserSubscription{
		101: {ID: 101, WeeklyWindowStart: &window, WeeklyUsageUSD: 45.6},
	}}
	svc := NewCarpoolUsageSummaryService(repo, maintainer)

	summary, err := svc.Get(context.Background(), 9, 7, false)
	require.NoError(t, err)
	require.NotNil(t, summary.Me)
	require.InDelta(t, 45.6, summary.Me.WeekUSD, 1e-9)
	require.Nil(t, summary.Me.DeclaredUSD)
	require.Nil(t, summary.Me.ReservedUSD)
	require.Nil(t, summary.Me.EstimatedBillUSD)
	require.InDelta(t, 55.6, summary.Total.WeekUSD, 1e-9)

	payload, err := json.Marshal(summary.Me)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"declared_usd":null`)
	require.Contains(t, string(payload), `"estimated_bill_usd":null`)
}

// 可见性与 upstream-quota 同一道闸：invite_only 非成员 403；repo 错误照常透传。
func TestCarpoolUsageSummaryVisibilityAndErrors(t *testing.T) {
	stranger := &usageSummaryRepoStub{carpool: &Carpool{
		ID: 7, CarType: CarpoolCarTypeQuotaV2, Status: "recruiting",
		Visibility: CarpoolVisibilityInviteOnly,
	}}
	svc := NewCarpoolUsageSummaryService(stranger, nil)
	_, err := svc.Get(context.Background(), 7, 99, false)
	require.ErrorIs(t, err, ErrCarpoolForbidden)

	broken := &usageSummaryRepoStub{getByIDErr: errors.New("db down")}
	svc = NewCarpoolUsageSummaryService(broken, nil)
	_, err = svc.Get(context.Background(), 7, 99, false)
	require.EqualError(t, err, "db down")
}

// 成员用量统计失败是 best-effort：不 500，返回仅带 weekly_limit 的部分摘要。
func TestCarpoolUsageSummaryMemberQueryFailureDegrades(t *testing.T) {
	repo := &usageSummaryRepoStub{
		carpool: &Carpool{
			ID: 7, CarType: CarpoolCarTypeQuotaV2, Status: "active",
			Visibility: CarpoolVisibilityInviteOnly, MemberRole: strPtr("member"), WeeklyLimitUSD: 2400,
		},
		rowsErr: errors.New("usage query failed"),
	}
	svc := NewCarpoolUsageSummaryService(repo, nil)

	summary, err := svc.Get(context.Background(), 7, 7, false)
	require.NoError(t, err)
	require.Nil(t, summary.Me)
	require.Zero(t, summary.Total.WeekUSD)
	require.Zero(t, summary.Total.MemberCount)
	require.Nil(t, summary.WeekStart)
	require.NotNil(t, summary.WeeklyLimitUSD)
	require.InDelta(t, 2400.0, *summary.WeeklyLimitUSD, 1e-9)
}

// 查看者窗口过期：维护后用量清零/窗口推进，me 与 total 都用维护后的值
// （与订阅页「先维护 viewer 再聚合」口径一致）。
func TestCarpoolUsageSummaryAppliesViewerWindowMaintenance(t *testing.T) {
	staleWindow := usageSummaryWindow().Add(-8 * 24 * time.Hour)
	freshWindow := usageSummaryWindow()
	repo := &usageSummaryRepoStub{
		carpool: &Carpool{
			ID: 7, CarType: CarpoolCarTypeQuotaV2, Status: "active",
			Visibility: CarpoolVisibilityInviteOnly, MemberRole: strPtr("member"), WeeklyLimitUSD: 2400,
		},
		rows: []CarpoolMemberWeekUsageRow{
			// 行数据是维护前的旧快照（viewer 用量 777 挂在过期窗口上）。
			{UserID: 7, SubscriptionID: 101, DeclaredWeeklyQuotaUSD: 600, WeeklyUsageUSD: 777, WeeklyReservedUSD: float64Ptr(480), WeeklyWindowStart: &staleWindow},
			{UserID: 9, SubscriptionID: 103, DeclaredWeeklyQuotaUSD: 600, WeeklyUsageUSD: 100, WeeklyReservedUSD: float64Ptr(480), WeeklyWindowStart: &freshWindow},
		},
	}
	maintainer := &usageSummaryMaintainerStub{
		subs: map[int64]*UserSubscription{
			101: {ID: 101, WeeklyWindowStart: &freshWindow, WeeklyUsageUSD: 0, WeeklyReservedUSD: float64Ptr(480)},
		},
	}
	svc := NewCarpoolUsageSummaryService(repo, maintainer)

	summary, err := svc.Get(context.Background(), 7, 7, false)
	require.NoError(t, err)
	require.Equal(t, 1, maintainer.maintenanceCalls)
	require.NotNil(t, summary.WeekStart)
	require.Equal(t, freshWindow, *summary.WeekStart)
	require.InDelta(t, 0.0, summary.Me.WeekUSD, 1e-9)
	// total = 其他成员行 100 + viewer 维护后 0。
	require.InDelta(t, 100.0, summary.Total.WeekUSD, 1e-9)
}

// 维护失败（加载/重置挂了）只降级到成员行快照，不 500。
func TestCarpoolUsageSummaryMaintenanceFailureFallsBackToRow(t *testing.T) {
	window := usageSummaryWindow()
	repo := &usageSummaryRepoStub{
		carpool: &Carpool{
			ID: 7, CarType: CarpoolCarTypeQuotaV2, Status: "active",
			Visibility: CarpoolVisibilityInviteOnly, MemberRole: strPtr("member"), WeeklyLimitUSD: 2400,
		},
		rows: []CarpoolMemberWeekUsageRow{
			{UserID: 7, SubscriptionID: 101, DeclaredWeeklyQuotaUSD: 600, WeeklyUsageUSD: 426.4, WeeklyReservedUSD: float64Ptr(480), WeeklyWindowStart: &window},
		},
	}
	maintainer := &usageSummaryMaintainerStub{
		subs: map[int64]*UserSubscription{
			101: {ID: 101, WeeklyWindowStart: &window, WeeklyUsageUSD: 426.4, WeeklyReservedUSD: float64Ptr(480)},
		},
		maintenanceErr: errors.New("reset failed"),
	}
	svc := NewCarpoolUsageSummaryService(repo, maintainer)

	summary, err := svc.Get(context.Background(), 7, 7, false)
	require.NoError(t, err)
	require.Equal(t, 1, maintainer.maintenanceCalls)
	require.InDelta(t, 426.4, summary.Me.WeekUSD, 1e-9)
	require.NotNil(t, summary.Me.EstimatedBillUSD)
	require.InDelta(t, 480.0, *summary.Me.EstimatedBillUSD, 1e-9)
}

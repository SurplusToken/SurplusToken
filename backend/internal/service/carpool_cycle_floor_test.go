package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func cycle(start time.Time, declared, reserved, actual float64) CarpoolBillingCycle {
	return CarpoolBillingCycle{
		CycleStart:             start,
		CycleEnd:               start.AddDate(0, 0, 7),
		DeclaredWeeklyQuotaUSD: declared,
		ReservedUSD:            reserved,
		ActualUsageUSD:         actual,
		BillableUsageUSD:       CarpoolCycleBillableUSD(actual, reserved),
	}
}

// 80% 地板必须按周逐个算，不能按月整体算——两者不等价，差的是真钱。
//
// 申报 $240/周（保底 $192），四周实际用量 300/300/50/50：
//
//	按周（正确）: 300 + 300 + 192 + 192 = 984
//	按月（错误）: max(700, 768)        = 768
//
// 按月算等于让超用的周补贴没用满的周，而保底"按周刷新、未用完不结转"。
func TestPerCycleFloorDiffersFromMonthlyFloor(t *testing.T) {
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	cycles := []CarpoolBillingCycle{
		cycle(base, 240, 192, 300),
		cycle(base.AddDate(0, 0, 7), 240, 192, 300),
		cycle(base.AddDate(0, 0, 14), 240, 192, 50),
		cycle(base.AddDate(0, 0, 21), 240, 192, 50),
	}

	withCycles := CarpoolSettlementMemberInput{
		UserID: 1, DeclaredWeeklyQuotaUSD: 240, ActualUsageUSD: 700, PeriodDays: 28, Cycles: cycles,
	}
	billable, floor, actual, cycleBased, stats := memberBillable(withCycles, 0.8)
	require.True(t, cycleBased)
	require.InDelta(t, 984, billable, 1e-9, "按周口径")
	require.InDelta(t, 768, floor, 1e-9, "地板合计 = Σ各周保底")
	require.InDelta(t, 700, actual, 1e-9)
	require.Equal(t, 4, stats.count)
	require.Equal(t, 2, stats.floorCycles, "后两周被地板托底")

	// 没有台账时退回按月口径，并明确标记
	noCycles := withCycles
	noCycles.Cycles = nil
	billable, floor, actual, cycleBased, stats = memberBillable(noCycles, 0.8)
	require.False(t, cycleBased)
	require.InDelta(t, 768, billable, 1e-9, "按月口径")
	require.InDelta(t, 768, floor, 1e-9)
	require.InDelta(t, 700, actual, 1e-9)
	require.Zero(t, stats.count)
}

// 每周都用超时两种口径一致（地板不触发）。
func TestPerCycleFloorMatchesWhenAlwaysAboveFloor(t *testing.T) {
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	cycles := []CarpoolBillingCycle{
		cycle(base, 240, 192, 300),
		cycle(base.AddDate(0, 0, 7), 240, 192, 300),
	}
	in := CarpoolSettlementMemberInput{DeclaredWeeklyQuotaUSD: 240, ActualUsageUSD: 600, PeriodDays: 14, Cycles: cycles}
	billable, _, _, _, stats := memberBillable(in, 0.8)
	require.InDelta(t, 600, billable, 1e-9)
	require.Zero(t, stats.floorCycles)
}

// 一周都没用时，整月计费用量恰好等于全部保底之和。
func TestPerCycleFloorWhenNothingUsed(t *testing.T) {
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	cycles := []CarpoolBillingCycle{
		cycle(base, 240, 192, 0),
		cycle(base.AddDate(0, 0, 7), 240, 192, 0),
		cycle(base.AddDate(0, 0, 14), 240, 192, 0),
	}
	in := CarpoolSettlementMemberInput{DeclaredWeeklyQuotaUSD: 240, PeriodDays: 21, Cycles: cycles}
	billable, floor, actual, _, stats := memberBillable(in, 0.8)
	require.InDelta(t, 576, billable, 1e-9, "3 × 192")
	require.InDelta(t, 576, floor, 1e-9)
	require.Zero(t, actual)
	require.Equal(t, 3, stats.floorCycles)
}

// 变动池收支恒等：无论口径如何，Σ最终分摊必须等于变动池总额。
func TestUsagePoolStillBalancesWithCycles(t *testing.T) {
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	inputs := []CarpoolSettlementMemberInput{
		{UserID: 1, DeclaredWeeklyQuotaUSD: 240, PeriodDays: 28, QuotedPrepaidCNY: 140, Cycles: []CarpoolBillingCycle{
			cycle(base, 240, 192, 300), cycle(base.AddDate(0, 0, 7), 240, 192, 50),
		}},
		{UserID: 2, DeclaredWeeklyQuotaUSD: 120, PeriodDays: 28, QuotedPrepaidCNY: 90, Cycles: []CarpoolBillingCycle{
			cycle(base, 120, 96, 10), cycle(base.AddDate(0, 0, 7), 120, 96, 400),
		}},
	}
	members := ComputeCarpoolSettlementMembers(CarpoolCarTypeQuota, 2400, 400, 1000, 0.8, inputs)
	require.Len(t, members, 2)

	total := 0.0
	for _, m := range members {
		total += m.UsageFinalShareCNY
		require.True(t, m.CycleBased)
		require.Equal(t, 2, m.CycleCount)
	}
	require.InDelta(t, 1000, total, 1e-9, "变动池必须收支恒等")

	// 席位费同样必须摊平
	seat := 0.0
	for _, m := range members {
		seat += m.SeatFeeFinalCNY
	}
	require.InDelta(t, 400, seat, 1e-9)
}

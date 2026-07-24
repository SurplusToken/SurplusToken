package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCarpoolPrepaidCNYMatchesDesignAppendixA1(t *testing.T) {
	// A1 满车场景（Σ申报 = 2400）：预付 = 400/N + 1000×(申报/2400)
	require.InDelta(t, 233.33, CarpoolPrepaidCNY(400, 1000, 2400, 400, 6), 0.01) // 重度 6 人
	require.InDelta(t, 140.0, CarpoolPrepaidCNY(400, 1000, 2400, 240, 10), 0.01) // 均衡 10 人
	require.InDelta(t, 70.0, CarpoolPrepaidCNY(400, 1000, 2400, 120, 20), 0.01)  // 轻度 20 人
	require.InDelta(t, 154.17, CarpoolPrepaidCNY(400, 1000, 2400, 250, 8), 0.01) // A3 8 人各 250
	require.InDelta(t, 100.0, CarpoolPrepaidCNY(0, 1000, 2400, 240, 0), 0.01)    // memberCount<1 按 1 计
}

func TestCarpoolMemberWeeklyLimitUSDGuaranteesReserve(t *testing.T) {
	// §4.2：个人订阅周限额 = 0.8×申报 + C，C = 2400 − 0.8×Σ申报。
	// 保底是数学保证：组限额钉死 2400 时，其他人最多消耗 0.8×(Σ−d_i)+C，
	// 恰好给每位成员剩下 0.8×申报。
	weeklyLimit, reserve := 2400.0, 0.8
	declared := []float64{400, 400, 200, 200, 200, 200, 200, 200, 100, 100, 100, 100} // A2 混合场景 Σ=2400
	total := 0.0
	for _, d := range declared {
		total += d
	}
	sharedPool := weeklyLimit - reserve*total
	require.InDelta(t, 480.0, sharedPool, 1e-9)

	for i, d := range declared {
		limit := CarpoolMemberWeeklyLimitUSD(weeklyLimit, reserve, d, total)
		require.InDelta(t, reserve*d+sharedPool, limit, 1e-9)
		others := total - d
		othersMaxConsumption := reserve*others + sharedPool
		guaranteed := weeklyLimit - othersMaxConsumption
		require.InDelta(t, reserve*d, guaranteed, 1e-9, "成员 %d 的保底应恰好等于 0.8×申报", i)
	}
}

func TestBuildDeclarationRecommendation(t *testing.T) {
	// ≥7 天记录：推荐值 = 最近 7 天实际用量（叠加缓冲系数）
	rec := BuildDeclarationRecommendation(70, 7, 1.1)
	require.Equal(t, "usage_history", rec.Basis)
	require.Equal(t, 7, rec.DaysWithRecords)
	require.InDelta(t, 70, rec.RawWeeklyUsageUSD, 1e-9)
	require.InDelta(t, 77, rec.RecommendedWeeklyQuotaUSD, 1e-9)

	// 8 天（跨 8 个日历日）同样按满周处理
	rec = BuildDeclarationRecommendation(80, 8, 1.1)
	require.Equal(t, 7, rec.DaysWithRecords)
	require.InDelta(t, 88, rec.RecommendedWeeklyQuotaUSD, 1e-9)

	// 不足 7 天：按日均外推 ×7
	rec = BuildDeclarationRecommendation(42, 3, 1.1)
	require.Equal(t, "usage_history", rec.Basis)
	require.Equal(t, 3, rec.DaysWithRecords)
	require.InDelta(t, 98, rec.RawWeeklyUsageUSD, 1e-9)
	require.InDelta(t, 107.8, rec.RecommendedWeeklyQuotaUSD, 1e-9)
	require.Contains(t, rec.Message, "3 天")

	// 无记录：返回 1 个 Plus 等价锚点
	rec = BuildDeclarationRecommendation(0, 0, 1.1)
	require.Equal(t, "anchor", rec.Basis)
	require.Equal(t, 0, rec.DaysWithRecords)
	require.InDelta(t, 120, rec.RecommendedWeeklyQuotaUSD, 1e-9)
	require.Contains(t, rec.Message, "锚点")

	// 非法缓冲系数回退为 1
	rec = BuildDeclarationRecommendation(70, 7, 0)
	require.InDelta(t, 70, rec.RecommendedWeeklyQuotaUSD, 1e-9)
}

// TestComputeCarpoolSettlementMembersMatchesAppendixA4 复算设计文档附录 A4：
// 10 人满车各申报 $240（周期 7 天，地板 192），8 人各用 $200，A 用 $120，B 用 $360。
func TestComputeCarpoolSettlementMembersMatchesAppendixA4(t *testing.T) {
	inputs := make([]CarpoolSettlementMemberInput, 0, 10)
	for i := 0; i < 10; i++ {
		actual := 200.0
		if i == 0 {
			actual = 120 // A 虚报
		} else if i == 9 {
			actual = 360 // B 超用
		}
		inputs = append(inputs, CarpoolSettlementMemberInput{
			UserID:                 int64(100 + i),
			Role:                   "member",
			DeclaredWeeklyQuotaUSD: 240,
			PrepaidAmountCNY:       140, // 400/10 + 1000×240/2400
			ActualUsageUSD:         actual,
			PeriodDays:             7,
		})
	}

	members := ComputeCarpoolSettlementMembers(2400, 400, 1000, 0.8, inputs)
	require.Len(t, members, 10)

	shareTotal := 0.0
	for _, m := range members {
		require.InDelta(t, 192, m.FloorUsageUSD, 1e-9)
		require.InDelta(t, 100, m.UsagePrepaidCNY, 1e-9)
		require.InDelta(t, 40, m.SeatFeePrepaidCNY, 1e-9)
		require.InDelta(t, 40, m.SeatFeeFinalCNY, 1e-9)
		require.InDelta(t, 0, m.SeatFeeDeltaCNY, 1e-9)
		shareTotal += m.UsageFinalShareCNY
	}
	// 恒等闭合：全车变动池收支恒等于 ¥1000
	require.InDelta(t, 1000, shareTotal, 1e-6)

	a := members[0]
	require.True(t, a.FloorTriggered)
	require.InDelta(t, 192, a.BillableUsageUSD, 1e-9)
	require.InDelta(t, 89.2, a.UsageFinalShareCNY, 0.05)
	require.InDelta(t, 10.8, a.UsageDeltaCNY, 0.05) // 退 ¥10.8

	other := members[1]
	require.False(t, other.FloorTriggered)
	require.InDelta(t, 200, other.BillableUsageUSD, 1e-9)
	require.InDelta(t, 92.9, other.UsageFinalShareCNY, 0.05)
	require.InDelta(t, 7.1, other.UsageDeltaCNY, 0.05) // 退 ¥7.1

	b := members[9]
	require.False(t, b.FloorTriggered)
	require.InDelta(t, 360, b.BillableUsageUSD, 1e-9)
	require.InDelta(t, 167.3, b.UsageFinalShareCNY, 0.05)
	require.InDelta(t, -67.3, b.UsageDeltaCNY, 0.05) // 补 ¥67.3
}

func TestComputeCarpoolSettlementMembersFloorExtrapolatesByPeriod(t *testing.T) {
	// 月度周期（30 天）地板 = 0.8×申报×(30/7)
	inputs := []CarpoolSettlementMemberInput{{
		UserID:                 1,
		DeclaredWeeklyQuotaUSD: 240,
		PrepaidAmountCNY:       140,
		ActualUsageUSD:         100,
		PeriodDays:             30,
	}}
	members := ComputeCarpoolSettlementMembers(2400, 400, 1000, 0.8, inputs)
	require.Len(t, members, 1)
	require.InDelta(t, 0.8*240*30/7, members[0].FloorUsageUSD, 1e-9)
	require.True(t, members[0].FloorTriggered)
	require.InDelta(t, members[0].FloorUsageUSD, members[0].BillableUsageUSD, 1e-9)
	// 唯一成员承担整个变动池
	require.InDelta(t, 1000, members[0].UsageFinalShareCNY, 1e-9)
}

func TestComputeCarpoolSettlementMembersZeroBillable(t *testing.T) {
	// 全员零申报零用量（遗留数据兜底）：分摊全 0，不出现除零
	inputs := []CarpoolSettlementMemberInput{{UserID: 1}, {UserID: 2}}
	members := ComputeCarpoolSettlementMembers(2400, 400, 1000, 0.8, inputs)
	require.Len(t, members, 2)
	for _, m := range members {
		require.Zero(t, m.UsageFinalShareCNY)
		require.Zero(t, m.BillableUsageUSD)
	}
}

func TestCarpoolFillDerivedMetrics(t *testing.T) {
	newCarpool := func(declaredTotal float64) *Carpool {
		return &Carpool{
			WeeklyLimitUSD:   CarpoolDefaultWeeklyLimitUSD,
			SeatFeeCNY:       CarpoolDefaultSeatFeeCNY,
			UsagePoolCNY:     CarpoolDefaultUsagePoolCNY,
			LaunchMaxRatio:   CarpoolDefaultLaunchMaxRatio,
			DeclaredTotalUSD: declaredTotal,
		}
	}

	// 满车 Σ=2400：均价 ¥70/Plus 等价（附录 A1）
	full := newCarpool(2400)
	full.FillDerivedMetrics()
	require.InDelta(t, 120, full.RemainingJoinableUSD, 1e-9)
	require.InDelta(t, 20, full.PlusEquivalents, 1e-9)
	require.InDelta(t, 70, full.AvgPriceCNY, 1e-9)

	// A3 不满车 Σ=2000：剩余 $520、16.7 Plus 等价、均价 ¥74
	partial := newCarpool(2000)
	partial.FillDerivedMetrics()
	require.InDelta(t, 520, partial.RemainingJoinableUSD, 1e-9)
	require.InDelta(t, 2000.0/120, partial.PlusEquivalents, 1e-9)
	require.InDelta(t, 74, partial.AvgPriceCNY, 0.01)

	// 空车：Plus 等价与均价为 0，剩余 = 105%×2400
	empty := newCarpool(0)
	empty.FillDerivedMetrics()
	require.InDelta(t, 2520, empty.RemainingJoinableUSD, 1e-9)
	require.Zero(t, empty.PlusEquivalents)
	require.Zero(t, empty.AvgPriceCNY)
}

type carpoolRepoStub struct {
	carpool  *Carpool
	rows     []CarpoolSettlementMemberRow
	joinErr  error
	joinCall int
}

func (s *carpoolRepoStub) List(ctx context.Context, userID int64) ([]Carpool, error) {
	panic("unexpected call")
}
func (s *carpoolRepoStub) GetByID(ctx context.Context, carpoolID, userID int64) (*Carpool, error) {
	return s.carpool, nil
}
func (s *carpoolRepoStub) GetByInvite(ctx context.Context, userID int64, tokenHash string) (*Carpool, error) {
	panic("unexpected call")
}
func (s *carpoolRepoStub) Create(ctx context.Context, ownerUserID int64, input CreateCarpoolInput, inviteHash, inviteHint string) (*CarpoolMutationResult, error) {
	panic("unexpected call")
}
func (s *carpoolRepoStub) CreateInvite(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool, inviteHash, inviteHint string) error {
	panic("unexpected call")
}
func (s *carpoolRepoStub) Join(ctx context.Context, carpoolID, userID int64, declaredWeeklyQuotaUSD float64, inviteHash *string) (*CarpoolMutationResult, error) {
	s.joinCall++
	return nil, s.joinErr
}
func (s *carpoolRepoStub) Launch(ctx context.Context, carpoolID, actorUserID int64, isAdmin, force bool) (*CarpoolMutationResult, error) {
	panic("unexpected call")
}
func (s *carpoolRepoStub) Cancel(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error {
	panic("unexpected call")
}
func (s *carpoolRepoStub) SetJoinLocked(ctx context.Context, carpoolID, actorUserID int64, locked bool) error {
	panic("unexpected call")
}
func (s *carpoolRepoStub) ListSettlementMembers(ctx context.Context, carpoolID int64) ([]CarpoolSettlementMemberRow, error) {
	return s.rows, nil
}
func (s *carpoolRepoStub) GetRecentWeeklyUsageStats(ctx context.Context, userID int64) (float64, int, error) {
	panic("unexpected call")
}

func settlementFixture() (*Carpool, []CarpoolSettlementMemberRow) {
	owner := int64(11)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 7)
	carpool := &Carpool{
		ID: 7, OwnerUserID: &owner, Status: "active",
		WeeklyLimitUSD: 2400, SeatFeeCNY: 400, UsagePoolCNY: 1000, ReserveRatio: 0.8,
	}
	rows := []CarpoolSettlementMemberRow{
		{UserID: 11, Role: "owner", DeclaredWeeklyQuotaUSD: 240, PrepaidAmountCNY: 140, ActualUsageUSD: 200, PeriodStart: &start, PeriodEnd: &end},
		{UserID: 12, Role: "member", DeclaredWeeklyQuotaUSD: 240, PrepaidAmountCNY: 140, ActualUsageUSD: 120, PeriodStart: &start, PeriodEnd: &end},
	}
	return carpool, rows
}

func TestGetSettlementVisibility(t *testing.T) {
	carpool, rows := settlementFixture()

	newSvc := func(memberRole *string) *CarpoolService {
		c := *carpool
		c.MemberRole = memberRole
		return NewCarpoolService(&carpoolRepoStub{carpool: &c, rows: rows}, nil)
	}

	ownerRole, memberRole := "owner", "member"

	// owner 见全车
	settlement, err := newSvc(&ownerRole).GetSettlement(context.Background(), 7, 11, false)
	require.NoError(t, err)
	require.True(t, settlement.FullView)
	require.Len(t, settlement.Members, 2)
	require.Equal(t, 2, settlement.MemberCount)
	require.NotNil(t, settlement.PeriodStart)
	require.InDelta(t, 192, settlement.Members[1].FloorUsageUSD, 1e-9)
	require.True(t, settlement.Members[1].FloorTriggered)

	// admin 见全车
	settlement, err = newSvc(nil).GetSettlement(context.Background(), 7, 99, true)
	require.NoError(t, err)
	require.True(t, settlement.FullView)
	require.Len(t, settlement.Members, 2)

	// 普通成员仅见自己
	settlement, err = newSvc(&memberRole).GetSettlement(context.Background(), 7, 12, false)
	require.NoError(t, err)
	require.False(t, settlement.FullView)
	require.Len(t, settlement.Members, 1)
	require.Equal(t, int64(12), settlement.Members[0].UserID)

	// 非成员非 admin 被拒
	_, err = newSvc(nil).GetSettlement(context.Background(), 7, 99, false)
	require.ErrorIs(t, err, ErrCarpoolForbidden)
}

func TestJoinRejectsNonPositiveDeclaration(t *testing.T) {
	stub := &carpoolRepoStub{}
	svc := NewCarpoolService(stub, nil)
	for _, declared := range []float64{0, -10} {
		_, err := svc.Join(context.Background(), 7, 12, declared)
		require.ErrorIs(t, err, ErrCarpoolInvalidRequest)
		_, err = svc.JoinByInvite(context.Background(), "token", 12, declared)
		require.ErrorIs(t, err, ErrCarpoolInvalidRequest)
	}
	require.Zero(t, stub.joinCall, "非法申报不应触达 repo 层")
}

func TestCreateAppliesQuotaDefaults(t *testing.T) {
	input := CreateCarpoolInput{}
	input.applyQuotaDefaults()
	require.Equal(t, CarpoolDefaultWeeklyLimitUSD, input.WeeklyLimitUSD)
	require.Equal(t, CarpoolDefaultSeatFeeCNY, input.SeatFeeCNY)
	require.Equal(t, CarpoolDefaultUsagePoolCNY, input.UsagePoolCNY)
	require.Equal(t, CarpoolDefaultReserveRatio, input.ReserveRatio)
	require.Equal(t, CarpoolDefaultLaunchMinRatio, input.LaunchMinRatio)
	require.Equal(t, CarpoolDefaultLaunchMaxRatio, input.LaunchMaxRatio)
	require.Equal(t, CarpoolTypeSmall, input.CarType)
	require.Equal(t, 1, input.Level)
}

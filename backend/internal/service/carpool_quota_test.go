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
	// §4.2（v3.2 公共池硬约束）：保底 r = 0.8×申报 写入 weekly_reserved_usd；
	// 公共池 C = 2400 − 0.8×Σ申报；个人周限额 = r + C（保留为个人绝对上限防呆）。
	// 新的数学不变式（替代旧版"组限额钉死总量"的错误前提）：
	//   1) Σr + C = 2400 —— 全车理论用量上界恰好等于上游周限：
	//      每人保底内用量 ≤ r，超额部分计入组级计数器且 Σ超额 < C，
	//      故 Σ用量 ≤ Σr + C = 2400；
	//   2) 105% 超募上限保证 C ≥ 16%×2400 > 0 —— 公共池不会消失；
	//   3) 保底内放行不依赖计数器（见 carpool_commons_test.go 的行为测试）。
	weeklyLimit, reserve := 2400.0, 0.8
	declared := []float64{400, 400, 200, 200, 200, 200, 200, 200, 100, 100, 100, 100} // A2 混合场景 Σ=2400
	total := 0.0
	for _, d := range declared {
		total += d
	}
	sharedPool := CarpoolSharedPoolCapacityUSD(weeklyLimit, reserve, total)
	require.InDelta(t, 480.0, sharedPool, 1e-9)

	reservedTotal := 0.0
	for i, d := range declared {
		r := CarpoolMemberReservedUSD(reserve, d)
		require.InDelta(t, reserve*d, r, 1e-9, "成员 %d 的保底应恰好等于 0.8×申报", i)
		reservedTotal += r
		limit := CarpoolMemberWeeklyLimitUSD(weeklyLimit, reserve, d, total)
		require.InDelta(t, r+sharedPool, limit, 1e-9, "成员 %d 的个人上限应为 r+C", i)
	}
	// 不变式 1：Σr + C 恰好钉死上游周限 2400（全车总用量的硬上界）。
	require.InDelta(t, weeklyLimit, reservedTotal+sharedPool, 1e-9)
}

func TestCarpoolSharedPoolInvariantHoldsAtRecruitmentBounds(t *testing.T) {
	// 发车区间 [95%, 105%]×2400 内，公共池恒为正且 Σr + C = 2400 恒成立。
	weeklyLimit, reserve := 2400.0, 0.8
	for _, total := range []float64{2280, 2400, 2520} {
		sharedPool := CarpoolSharedPoolCapacityUSD(weeklyLimit, reserve, total)
		require.Greater(t, sharedPool, 0.0, "Σ=%v 时公共池必须为正", total)
		require.InDelta(t, weeklyLimit, reserve*total+sharedPool, 1e-9, "Σ=%v 时 Σr+C 必须等于周限", total)
	}
	// 105% 超募上限处 C 恰为 16%×2400（公共池下界，保底机制不会失效）。
	require.InDelta(t, 0.16*weeklyLimit, CarpoolSharedPoolCapacityUSD(weeklyLimit, reserve, 1.05*weeklyLimit), 1e-9)
}

func TestCarpoolCommonsExcessDelta(t *testing.T) {
	const r = 100.0
	cases := []struct {
		name          string
		oldUsage      float64
		newUsage      float64
		expectedDelta float64
	}{
		{"跨界：保底内到保底外（r-10 → r+15）", r - 10, r + 15, 15},
		{"全程保底内（r-20 → r-5）", r - 20, r - 5, 0},
		{"恰好抵达保底（r-5 → r）", r - 5, r, 0},
		{"从保底出发越界（r → r+10）", r, r + 10, 10},
		{"全程保底外（r+5 → r+10）", r + 5, r + 10, 5},
		{"从零起步直接越界（0 → r+50）", 0, r + 50, 50},
		{"用量原地不动", r + 5, r + 5, 0},
		{"防御负增量（脏读/重置竞态）", r + 20, r + 5, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.InDelta(t, tc.expectedDelta, CarpoolCommonsExcessDelta(tc.oldUsage, tc.newUsage, r), 1e-9)
		})
	}
}

func TestCarpoolWeeklyWindowGridStart(t *testing.T) {
	anchor := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	week := 7 * 24 * time.Hour

	// 窗口未过期：保持原起点
	within := anchor.Add(3 * 24 * time.Hour)
	require.Equal(t, anchor, CarpoolWeeklyWindowGridStart(anchor, within))

	// 过期一次：推进到下一网格点
	atBoundary := anchor.Add(week)
	require.Equal(t, anchor.Add(week), CarpoolWeeklyWindowGridStart(anchor, atBoundary))

	// 过期多日但不足两周：仍只推进一格
	midSecondWeek := anchor.Add(week + 30*time.Hour)
	require.Equal(t, anchor.Add(week), CarpoolWeeklyWindowGridStart(anchor, midSecondWeek))

	// 跨多个周期（长期未用后归来）：吸附到当前所属网格点，而非"当天零点"
	afterLongIdle := anchor.Add(23*24*time.Hour + 5*time.Hour)
	got := CarpoolWeeklyWindowGridStart(anchor, afterLongIdle)
	require.Equal(t, anchor.Add(3*week), got)

	// 不变式：结果总是网格点，且满足 起点 ≤ now < 起点+7d
	require.Zero(t, got.Sub(anchor)%week)
	require.False(t, afterLongIdle.Before(got))
	require.True(t, afterLongIdle.Before(got.Add(week)))
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
	carpool    *Carpool
	rows       []CarpoolSettlementMemberRow
	joinErr    error
	joinCall   int
	joinResult *CarpoolMutationResult
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
func (s *carpoolRepoStub) Join(ctx context.Context, carpoolID, userID int64, declaredWeeklyQuotaUSD float64, joinedWechatGroup bool, inviteHash *string) (*CarpoolMutationResult, error) {
	s.joinCall++
	if s.joinErr != nil {
		return nil, s.joinErr
	}
	return s.joinResult, nil
}
func (s *carpoolRepoStub) Leave(ctx context.Context, carpoolID, userID int64) (*CarpoolMutationResult, error) {
	panic("unexpected call")
}
func (s *carpoolRepoStub) Confirm(ctx context.Context, carpoolID, ownerUserID int64) (*CarpoolMutationResult, error) {
	panic("unexpected call")
}
func (s *carpoolRepoStub) Unconfirm(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*CarpoolMutationResult, error) {
	panic("unexpected call")
}
func (s *carpoolRepoStub) ListPendingLaunch(ctx context.Context) ([]CarpoolPendingLaunch, error) {
	panic("unexpected call")
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
func (s *carpoolRepoStub) GetGroupQRCode(ctx context.Context, carpoolID int64) ([]byte, string, error) {
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
		return NewCarpoolService(&carpoolRepoStub{carpool: &c, rows: rows}, nil, nil, nil)
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
	svc := NewCarpoolService(stub, nil, nil, nil)
	for _, declared := range []float64{0, -10} {
		_, err := svc.Join(context.Background(), 7, 12, declared, true)
		require.ErrorIs(t, err, ErrCarpoolInvalidRequest)
		_, err = svc.JoinByInvite(context.Background(), "token", 12, declared, true)
		require.ErrorIs(t, err, ErrCarpoolInvalidRequest)
	}
	require.Zero(t, stub.joinCall, "非法申报不应触达 repo 层")
}

// 申报下限：低于 CarpoolMinDeclaredWeeklyQuotaUSD 直接拒绝。
// 没有下限时 $0.01 就能占一个席位并拿到公共池准入。
func TestJoinRejectsDeclarationBelowFloor(t *testing.T) {
	stub := &carpoolRepoStub{}
	svc := NewCarpoolService(stub, nil, nil, nil)
	_, err := svc.Join(context.Background(), 7, 12, 0.01, true)
	require.ErrorIs(t, err, ErrCarpoolDeclarationTooSmall)
	_, err = svc.JoinByInvite(context.Background(), "token", 12, CarpoolMinDeclaredWeeklyQuotaUSD-0.01, true)
	require.ErrorIs(t, err, ErrCarpoolDeclarationTooSmall)
	require.Zero(t, stub.joinCall, "低于下限的申报不应触达 repo 层")

	// 正好等于下限应放行到 repo 层。
	stub.joinResult = &CarpoolMutationResult{Carpool: &Carpool{ID: 7}}
	_, err = svc.Join(context.Background(), 7, 12, CarpoolMinDeclaredWeeklyQuotaUSD, true)
	require.NoError(t, err)
	require.Equal(t, 1, stub.joinCall)
}

// 额度参数校验：公共池必须严格为正，否则组级硬约束（C ≤ 0 时被跳过）静默失效。
func TestValidateQuotaParamsRejectsNonPositiveSharedPool(t *testing.T) {
	base := func() CreateCarpoolInput {
		in := CreateCarpoolInput{}
		in.applyQuotaDefaults()
		return in
	}

	ok := base()
	require.NoError(t, ok.validateQuotaParams())
	require.Greater(t, CarpoolSharedPoolCapacityUSD(ok.WeeklyLimitUSD, ok.ReserveRatio,
		ok.LaunchMaxRatio*ok.WeeklyLimitUSD), 0.0)

	// reserve×launch_max = 1 → C = 0 → 公共池检查被整体跳过。
	bad := base()
	bad.ReserveRatio = 1.0
	bad.LaunchMaxRatio = 1.0
	require.ErrorIs(t, bad.validateQuotaParams(), ErrCarpoolInvalidRequest)
	require.LessOrEqual(t, CarpoolSharedPoolCapacityUSD(bad.WeeklyLimitUSD, bad.ReserveRatio,
		bad.LaunchMaxRatio*bad.WeeklyLimitUSD), 0.0)

	// reserve×launch_max > 1 → C 为负。
	worse := base()
	worse.ReserveRatio = 0.9
	worse.LaunchMaxRatio = 1.5
	require.ErrorIs(t, worse.validateQuotaParams(), ErrCarpoolInvalidRequest)
}

// owner 申报超过整车上限的车永远进不了发车区间，会一直卡在 recruiting。
func TestValidateQuotaParamsRejectsOwnerDeclarationAboveCarCap(t *testing.T) {
	in := CreateCarpoolInput{}
	in.applyQuotaDefaults()
	in.DeclaredWeeklyQuotaUSD = in.LaunchMaxRatio*in.WeeklyLimitUSD + 1
	require.ErrorIs(t, in.validateQuotaParams(), ErrCarpoolQuotaExceeded)

	in.DeclaredWeeklyQuotaUSD = 1
	require.ErrorIs(t, in.validateQuotaParams(), ErrCarpoolDeclarationTooSmall)

	// 0 = 仅发起、不占额度，合法。
	in.DeclaredWeeklyQuotaUSD = 0
	require.NoError(t, in.validateQuotaParams())
}

// 非 admin 不得自助设定额度池参数（否则任何用户都能造出"整车 $10 亿"的车）。
func TestCreateRejectsCustomQuotaParamsForNonAdmin(t *testing.T) {
	stub := &carpoolRepoStub{}
	svc := NewCarpoolService(stub, nil, nil, nil)
	input := CreateCarpoolInput{
		Name: "whale-car", Visibility: CarpoolVisibilityPublic, AddedAdminWechat: true,
		WeeklyLimitUSD: 1e9,
	}
	_, err := svc.Create(context.Background(), 11, false, input)
	require.ErrorIs(t, err, ErrCarpoolCustomParamsForbidden)
}

// 咨询限流：冷却窗口内第二次预占被拒。
func TestReserveInterestSlotCooldown(t *testing.T) {
	svc := NewCarpoolService(&carpoolRepoStub{}, nil, nil, nil)
	require.NoError(t, svc.ReserveInterestSlot(12))
	require.ErrorIs(t, svc.ReserveInterestSlot(12), ErrCarpoolInterestTooFrequent)
	// 不同用户互不影响。
	require.NoError(t, svc.ReserveInterestSlot(13))
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

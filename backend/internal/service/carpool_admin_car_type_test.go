package service

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 指定车型是管理端能力：非 admin 传了 car_type 直接 403（与自定义额度参数同一
// 收口口径），且不能触达仓储。
func TestCarpoolCreateCarTypeForbiddenForNonAdmin(t *testing.T) {
	repo := &launchFlowRepoStub{}
	svc := newLaunchFlowService(repo, nil, nil)
	input := CreateCarpoolInput{
		Name: "manual-car", Visibility: CarpoolVisibilityPublic,
		CarType: intPtr(CarpoolCarTypeQuotaLegacy),
	}
	_, err := svc.Create(context.Background(), 11, false, input)
	require.ErrorIs(t, err, ErrCarpoolCustomParamsForbidden)
	require.Nil(t, repo.createInput, "非 admin 传 car_type 不应触达 repo 层")
}

// 非法车型值（不在 0-3）→ 400，且不能触达仓储。
func TestCarpoolCreateRejectsInvalidCarType(t *testing.T) {
	repo := &launchFlowRepoStub{}
	svc := newLaunchFlowService(repo, nil, nil)
	input := CreateCarpoolInput{
		Name: "manual-car", Visibility: CarpoolVisibilityPublic,
		CarType: intPtr(9),
	}
	_, err := svc.Create(context.Background(), 11, true, input)
	require.ErrorIs(t, err, ErrCarpoolInvalidRequest)
	require.Nil(t, repo.createInput)
}

// admin 创建各车型的默认参数：type 3（含缺省）用新计价 2400/50每人/1200/0.8，
// type 2 用存量参数 2400/400/1000/0.8。两型都走招募流程，三项强制确认不能少。
func TestCarpoolCreateCarTypeQuotaDefaults(t *testing.T) {
	png := "data:image/png;base64," + base64.StdEncoding.EncodeToString(testPNGBytes)
	cases := []struct {
		name           string
		carType        *int
		wantCarType    int
		wantSeatFee    float64
		wantUsagePool  float64
		wantReserve    float64
		wantWeeklyLmit float64
	}{
		{name: "default is type 3", carType: nil, wantCarType: CarpoolCarTypeQuotaV2, wantSeatFee: 50, wantUsagePool: 1200, wantReserve: 0.8, wantWeeklyLmit: 2400},
		{name: "explicit type 3", carType: intPtr(CarpoolCarTypeQuotaV2), wantCarType: CarpoolCarTypeQuotaV2, wantSeatFee: 50, wantUsagePool: 1200, wantReserve: 0.8, wantWeeklyLmit: 2400},
		{name: "type 2 legacy quota params", carType: intPtr(CarpoolCarTypeQuota), wantCarType: CarpoolCarTypeQuota, wantSeatFee: 400, wantUsagePool: 1000, wantReserve: 0.8, wantWeeklyLmit: 2400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner := int64(11)
			repo := &launchFlowRepoStub{createResult: &CarpoolMutationResult{Carpool: &Carpool{
				ID: 7, OwnerUserID: &owner, CarType: tc.wantCarType, Status: "recruiting",
				WeeklyLimitUSD: 2400, LaunchMaxRatio: 1.05,
			}}}
			svc := newLaunchFlowService(repo, nil, nil)
			input := CreateCarpoolInput{
				Name: "quota-car", Visibility: CarpoolVisibilityPublic, CarType: tc.carType,
				AddedAdminWechat: true, AcknowledgedRisk: true, GroupQRCode: png,
			}
			_, err := svc.Create(context.Background(), 11, true, input)
			require.NoError(t, err)
			require.NotNil(t, repo.createInput)
			require.NotNil(t, repo.createInput.CarType)
			require.Equal(t, tc.wantCarType, *repo.createInput.CarType)
			require.InDelta(t, tc.wantWeeklyLmit, repo.createInput.WeeklyLimitUSD, 1e-9)
			require.InDelta(t, tc.wantSeatFee, repo.createInput.SeatFeeCNY, 1e-9)
			require.InDelta(t, tc.wantUsagePool, repo.createInput.UsagePoolCNY, 1e-9)
			require.InDelta(t, tc.wantReserve, repo.createInput.ReserveRatio, 1e-9)
		})
	}
}

// type 1 手动车创建即生效：不强制三项确认（管理员微信/二维码/风险确认），
// 无池/席位/保底概念（恒写 0），owner 申报强制为 0，不生成邀请 token。
func TestCarpoolCreateDirectActiveCarType1(t *testing.T) {
	owner := int64(11)
	repo := &launchFlowRepoStub{createResult: &CarpoolMutationResult{Carpool: &Carpool{
		ID: 7, OwnerUserID: &owner, CarType: CarpoolCarTypeQuotaLegacy, Status: "active",
		WeeklyLimitUSD: 2400, LaunchMaxRatio: 1.05,
	}}}
	svc := newLaunchFlowService(repo, nil, nil)
	input := CreateCarpoolInput{
		Name: "legacy-car", Visibility: CarpoolVisibilityPublic,
		CarType: intPtr(CarpoolCarTypeQuotaLegacy),
		// 故意传入申报与池参数：手动车必须忽略（申报强制 0、池/席位/保底强制 0）。
		DeclaredWeeklyQuotaUSD: 500,
		SeatFeeCNY:             400,
		UsagePoolCNY:           1000,
		ReserveRatio:           0.8,
	}
	result, err := svc.Create(context.Background(), 11, true, input)
	require.NoError(t, err)
	require.NotNil(t, repo.createInput)
	require.Equal(t, CarpoolCarTypeQuotaLegacy, *repo.createInput.CarType)
	require.InDelta(t, 2400, repo.createInput.WeeklyLimitUSD, 1e-9)
	require.Zero(t, repo.createInput.SeatFeeCNY)
	require.Zero(t, repo.createInput.UsagePoolCNY)
	require.Zero(t, repo.createInput.ReserveRatio)
	require.Zero(t, repo.createInput.DeclaredWeeklyQuotaUSD, "手动车 owner 行恒 0 申报")
	require.Empty(t, repo.createInput.GroupQRCodeBytes, "手动车不强制二维码")
	require.Empty(t, result.InviteToken, "手动车不进招募，不应生成邀请")
}

// type 0 自定义规则车：rule_note 必填（人工结算依据）；三项强制确认同样不适用。
func TestCarpoolCreateDirectActiveCarType0(t *testing.T) {
	owner := int64(11)
	repo := &launchFlowRepoStub{createResult: &CarpoolMutationResult{Carpool: &Carpool{
		ID: 7, OwnerUserID: &owner, CarType: CarpoolCarTypeCustom, Status: "active",
		PricingModel: CarpoolPricingCustom, WeeklyLimitUSD: 2400, LaunchMaxRatio: 1.05,
	}}}
	svc := newLaunchFlowService(repo, nil, nil)

	// 缺 rule_note → 400，不触达仓储
	missing := CreateCarpoolInput{
		Name: "custom-car", Visibility: CarpoolVisibilityPublic,
		CarType: intPtr(CarpoolCarTypeCustom),
	}
	_, err := svc.Create(context.Background(), 11, true, missing)
	require.ErrorIs(t, err, ErrCarpoolInvalidRequest)
	require.Nil(t, repo.createInput)

	// 带 rule_note → 成功
	ok := CreateCarpoolInput{
		Name: "custom-car", Visibility: CarpoolVisibilityPublic,
		CarType:  intPtr(CarpoolCarTypeCustom),
		RuleNote: "每人每月 ¥200，按微信群里对账人工结算",
	}
	result, err := svc.Create(context.Background(), 11, true, ok)
	require.NoError(t, err)
	require.NotNil(t, repo.createInput)
	require.Equal(t, CarpoolCarTypeCustom, *repo.createInput.CarType)
	require.Equal(t, ok.RuleNote, repo.createInput.RuleNote)
	require.Empty(t, result.InviteToken)
}

// 代加成员仅 admin：非 admin 在打到仓储之前被挡下。
func TestCarpoolAddMemberRejectsNonAdmin(t *testing.T) {
	repo := &launchFlowRepoStub{}
	svc := adminOpsService(repo, &recordingSender{}, nil)
	_, err := svc.AddMember(context.Background(), 10, 1, false, AddCarpoolMemberInput{UserID: 2})
	require.ErrorIs(t, err, ErrCarpoolForbidden)
	require.Nil(t, repo.addMemberInput)
}

// type 2/3 申报必填：缺申报 400、低于下限 400，都在打到仓储之前挡住；
// 合法申报连同代录的风险确认一并下传。
func TestCarpoolAddMemberQuotaValidatesDeclaration(t *testing.T) {
	repo := &launchFlowRepoStub{
		carpoolByID:     &Carpool{ID: 10, CarType: CarpoolCarTypeQuotaV2, Status: "recruiting"},
		addMemberResult: &CarpoolMutationResult{Carpool: &Carpool{ID: 10, CarType: CarpoolCarTypeQuotaV2, Status: "recruiting", WeeklyLimitUSD: 2400, LaunchMaxRatio: 1.05}},
	}
	svc := adminOpsService(repo, &recordingSender{}, nil)
	ctx := context.Background()

	_, err := svc.AddMember(ctx, 10, 1, true, AddCarpoolMemberInput{UserID: 2})
	require.ErrorIs(t, err, ErrCarpoolInvalidRequest)
	_, err = svc.AddMember(ctx, 10, 1, true, AddCarpoolMemberInput{UserID: 2, DeclaredWeeklyQuotaUSD: CarpoolMinDeclaredWeeklyQuotaUSD - 1})
	require.ErrorIs(t, err, ErrCarpoolDeclarationTooSmall)
	require.Nil(t, repo.addMemberInput)

	_, err = svc.AddMember(ctx, 10, 1, true, AddCarpoolMemberInput{UserID: 2, DeclaredWeeklyQuotaUSD: 100, AcknowledgedRisk: true})
	require.NoError(t, err)
	require.NotNil(t, repo.addMemberInput)
	require.Equal(t, int64(2), repo.addMemberInput.UserID)
	require.InDelta(t, 100, repo.addMemberInput.DeclaredWeeklyQuotaUSD, 1e-9)
	require.True(t, repo.addMemberInput.AcknowledgedRisk, "代录的风险确认必须照存下传")
}

// 代加把 confirmed 车打出区间时，车主要收到「已退回招募中」通知。
func TestCarpoolAddMemberNotifiesOwnerWhenAutoUnconfirmed(t *testing.T) {
	owner := int64(7)
	repo := &launchFlowRepoStub{
		carpoolByID: &Carpool{ID: 10, CarType: CarpoolCarTypeQuotaV2, Status: "confirmed"},
		addMemberResult: &CarpoolMutationResult{
			Carpool: &Carpool{
				ID: 10, Name: "eve", OwnerUserID: &owner, CarType: CarpoolCarTypeQuotaV2,
				DeclaredTotalUSD: 1800, WeeklyLimitUSD: 2400, LaunchMinRatio: 0.95,
			},
			DeclaredWeeklyQuotaUSD: 100,
			AutoUnconfirmed:        true,
		},
	}
	sender := &recordingSender{}
	svc := adminOpsService(repo, sender, map[int64]*User{7: {ID: 7, Email: "owner@test.local"}})

	_, err := svc.AddMember(context.Background(), 10, 1, true, AddCarpoolMemberInput{UserID: 2, DeclaredWeeklyQuotaUSD: 100, AcknowledgedRisk: true})
	require.NoError(t, err)
	require.Equal(t, []string{"owner@test.local"}, sender.to)
	require.Contains(t, sender.subject[0], "退回招募中")
}

// 直接生效链路（type 1/0）：成员行落库后建订阅——weekly_limit_usd=车周限额、
// weekly_reserved_usd=NULL（无保底口径）、weekly_window_start 对齐当日 UTC 零点，
// 并把 subscription_id 回填成员行。
func TestCarpoolAddMemberDirectAssignsSubscription(t *testing.T) {
	groupID := int64(91)
	repo := &launchFlowRepoStub{
		carpoolByID:       &Carpool{ID: 10, CarType: CarpoolCarTypeQuotaLegacy, Status: "active", GroupID: &groupID, WeeklyLimitUSD: 2400},
		addMemberResult:   &CarpoolMutationResult{Carpool: &Carpool{ID: 10, CarType: CarpoolCarTypeQuotaLegacy, Status: "active", WeeklyLimitUSD: 2400, LaunchMaxRatio: 1.05}},
		directGroupID:     groupID,
		directWeeklyLimit: 2400,
	}
	groupRepo := &subscriptionGroupRepoStub{group: &Group{ID: groupID, SubscriptionType: SubscriptionTypeSubscription}}
	subRepo := newSubscriptionUserSubRepoStub()
	svc := adminOpsService(repo, &recordingSender{}, nil)
	svc.subscriptionService = NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)

	result, err := svc.AddMember(context.Background(), 10, 1, true, AddCarpoolMemberInput{UserID: 2})
	require.NoError(t, err)
	require.NotNil(t, result.Carpool)
	require.Equal(t, 1, subRepo.createCalls, "必须恰好建一份订阅")

	sub := subRepo.byID[1]
	require.NotNil(t, sub)
	require.Equal(t, int64(2), sub.UserID)
	require.Equal(t, groupID, sub.GroupID)
	require.NotNil(t, sub.WeeklyLimitUSD)
	require.InDelta(t, 2400, *sub.WeeklyLimitUSD, 1e-9, "订阅周限额 = 车周限额")
	require.Nil(t, sub.WeeklyReservedUSD, "type 1/0 无保底：weekly_reserved_usd 必须为 NULL")
	require.NotNil(t, sub.WeeklyWindowStart)
	require.Equal(t, time.Now().UTC().Format("2006-01-02"), sub.WeeklyWindowStart.Format("2006-01-02"),
		"周窗口起点对齐当日 UTC 零点")
	require.Equal(t, time.UTC, sub.WeeklyWindowStart.Location())
	require.NotNil(t, sub.AssignedBy)
	require.Equal(t, int64(1), *sub.AssignedBy)
	require.InDelta(t, 30, sub.ExpiresAt.Sub(sub.StartsAt).Hours()/24, 1)
	require.Equal(t, sub.ID, repo.boundSubscription, "subscription_id 必须回填成员行")
}

// 订阅创建失败时补偿：把刚落的成员行退回 left，不留「在车上却没订阅」的僵尸席位。
func TestCarpoolAddMemberDirectCompensatesOnAssignFailure(t *testing.T) {
	groupID := int64(91)
	repo := &launchFlowRepoStub{
		carpoolByID:       &Carpool{ID: 10, CarType: CarpoolCarTypeQuotaLegacy, Status: "active", GroupID: &groupID, WeeklyLimitUSD: 2400},
		addMemberResult:   &CarpoolMutationResult{Carpool: &Carpool{ID: 10, CarType: CarpoolCarTypeQuotaLegacy, Status: "active", WeeklyLimitUSD: 2400}},
		directGroupID:     groupID,
		directWeeklyLimit: 2400,
	}
	// 非订阅类型分组 → AssignOrExtendSubscription 必然失败。
	groupRepo := &subscriptionGroupRepoStub{group: &Group{ID: groupID, SubscriptionType: SubscriptionTypeStandard}}
	subRepo := newSubscriptionUserSubRepoStub()
	svc := adminOpsService(repo, &recordingSender{}, nil)
	svc.subscriptionService = NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)

	_, err := svc.AddMember(context.Background(), 10, 1, true, AddCarpoolMemberInput{UserID: 2})
	require.ErrorIs(t, err, ErrGroupNotSubscriptionType)
	require.Equal(t, int64(2), repo.removedDirectUser, "失败必须补偿退回成员行")
	require.Contains(t, repo.removedDirectCause, "subscription")
	require.Zero(t, repo.boundSubscription)
}

// subscription_id 回填失败同样补偿，并返回错误（孤儿订阅由日志人工清理）。
func TestCarpoolAddMemberDirectCompensatesOnBindFailure(t *testing.T) {
	groupID := int64(91)
	repo := &launchFlowRepoStub{
		carpoolByID:       &Carpool{ID: 10, CarType: CarpoolCarTypeCustom, Status: "active", GroupID: &groupID, WeeklyLimitUSD: 2400},
		addMemberResult:   &CarpoolMutationResult{Carpool: &Carpool{ID: 10, CarType: CarpoolCarTypeCustom, Status: "active", PricingModel: CarpoolPricingCustom, WeeklyLimitUSD: 2400}},
		directGroupID:     groupID,
		directWeeklyLimit: 2400,
		bindErr:           ErrCarpoolNotMember,
	}
	groupRepo := &subscriptionGroupRepoStub{group: &Group{ID: groupID, SubscriptionType: SubscriptionTypeSubscription}}
	subRepo := newSubscriptionUserSubRepoStub()
	svc := adminOpsService(repo, &recordingSender{}, nil)
	svc.subscriptionService = NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)

	_, err := svc.AddMember(context.Background(), 10, 1, true, AddCarpoolMemberInput{UserID: 2})
	require.ErrorIs(t, err, ErrCarpoolNotMember)
	require.Equal(t, int64(2), repo.removedDirectUser)
}

// 手动车必须在 active 状态才能代加：recruiting 的手动老车（升级前遗留）直接 409。
func TestCarpoolAddMemberDirectRequiresActive(t *testing.T) {
	repo := &launchFlowRepoStub{
		carpoolByID: &Carpool{ID: 10, CarType: CarpoolCarTypeQuotaLegacy, Status: "recruiting"},
	}
	svc := adminOpsService(repo, &recordingSender{}, nil)
	_, err := svc.AddMember(context.Background(), 10, 1, true, AddCarpoolMemberInput{UserID: 2})
	require.ErrorIs(t, err, ErrCarpoolUnavailable)
	require.Nil(t, repo.addMemberInput)
}

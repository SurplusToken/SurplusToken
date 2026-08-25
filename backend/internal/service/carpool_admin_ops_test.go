package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func adminOpsService(repo *launchFlowRepoStub, sender *recordingSender, users map[int64]*User) *CarpoolService {
	return newLaunchFlowService(repo, sender, &stubUserDirectory{users: users})
}

// 管理端入口全部以 isAdmin 为唯一闸门。非管理员必须在打到仓储之前就被挡下——
// 否则一个普通用户就能把别人踢下车。
// 唯一例外是 UpdateMemberQuota：成员可自助改自己的申报（见下方专项用例），
// 这里断言的是「非 admin 改别人」仍然 403。
func TestCarpoolAdminOpsRejectNonAdmin(t *testing.T) {
	repo := &launchFlowRepoStub{}
	svc := adminOpsService(repo, &recordingSender{}, nil)
	ctx := context.Background()

	_, err := svc.ListForAdmin(ctx, 1, false)
	require.ErrorIs(t, err, ErrCarpoolForbidden)
	_, err = svc.RemoveMember(ctx, 10, 2, 1, false)
	require.ErrorIs(t, err, ErrCarpoolForbidden)
	_, err = svc.UpdateMemberQuota(ctx, 10, 2, 1, false, 100)
	require.ErrorIs(t, err, ErrCarpoolForbidden)
	_, err = svc.UpdateCarpool(ctx, 10, 1, false, UpdateCarpoolInput{})
	require.ErrorIs(t, err, ErrCarpoolForbidden)
	_, err = svc.TransferOwner(ctx, 10, 2, 1, false)
	require.ErrorIs(t, err, ErrCarpoolForbidden)

	require.Zero(t, repo.removedMemberID)
	require.Zero(t, repo.updatedQuota)
	require.Zero(t, repo.transferredTo)
	require.Nil(t, repo.updateInput)
}

// 被移出的人必须收到通知：他申报的额度被释放了，不能让他从别处才知道。
func TestCarpoolRemoveMemberNotifiesRemovedUser(t *testing.T) {
	owner := int64(7)
	repo := &launchFlowRepoStub{removeResult: &CarpoolMutationResult{
		Carpool:                &Carpool{ID: 10, Name: "eve", OwnerUserID: &owner},
		DeclaredWeeklyQuotaUSD: 250,
	}}
	sender := &recordingSender{}
	svc := adminOpsService(repo, sender, map[int64]*User{
		2: {ID: 2, Email: "kicked@test.local"},
		7: {ID: 7, Email: "owner@test.local"},
	})

	_, err := svc.RemoveMember(context.Background(), 10, 2, 1, true)
	require.NoError(t, err)
	require.Equal(t, int64(2), repo.removedMemberID)
	// 没跌破发车线就只通知被移除的人，别去打扰车主。
	require.Equal(t, []string{"kicked@test.local"}, sender.to)
	require.Contains(t, sender.subject[0], "eve")
	require.Contains(t, sender.body[0], "250")
}

// 踢人把 Σ申报 打出发车区间时，车主也要收到「已退回招募中」——
// 否则他会一直等一辆确认过却永远发不出去的车。
func TestCarpoolRemoveMemberNotifiesOwnerWhenAutoUnconfirmed(t *testing.T) {
	owner := int64(7)
	repo := &launchFlowRepoStub{removeResult: &CarpoolMutationResult{
		Carpool: &Carpool{
			ID: 10, Name: "eve", OwnerUserID: &owner,
			DeclaredTotalUSD: 1800, WeeklyLimitUSD: 2400, LaunchMinRatio: 0.95,
		},
		DeclaredWeeklyQuotaUSD: 250,
		AutoUnconfirmed:        true,
	}}
	sender := &recordingSender{}
	svc := adminOpsService(repo, sender, map[int64]*User{
		2: {ID: 2, Email: "kicked@test.local"},
		7: {ID: 7, Email: "owner@test.local"},
	})

	_, err := svc.RemoveMember(context.Background(), 10, 2, 1, true)
	require.NoError(t, err)
	require.Equal(t, []string{"kicked@test.local", "owner@test.local"}, sender.to)
	require.Contains(t, sender.subject[1], "退回招募中")
	// 正文要说清发车线是多少，车主才知道还差多少
	require.Contains(t, sender.body[1], "95")
}

// 邮件链路挂掉不能影响主流程：移除本身已经在仓储里提交了。
func TestCarpoolRemoveMemberSurvivesEmailFailure(t *testing.T) {
	owner := int64(7)
	repo := &launchFlowRepoStub{removeResult: &CarpoolMutationResult{
		Carpool: &Carpool{ID: 10, Name: "eve", OwnerUserID: &owner},
	}}
	svc := adminOpsService(repo, &recordingSender{fail: true}, map[int64]*User{
		2: {ID: 2, Email: "kicked@test.local"},
	})

	result, err := svc.RemoveMember(context.Background(), 10, 2, 1, true)
	require.NoError(t, err)
	require.NotNil(t, result)
}

// 招募期成员可自助修改自己的申报额度：非 admin 改自己放行到仓储；改别人 403 且
// 不触达仓储；admin 代改任何人不受影响（回归）。
func TestCarpoolUpdateMemberQuotaSelfService(t *testing.T) {
	repo := &launchFlowRepoStub{quotaResult: &CarpoolMutationResult{Carpool: &Carpool{ID: 10}}}
	svc := adminOpsService(repo, &recordingSender{}, nil)
	ctx := context.Background()

	// 非 admin 改自己 → 放行
	_, err := svc.UpdateMemberQuota(ctx, 10, 2, 2, false, 100)
	require.NoError(t, err)
	require.Equal(t, 100.0, repo.updatedQuota)

	// 非 admin 改别人 → 403，不触达仓储
	repo.updatedQuota = 0
	_, err = svc.UpdateMemberQuota(ctx, 10, 2, 1, false, 100)
	require.ErrorIs(t, err, ErrCarpoolForbidden)
	require.Zero(t, repo.updatedQuota)

	// admin 改别人 → 仍成功（回归）
	_, err = svc.UpdateMemberQuota(ctx, 10, 2, 1, true, 100)
	require.NoError(t, err)
	require.Equal(t, 100.0, repo.updatedQuota)
}

// 代改申报的下限与用户自己上车时一致，并且要在打到仓储之前挡住。
func TestCarpoolUpdateMemberQuotaEnforcesMinimum(t *testing.T) {
	repo := &launchFlowRepoStub{}
	svc := adminOpsService(repo, &recordingSender{}, nil)

	_, err := svc.UpdateMemberQuota(context.Background(), 10, 2, 1, true,
		CarpoolMinDeclaredWeeklyQuotaUSD-1)
	require.ErrorIs(t, err, ErrCarpoolDeclarationTooSmall)
	require.Zero(t, repo.updatedQuota)
}

// 改车信息：名称去空白后不能为空，可见性只认两个合法值，合法输入要把名称 trim 后下传。
func TestCarpoolUpdateCarpoolValidatesInput(t *testing.T) {
	repo := &launchFlowRepoStub{updateResult: &CarpoolMutationResult{Carpool: &Carpool{ID: 10}}}
	svc := adminOpsService(repo, &recordingSender{}, nil)
	ctx := context.Background()

	blank := "   "
	_, err := svc.UpdateCarpool(ctx, 10, 1, true, UpdateCarpoolInput{Name: &blank})
	require.ErrorIs(t, err, ErrCarpoolInvalidRequest)

	bad := "secret"
	_, err = svc.UpdateCarpool(ctx, 10, 1, true, UpdateCarpoolInput{Visibility: &bad})
	require.ErrorIs(t, err, ErrCarpoolInvalidRequest)
	require.Nil(t, repo.updateInput)

	name := "  新车名  "
	_, err = svc.UpdateCarpool(ctx, 10, 1, true, UpdateCarpoolInput{Name: &name})
	require.NoError(t, err)
	require.NotNil(t, repo.updateInput)
	require.Equal(t, "新车名", *repo.updateInput.Name)
}

// 总览必须把派生指标算好再返回：List 走的是同一套 FillDerivedMetrics，
// 漏掉的话管理页里剩余可预约、进度条全是 0。
func TestCarpoolListForAdminFillsDerivedMetrics(t *testing.T) {
	repo := &launchFlowRepoStub{allCarpools: []Carpool{{
		ID: 10, WeeklyLimitUSD: 2400, DeclaredTotalUSD: 1200,
		LaunchMaxRatio: 1.05, SeatFeeCNY: 400, UsagePoolCNY: 1000,
	}}}
	svc := adminOpsService(repo, &recordingSender{}, nil)

	items, err := svc.ListForAdmin(context.Background(), 1, true)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.InDelta(t, 1.05*2400-1200, items[0].RemainingJoinableUSD, 1e-9)
}

// 转让车主直接透传到仓储（成员校验在仓储的事务里做，避免 TOCTOU）。
func TestCarpoolTransferOwnerPassesThrough(t *testing.T) {
	repo := &launchFlowRepoStub{transferResult: &CarpoolMutationResult{Carpool: &Carpool{ID: 10}}}
	svc := adminOpsService(repo, &recordingSender{}, nil)

	_, err := svc.TransferOwner(context.Background(), 10, 42, 1, true)
	require.NoError(t, err)
	require.Equal(t, int64(42), repo.transferredTo)
}

// 邮箱只对 admin 输出：管理员在成员管理里要直接联系到人；普通成员与邀请持有者
// 看到的同车名单不该带出邮箱（与结算单 FullView 同一信任边界）。
func TestCarpoolRosterEmailOnlyForAdmin(t *testing.T) {
	memberRole := "member"
	repo := &launchFlowRepoStub{
		carpoolByID: &Carpool{ID: 10, Name: "eve", MemberRole: &memberRole},
		settlementMembers: []CarpoolSettlementMemberRow{
			{UserID: 2, Username: "bob", Email: "bob@test.local", Role: "member", DeclaredWeeklyQuotaUSD: 400},
		},
	}
	svc := adminOpsService(repo, &recordingSender{}, nil)

	// 普通成员：能看名单，但 email 必须为空
	roster, err := svc.GetRoster(context.Background(), 10, 2, false, "")
	require.NoError(t, err)
	require.Len(t, roster, 1)
	require.Empty(t, roster[0].Email)

	// admin：同一批数据，email 必须带出来
	roster, err = svc.GetRoster(context.Background(), 10, 1, true, "")
	require.NoError(t, err)
	require.Len(t, roster, 1)
	require.Equal(t, "bob@test.local", roster[0].Email)
}

// 更换群二维码：非图片在打到仓储之前就被拒；合法图片解析出字节与内容类型后下传
// （车主/admin 的权限闸在仓储锁内复核，与 CreateInvite 同一处）。
func TestCarpoolReplaceGroupQRCodeValidatesAndForwards(t *testing.T) {
	repo := &launchFlowRepoStub{}
	svc := adminOpsService(repo, &recordingSender{}, nil)

	err := svc.ReplaceGroupQRCode(context.Background(), 10, 7, false, "not-an-image")
	require.ErrorIs(t, err, ErrCarpoolGroupQRCodeInvalid)
	require.Nil(t, repo.qrReplaceData)

	err = svc.ReplaceGroupQRCode(context.Background(), 10, 7, false, "data:image/png;base64,iVBORw0KGgo=")
	require.NoError(t, err)
	require.NotEmpty(t, repo.qrReplaceData)
	require.Equal(t, "image/png", repo.qrReplaceType)
}

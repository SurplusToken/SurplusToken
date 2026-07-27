package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 自定义规则车（含平台升级前建立的老车）的结算单只列实际用量，不出任何退补。
//
// 这些车的成员申报恒为 0（migration 187 给存量行填的默认值），套用额度预约制
// 的地板/变动池分摊会凭空算出每人几百块的"补款"——一笔他们从没同意过的账。
func TestGetSettlementCustomRuleCarIsUsageOnly(t *testing.T) {
	carpool, rows := settlementFixture()
	custom := *carpool
	custom.PricingModel = CarpoolPricingCustom
	custom.RuleNote = "旧版席位规则：共 5 席，基础费 ¥130/席。"

	settlement, err := NewCarpoolService(&carpoolRepoStub{carpool: &custom, rows: rows}, nil, nil, nil).
		GetSettlement(context.Background(), 7, 11, true)
	require.NoError(t, err)

	require.True(t, settlement.ManualSettlement)
	require.Equal(t, CarpoolPricingCustom, settlement.PricingModel)
	require.Contains(t, settlement.RuleNote, "旧版席位规则")
	require.False(t, settlement.CanSettle)
	require.Equal(t, "manual_settlement", settlement.SettleBlockedFor)

	require.Len(t, settlement.Members, 2)
	for _, m := range settlement.Members {
		require.Greater(t, m.ActualUsageUSD, 0.0, "实际用量要给——车主按自己那套规则分账正需要它")
		require.Zero(t, m.TotalDeltaCNY, "不得出现任何退补金额")
		require.Zero(t, m.UsageFinalShareCNY)
		require.Zero(t, m.SeatFeeFinalCNY)
		require.Zero(t, m.BillableUsageUSD)
		require.Zero(t, m.FloorUsageUSD)
	}
}

// 普通成员在自定义规则车里仍然只看得到自己那一行。
func TestGetSettlementCustomRuleCarRespectsVisibility(t *testing.T) {
	carpool, rows := settlementFixture()
	memberRole := "member"
	custom := *carpool
	custom.PricingModel = CarpoolPricingCustom
	custom.MemberRole = &memberRole

	settlement, err := NewCarpoolService(&carpoolRepoStub{carpool: &custom, rows: rows}, nil, nil, nil).
		GetSettlement(context.Background(), 7, 12, false)
	require.NoError(t, err)
	require.True(t, settlement.ManualSettlement)
	require.Len(t, settlement.Members, 1)
	require.Equal(t, int64(12), settlement.Members[0].UserID)
}

// 自定义规则车不能走自动结算冻结。
func TestSettleCarpoolRejectsCustomRuleCar(t *testing.T) {
	carpool, rows := settlementFixture()
	custom := *carpool
	custom.PricingModel = CarpoolPricingCustom

	_, err := NewCarpoolService(&carpoolRepoStub{carpool: &custom, rows: rows}, nil, nil, nil).
		SettleCarpool(context.Background(), 7, 11, false)
	require.Error(t, err, "人工结算的车没有可冻结的自动结果")
}

// 派生指标（剩余可预约 / Plus 等价 / 均价）对自定义规则车不成立，必须清零，
// 否则卡片会显示 "0 / 2400"、"均价 ¥0" 这类误导数字。
func TestFillCarpoolPresentationSkipsDerivedMetricsForCustomRule(t *testing.T) {
	quota := &Carpool{
		WeeklyLimitUSD: 2400, SeatFeeCNY: 400, UsagePoolCNY: 1000,
		LaunchMaxRatio: 1.05, DeclaredTotalUSD: 1200,
	}
	fillCarpoolPresentation(quota)
	require.Greater(t, quota.RemainingJoinableUSD, 0.0)
	require.Greater(t, quota.PlusEquivalents, 0.0)
	require.Greater(t, quota.AvgPriceCNY, 0.0)

	custom := &Carpool{
		PricingModel:   CarpoolPricingCustom,
		WeeklyLimitUSD: 2400, SeatFeeCNY: 400, UsagePoolCNY: 1000,
		LaunchMaxRatio: 1.05, DeclaredTotalUSD: 1200,
	}
	fillCarpoolPresentation(custom)
	require.Zero(t, custom.RemainingJoinableUSD)
	require.Zero(t, custom.PlusEquivalents)
	require.Zero(t, custom.AvgPriceCNY)
	require.Zero(t, custom.DeclaredTotalUSD)
	require.Equal(t, CarpoolAdminWechatID, custom.AdminWechat, "管理员微信仍要填")
}

// IsQuotaModel：空值按额度制处理（老数据/未迁移库的安全默认）。
func TestIsQuotaModelDefaults(t *testing.T) {
	require.True(t, (&Carpool{}).IsQuotaModel(), "空 pricing_model 视为额度制")
	require.True(t, (&Carpool{PricingModel: CarpoolPricingQuota}).IsQuotaModel())
	require.False(t, (&Carpool{PricingModel: CarpoolPricingCustom}).IsQuotaModel())
}

// 冻结快照不该被套用到人工结算的车上（防御：即使库里误留了 settled_at）。
func TestCustomRuleCarIgnoresFrozenSnapshot(t *testing.T) {
	carpool, rows := settlementFixture()
	settledAt := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	custom := *carpool
	custom.PricingModel = CarpoolPricingCustom
	custom.SettledAt = &settledAt
	rows[0].Frozen = &CarpoolSettlementFrozenRow{TotalDeltaCNY: -999}

	settlement, err := NewCarpoolService(&carpoolRepoStub{carpool: &custom, rows: rows}, nil, nil, nil).
		GetSettlement(context.Background(), 7, 11, true)
	require.NoError(t, err)
	require.True(t, settlement.ManualSettlement)
	for _, m := range settlement.Members {
		require.Zero(t, m.TotalDeltaCNY)
	}
}

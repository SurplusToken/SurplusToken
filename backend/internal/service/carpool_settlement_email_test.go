package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func emailFixture() (*Carpool, *CarpoolSettlement) {
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	start, end := base, base.AddDate(0, 0, 28)
	carpool := &Carpool{
		ID: 7, Name: "周末车", WeeklyLimitUSD: 2400,
		SeatFeeCNY: 400, UsagePoolCNY: 1000, ReserveRatio: 0.8,
	}
	settlement := &CarpoolSettlement{
		CarpoolID: 7, SeatFeeCNY: 400, UsagePoolCNY: 1000,
		MemberCount: 1, PeriodStart: &start, PeriodEnd: &end,
		Members: []CarpoolSettlementMember{{
			UserID: 12, Email: "alice@example.com", Username: "alice", Role: "member",
			DeclaredWeeklyQuotaUSD: 240,
			CycleBased:             true,
			CycleCount:             4,
			FloorCycles:            2,
			Cycles: []CarpoolBillingCycle{
				cycle(base, 240, 192, 300),
				cycle(base.AddDate(0, 0, 7), 240, 192, 300),
				cycle(base.AddDate(0, 0, 14), 240, 192, 50),
				cycle(base.AddDate(0, 0, 21), 240, 192, 50),
			},
			ActualUsageUSD:     700,
			BillableUsageUSD:   984,
			FloorUsageUSD:      768,
			QuotedPrepaidCNY:   140,
			SeatFeeFinalCNY:    400,
			UsageFinalShareCNY: 1000,
			TotalDeltaCNY:      -1260,
		}},
	}
	return carpool, settlement
}

// 邮件必须逐周期列明：申报 / 实际 / 计入月消费，并给出月度合计与两部分费用。
func TestBuildCarpoolSettlementEmailHasPerCycleBreakdown(t *testing.T) {
	carpool, settlement := emailFixture()
	subject, body := BuildCarpoolSettlementEmail(carpool, settlement)

	require.Contains(t, subject, "周末车")
	require.Contains(t, subject, "已结束")
	require.Contains(t, subject, "1400.00", "标题要带账单合计 = 席位费 400 + 变动池 1000")

	// 计费周期数与地板周期数
	require.Contains(t, body, "共 <b>4</b> 个计费周期")
	require.Contains(t, body, "2 个周期实际用量不足申报的保底")

	// 四个周期各自一行：申报 240、实际、计入月消费
	require.Contains(t, body, "240.00")
	require.Contains(t, body, "300.00")
	require.Contains(t, body, "192.00", "被地板托底的周期按 192 计入")
	require.Contains(t, body, "（按保底计）")

	// 月度合计
	require.Contains(t, body, "700.00", "月度实际合计")
	require.Contains(t, body, "984.00", "月度计费合计 = Σ max(实际, 保底)")

	// 两部分费用与合计
	require.Contains(t, body, "席位费 <b>¥400.00</b>")
	require.Contains(t, body, "变动池分摊 <b>¥1000.00</b>")
	require.Contains(t, body, "¥1400.00")
	require.Contains(t, body, "应<b>补</b> ¥1260.00")

	// 成员身份可辨认
	require.Contains(t, body, "alice")
	require.Contains(t, body, "alice@example.com")
	require.Contains(t, body, "#12")
}

// 车名进 HTML 必须转义——车名是车主自由输入的。
func TestBuildCarpoolSettlementEmailEscapesCarpoolName(t *testing.T) {
	carpool, settlement := emailFixture()
	carpool.Name = `<img src=x onerror=alert(1)>`
	subject, body := BuildCarpoolSettlementEmail(carpool, settlement)

	require.NotContains(t, body, "<img src=x")
	require.Contains(t, body, "&lt;img src=x")
	require.NotContains(t, subject, "<img src=x")
}

// 无分周期台账的成员（老车）必须显式说明口径不同，不能默默按月算了事。
func TestBuildCarpoolSettlementEmailFlagsMonthlyFallback(t *testing.T) {
	carpool, settlement := emailFixture()
	settlement.Members[0].CycleBased = false
	settlement.Members[0].Cycles = nil

	_, body := BuildCarpoolSettlementEmail(carpool, settlement)
	require.Contains(t, body, "无分周期台账")
	require.Contains(t, body, "超用的周会补贴没用满的周")
	require.NotContains(t, body, "个计费周期，其中")
}

// 自定义规则车不发结算邮件——它们的账不由平台计算。
func TestNotifyCarpoolSettlementSkipsManualSettlement(t *testing.T) {
	sender := &recordingSender{}
	svc := newLaunchFlowService(&launchFlowRepoStub{}, sender, &stubUserDirectory{})
	carpool, settlement := emailFixture()
	settlement.ManualSettlement = true

	svc.NotifyCarpoolSettlement(context.Background(), carpool, settlement)
	require.Empty(t, sender.to)
}

// 正常结算发给运营联系人，且只发这一个地址。
func TestNotifyCarpoolSettlementGoesToCarpoolAdmin(t *testing.T) {
	sender := &recordingSender{}
	dir := &stubUserDirectory{admins: []User{{ID: 1, Email: "other-admin@example.com"}}}
	svc := newLaunchFlowService(&launchFlowRepoStub{}, sender, dir)
	carpool, settlement := emailFixture()

	svc.NotifyCarpoolSettlement(context.Background(), carpool, settlement)
	require.Equal(t, []string{CarpoolAdminEmail}, sender.to)
	require.Len(t, sender.body, 1)
	require.True(t, strings.Contains(sender.subject[0], "已结束"))
}

// type 3 新计价车的结算邮件：席位费每人固定（全车合计 = 50×人数），不按人数均摊。
func TestBuildCarpoolSettlementEmailType3SeatFeePerMember(t *testing.T) {
	carpool, settlement := emailFixture()
	carpool.CarType = CarpoolCarTypeQuotaV2
	carpool.WeeklyLimitUSD = 2800
	carpool.SeatFeeCNY = 50
	carpool.UsagePoolCNY = 1200
	settlement.CarType = CarpoolCarTypeQuotaV2
	settlement.SeatFeeCNY = 50
	settlement.UsagePoolCNY = 1200
	settlement.MemberCount = 3
	settlement.Members[0].SeatFeeFinalCNY = 50
	settlement.Members[0].UsageFinalShareCNY = 400

	subject, body := BuildCarpoolSettlementEmail(carpool, settlement)
	require.Contains(t, subject, "1350.00", "账单合计 = 席位费 50×3 + 变动池 1200")
	require.Contains(t, body, "席位费 ¥150.00")
	require.Contains(t, body, "席位费每人固定")
	require.NotContains(t, body, "按发车人数均摊")
	require.Contains(t, body, "席位费 <b>¥50.00</b>", "成员行是每人固定的 50，不是 150/3")
}

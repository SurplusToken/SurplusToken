package service

import (
	"context"
	"fmt"
	"html"
	"strings"
)

// BuildCarpoolSettlementEmail 生成期末结算通知邮件（发给拼车运营联系人）。
//
// 明细逐层展开，便于人工核账：
//
//	车 → 每位成员 → 每个计费周期（申报 / 实际 / 计入月消费）
//	              → 月度合计计费用量
//	              → 席位费 + 变动池两部分金额与合计
//
// "计入月消费"就是该周期的 max(实际用量, 80%×申报)——不足申报 80% 的按 80% 记。
// 这一步必须按周做，按月整体算会让超用的周补贴没用满的周（见 migration 193）。
func BuildCarpoolSettlementEmail(carpool *Carpool, settlement *CarpoolSettlement) (subject, body string) {
	if carpool == nil || settlement == nil {
		return "", ""
	}
	safeName := carpoolDisplayName(carpool.Name)
	subject = fmt.Sprintf("拼车「%s」已结束，账单合计 ¥%.2f", safeName, settlementGrandTotalCNY(settlement))

	var b strings.Builder
	fmt.Fprintf(&b, `<p>拼车「%s」（ID %d）本期已结束，结算单如下。</p>`, safeName, carpool.ID)
	fmt.Fprintf(&b, `<p><b>账单合计：¥%.2f</b>（席位费 ¥%.2f + 变动池 ¥%.2f）</p>`,
		settlementGrandTotalCNY(settlement), carpoolSeatFeeTotalCNY(settlement), carpool.UsagePoolCNY)
	if settlement.PeriodStart != nil && settlement.PeriodEnd != nil {
		fmt.Fprintf(&b, `<p>计费区间：%s ～ %s</p>`,
			settlement.PeriodStart.Format("2006-01-02"), settlement.PeriodEnd.Format("2006-01-02"))
	}
	fmt.Fprintf(&b, `<p>成员 %d 人；保底比例 %.0f%%（不足申报额 %.0f%% 的周期按 %.0f%% 计入月消费）。</p>`,
		settlement.MemberCount, carpool.ReserveRatio*100, carpool.ReserveRatio*100, carpool.ReserveRatio*100)

	for _, m := range settlement.Members {
		writeMemberSection(&b, m)
	}

	seatFeeNote := "席位费按发车人数均摊。"
	if settlement.CarType == CarpoolCarTypeQuotaV2 {
		seatFeeNote = "席位费每人固定，不按人数均摊。"
	}
	_, _ = b.WriteString(`<hr><p style="color:#888;font-size:12px">` +
		`计入月消费 = max(该周期实际用量, ` + fmt.Sprintf("%.0f%%", carpool.ReserveRatio*100) + `×申报额)，逐周期计算后求和。<br>` +
		`变动池按各人计费用量占全车比例分摊；` + seatFeeNote + `</p>`)
	return subject, b.String()
}

// writeMemberSection 输出一位成员的分周期明细与两部分费用。
func writeMemberSection(b *strings.Builder, m CarpoolSettlementMember) {
	who := m.Username
	if who == "" {
		who = m.Email
	}
	if who == "" {
		who = fmt.Sprintf("#%d", m.UserID)
	}
	fmt.Fprintf(b, `<h3>%s`, html.EscapeString(who))
	if m.Email != "" && m.Username != "" {
		fmt.Fprintf(b, ` <span style="font-weight:normal;color:#888">%s</span>`, html.EscapeString(m.Email))
	}
	fmt.Fprintf(b, ` <span style="font-weight:normal;color:#888">#%d</span></h3>`, m.UserID)

	if m.CycleBased && len(m.Cycles) > 0 {
		fmt.Fprintf(b, `<p>共 <b>%d</b> 个计费周期，其中 %d 个周期实际用量不足申报的保底、按保底计入。</p>`,
			m.CycleCount, m.FloorCycles)
		_, _ = b.WriteString(`<table border="1" cellpadding="6" cellspacing="0" style="border-collapse:collapse;font-size:13px">`)
		_, _ = b.WriteString(`<tr style="background:#f5f5f5">` +
			`<th>计费周期</th><th>申报额 (USD)</th><th>实际用量 (USD)</th><th>计入月消费 (USD)</th></tr>`)
		for _, c := range m.Cycles {
			floored := c.ActualUsageUSD <= c.ReservedUSD
			mark := ""
			if floored {
				mark = ` <span style="color:#b45309">（按保底计）</span>`
			}
			fmt.Fprintf(b, `<tr><td>%s ～ %s</td><td align="right">%.2f</td><td align="right">%.2f</td><td align="right">%.2f%s</td></tr>`,
				c.CycleStart.Format("01-02"), c.CycleEnd.Format("01-02"),
				c.DeclaredWeeklyQuotaUSD, c.ActualUsageUSD, c.BillableUsageUSD, mark)
		}
		fmt.Fprintf(b, `<tr style="background:#fafafa"><td><b>月度合计</b></td><td align="right">—</td>`+
			`<td align="right"><b>%.2f</b></td><td align="right"><b>%.2f</b></td></tr>`,
			m.ActualUsageUSD, m.BillableUsageUSD)
		_, _ = b.WriteString(`</table>`)
	} else {
		// 没有分周期台账（发车早于台账功能）：口径不同，必须说明白，
		// 否则收款方会以为这是按周算出来的。
		fmt.Fprintf(b, `<p style="color:#b45309">本成员无分周期台账，按整期合计计算：`+
			`实际用量 %.2f USD，地板 %.2f USD，计入月消费 %.2f USD。`+
			`（该口径下超用的周会补贴没用满的周，与逐周期计算的结果可能不同。）</p>`,
			m.ActualUsageUSD, m.FloorUsageUSD, m.BillableUsageUSD)
	}

	fmt.Fprintf(b, `<p>费用：席位费 <b>¥%.2f</b> ＋ 变动池分摊 <b>¥%.2f</b> ＝ <b>¥%.2f</b><br>`,
		m.SeatFeeFinalCNY, m.UsageFinalShareCNY, m.SeatFeeFinalCNY+m.UsageFinalShareCNY)
	fmt.Fprintf(b, `已预付 ¥%.2f，%s</p>`, m.QuotedPrepaidCNY, deltaPhrase(m.TotalDeltaCNY))
}

// deltaPhrase 把退补金额说成人话（正=退给成员，负=成员补交）。
func deltaPhrase(delta float64) string {
	switch {
	case delta > 0.004:
		return fmt.Sprintf(`应<b>退</b> ¥%.2f`, delta)
	case delta < -0.004:
		return fmt.Sprintf(`应<b>补</b> ¥%.2f`, -delta)
	default:
		return `无需退补`
	}
}

// carpoolSeatFeeTotalCNY 是本期席位费合计：type 3 每人固定（seatFeeCNY×人数）；
// type 1/2 全车 seatFeeCNY（按发车人数均摊，合计即 seatFeeCNY 本身）。
func carpoolSeatFeeTotalCNY(s *CarpoolSettlement) float64 {
	if s.CarType == CarpoolCarTypeQuotaV2 {
		return s.SeatFeeCNY * float64(s.MemberCount)
	}
	return s.SeatFeeCNY
}

// settlementGrandTotalCNY 是本期账单总额 = 席位费合计 + 变动池。
// 变动池是全车固定总额，与成员如何分摊无关；席位费合计按车型分支。
func settlementGrandTotalCNY(s *CarpoolSettlement) float64 {
	return carpoolSeatFeeTotalCNY(s) + s.UsagePoolCNY
}

// NotifyCarpoolSettlement 期末给运营联系人发结算邮件。
// 自定义规则车不发——它们的账不由平台计算。
func (s *CarpoolService) NotifyCarpoolSettlement(ctx context.Context, carpool *Carpool, settlement *CarpoolSettlement) {
	if carpool == nil || settlement == nil || settlement.ManualSettlement {
		return
	}
	subject, body := BuildCarpoolSettlementEmail(carpool, settlement)
	if subject == "" {
		return
	}
	s.sendCarpoolNotification(ctx, CarpoolAdminEmail, subject, body, "carpool_id", carpool.ID)
}

package service

import "fmt"

// 拼车额度预约制核心参数默认值（设计文档 §3）。
const (
	CarpoolDefaultWeeklyLimitUSD = 2400.0 // 整车（Pro 20x 账号）周限额
	CarpoolDefaultSeatFeeCNY     = 400.0  // 席位费（固定部分），全车每月
	CarpoolDefaultUsagePoolCNY   = 1000.0 // 变动池（用量部分），全车每月
	CarpoolDefaultReserveRatio   = 0.80   // 申报额中定向锁定/保底付费比例
	CarpoolDefaultLaunchMinRatio = 0.95   // 可发车总申报区间下限（×周限额）
	CarpoolDefaultLaunchMaxRatio = 1.05   // 可发车/可上车总申报区间上限（×周限额）

	// CarpoolPlusAccountsPerCar 是整车周限额对应的 Plus 账号数（2400 = 20 × Plus）。
	CarpoolPlusAccountsPerCar = 20.0
	// CarpoolDeclarationBufferRatio 是申报推荐值的缓冲系数（设计文档 §4.1，10–20%）。
	CarpoolDeclarationBufferRatio = 1.1
	// CarpoolForceLaunchMinRatio 是"降档发车"（force）允许的最低总申报比例。
	CarpoolForceLaunchMinRatio = 0.80
)

// carpoolPlusEquivalentUSD 返回 1 个 Plus 账号等价的周额度（整车 = 20 × Plus）。
func carpoolPlusEquivalentUSD(weeklyLimitUSD float64) float64 {
	return weeklyLimitUSD / CarpoolPlusAccountsPerCar
}

// CarpoolPrepaidCNY 计算上车预付（第一笔账，设计文档 §4.4）：
// 预付 = 席位费/人数 + 变动池×(申报/周限额)。
func CarpoolPrepaidCNY(seatFeeCNY, usagePoolCNY, weeklyLimitUSD, declaredWeeklyQuotaUSD float64, memberCount int) float64 {
	if memberCount < 1 {
		memberCount = 1
	}
	prepaid := seatFeeCNY / float64(memberCount)
	if weeklyLimitUSD > 0 {
		prepaid += usagePoolCNY * declaredWeeklyQuotaUSD / weeklyLimitUSD
	}
	return prepaid
}

// CarpoolMemberWeeklyLimitUSD 计算开车时写入成员订阅的周限额（设计文档 §4.2）：
// 个人订阅周限额 = reserveRatio×申报 + C，其中公共池 C = 周限额 − reserveRatio×Σ申报。
func CarpoolMemberWeeklyLimitUSD(weeklyLimitUSD, reserveRatio, declaredWeeklyQuotaUSD, declaredTotalUSD float64) float64 {
	sharedPool := weeklyLimitUSD - reserveRatio*declaredTotalUSD
	return reserveRatio*declaredWeeklyQuotaUSD + sharedPool
}

// CarpoolDeclarationRecommendation 是申报推荐结果（设计文档 §4.1）。
type CarpoolDeclarationRecommendation struct {
	RecommendedWeeklyQuotaUSD float64 `json:"recommended_weekly_quota_usd"`
	RawWeeklyUsageUSD         float64 `json:"raw_weekly_usage_usd"`
	BufferRatio               float64 `json:"buffer_ratio"`
	DaysWithRecords           int     `json:"days_with_records"`
	Basis                     string  `json:"basis"` // "usage_history" | "anchor"
	Message                   string  `json:"message"`
}

// BuildDeclarationRecommendation 按 §4.1 规则由最近 7 天用量聚合结果生成推荐申报值：
//   - ≥7 天记录：推荐值 = 最近 7 天实际用量；
//   - 1–6 天记录：按日均外推 ×7；
//   - 无记录：返回 1 个 Plus 等价的参照锚点。
//
// 推荐值叠加缓冲系数（raw 字段保留未加缓冲的原始值）。
func BuildDeclarationRecommendation(weeklyUsageUSD float64, daysWithRecords int, bufferRatio float64) CarpoolDeclarationRecommendation {
	if bufferRatio <= 0 {
		bufferRatio = 1
	}
	rec := CarpoolDeclarationRecommendation{
		BufferRatio:     bufferRatio,
		DaysWithRecords: daysWithRecords,
		Basis:           "usage_history",
	}
	switch {
	case daysWithRecords <= 0:
		anchor := carpoolPlusEquivalentUSD(CarpoolDefaultWeeklyLimitUSD)
		rec.DaysWithRecords = 0
		rec.Basis = "anchor"
		rec.RecommendedWeeklyQuotaUSD = anchor
		rec.Message = fmt.Sprintf("暂无使用记录，参考锚点：日均 2 小时编码 ≈ $%.0f/周（1 个 Plus 等价），请按自身用量估计", anchor)
	case daysWithRecords >= 7:
		rec.DaysWithRecords = 7
		rec.RawWeeklyUsageUSD = weeklyUsageUSD
		rec.RecommendedWeeklyQuotaUSD = weeklyUsageUSD * bufferRatio
		rec.Message = "基于你最近 7 天的使用记录"
	default:
		raw := weeklyUsageUSD / float64(daysWithRecords) * 7
		rec.RawWeeklyUsageUSD = raw
		rec.RecommendedWeeklyQuotaUSD = raw * bufferRatio
		rec.Message = fmt.Sprintf("基于你最近 %d 天的使用记录外推", daysWithRecords)
	}
	return rec
}

// CarpoolSettlementMemberInput 是结算计算的成员输入。
type CarpoolSettlementMemberInput struct {
	UserID                 int64
	Role                   string
	DeclaredWeeklyQuotaUSD float64
	PrepaidAmountCNY       float64 // 首月预付台账（发车时按发车人数锁定）
	ActualUsageUSD         float64 // 订阅周期内实际用量（月度窗口）
	PeriodDays             float64 // 订阅有效期天数（用于申报→周期地板折算）
}

// CarpoolSettlementMember 是结算单中的成员行。
type CarpoolSettlementMember struct {
	UserID                 int64   `json:"user_id"`
	Role                   string  `json:"role"`
	DeclaredWeeklyQuotaUSD float64 `json:"declared_weekly_quota_usd"`
	FloorUsageUSD          float64 `json:"floor_usage_usd"`    // 0.8×申报×周期周数（80% 地板）
	ActualUsageUSD         float64 `json:"actual_usage_usd"`   // 周期内实际用量
	BillableUsageUSD       float64 `json:"billable_usage_usd"` // 计费用量 = max(实际, 地板)
	FloorTriggered         bool    `json:"floor_triggered"`    // 实际未达地板，按地板计费
	PrepaidAmountCNY       float64 `json:"prepaid_amount_cny"`
	UsagePrepaidCNY        float64 `json:"usage_prepaid_cny"`     // 预付变动部分 = 变动池×申报/周限额
	UsageFinalShareCNY     float64 `json:"usage_final_share_cny"` // 最终分摊 = 变动池×计费用量/Σ计费用量
	UsageDeltaCNY          float64 `json:"usage_delta_cny"`       // 变动部分退/补：正=退，负=补
	SeatFeePrepaidCNY      float64 `json:"seat_fee_prepaid_cny"`  // 预付席位费部分
	SeatFeeFinalCNY        float64 `json:"seat_fee_final_cny"`    // 最终席位费 = 席位费/发车人数
	SeatFeeDeltaCNY        float64 `json:"seat_fee_delta_cny"`    // 席位费退/补：正=退，负=补
	TotalDeltaCNY          float64 `json:"total_delta_cny"`       // 合计退/补：正=退，负=补
}

// ComputeCarpoolSettlementMembers 按设计文档 §4.5 计算全车结算单（含 80% 地板规则）。
// 变动池收支恒等：ΣUsageFinalShareCNY = usagePoolCNY（Σ计费用量 > 0 时）。
func ComputeCarpoolSettlementMembers(weeklyLimitUSD, seatFeeCNY, usagePoolCNY, reserveRatio float64, inputs []CarpoolSettlementMemberInput) []CarpoolSettlementMember {
	members := make([]CarpoolSettlementMember, 0, len(inputs))
	if len(inputs) == 0 {
		return members
	}

	billableTotal := 0.0
	for _, in := range inputs {
		floor := reserveRatio * in.DeclaredWeeklyQuotaUSD * in.PeriodDays / 7
		actual := in.ActualUsageUSD
		if actual > floor {
			billableTotal += actual
		} else {
			billableTotal += floor
		}
	}

	seatFeeFinal := seatFeeCNY / float64(len(inputs))
	for _, in := range inputs {
		floor := reserveRatio * in.DeclaredWeeklyQuotaUSD * in.PeriodDays / 7
		billable := floor
		floorTriggered := true
		if in.ActualUsageUSD > floor {
			billable = in.ActualUsageUSD
			floorTriggered = false
		}

		usagePrepaid := 0.0
		if weeklyLimitUSD > 0 {
			usagePrepaid = usagePoolCNY * in.DeclaredWeeklyQuotaUSD / weeklyLimitUSD
		}
		usageFinalShare := 0.0
		if billableTotal > 0 {
			usageFinalShare = usagePoolCNY * billable / billableTotal
		}
		seatFeePrepaid := in.PrepaidAmountCNY - usagePrepaid

		member := CarpoolSettlementMember{
			UserID:                 in.UserID,
			Role:                   in.Role,
			DeclaredWeeklyQuotaUSD: in.DeclaredWeeklyQuotaUSD,
			FloorUsageUSD:          floor,
			ActualUsageUSD:         in.ActualUsageUSD,
			BillableUsageUSD:       billable,
			FloorTriggered:         floorTriggered,
			PrepaidAmountCNY:       in.PrepaidAmountCNY,
			UsagePrepaidCNY:        usagePrepaid,
			UsageFinalShareCNY:     usageFinalShare,
			UsageDeltaCNY:          usagePrepaid - usageFinalShare,
			SeatFeePrepaidCNY:      seatFeePrepaid,
			SeatFeeFinalCNY:        seatFeeFinal,
			SeatFeeDeltaCNY:        seatFeePrepaid - seatFeeFinal,
		}
		member.TotalDeltaCNY = member.UsageDeltaCNY + member.SeatFeeDeltaCNY
		members = append(members, member)
	}
	return members
}

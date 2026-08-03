package service

import (
	"fmt"
	"time"
)

// 拼车额度预约制核心参数默认值（设计文档 §3）。
const (
	CarpoolDefaultWeeklyLimitUSD = 2400.0 // 整车（Pro 20x 账号）周限额
	CarpoolDefaultSeatFeeCNY     = 400.0  // 席位费（固定部分），全车每月
	CarpoolDefaultUsagePoolCNY   = 1000.0 // 变动池（用量部分），全车每月
	CarpoolDefaultReserveRatio   = 0.80   // 申报额中定向锁定/保底付费比例
	CarpoolUsagePrepayRatio      = 0.80   // 首笔预付收取额度费分摊的比例
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
// 预付 = 席位费/人数 + 80%×变动池×(个人申报/全车申报)。
func CarpoolPrepaidCNY(seatFeeCNY, usagePoolCNY, declaredTotalUSD, declaredWeeklyQuotaUSD float64, memberCount int) float64 {
	if memberCount < 1 {
		memberCount = 1
	}
	prepaid := seatFeeCNY / float64(memberCount)
	if declaredTotalUSD > 0 {
		prepaid += CarpoolUsagePrepayRatio * usagePoolCNY * declaredWeeklyQuotaUSD / declaredTotalUSD
	}
	return prepaid
}

// CarpoolMemberWeeklyLimitUSD 计算开车时写入成员订阅的周限额（设计文档 §4.2）：
// 个人订阅周限额 = reserveRatio×申报 + C，其中公共池 C = 周限额 − reserveRatio×Σ申报。
// 该值是个人绝对上限（防呆）；公共池的硬约束由组级计数器执行（见 carpool_commons.go）。
func CarpoolMemberWeeklyLimitUSD(weeklyLimitUSD, reserveRatio, declaredWeeklyQuotaUSD, declaredTotalUSD float64) float64 {
	return CarpoolMemberReservedUSD(reserveRatio, declaredWeeklyQuotaUSD) +
		CarpoolSharedPoolCapacityUSD(weeklyLimitUSD, reserveRatio, declaredTotalUSD)
}

// CarpoolMemberReservedUSD 计算成员的保底额度 r = reserveRatio×申报。
// 发车时写入订阅的 weekly_reserved_usd；周用量 < r 无条件放行（保底硬保证）。
func CarpoolMemberReservedUSD(reserveRatio, declaredWeeklyQuotaUSD float64) float64 {
	return reserveRatio * declaredWeeklyQuotaUSD
}

// CarpoolSharedPoolCapacityUSD 计算公共池容量 C = 周限额 − reserveRatio×Σ申报。
// 105% 超募上限保证 C ≥ (1−0.8×1.05)×周限额 = 16%×周限额 > 0。
func CarpoolSharedPoolCapacityUSD(weeklyLimitUSD, reserveRatio, declaredTotalUSD float64) float64 {
	return weeklyLimitUSD - reserveRatio*declaredTotalUSD
}

// CarpoolCommonsExcessDelta 计算一次用量累加应计入组级公共池计数器的增量：
//
//	delta = max(0, newUsage − r) − max(0, oldUsage − r)
//
// 只有越过保底 r 的那部分消耗才占用公共池（跨界场景：r−10 → r+15，delta = 15）。
func CarpoolCommonsExcessDelta(oldUsage, newUsage, reserved float64) float64 {
	excessAfter := newUsage - reserved
	if excessAfter < 0 {
		excessAfter = 0
	}
	excessBefore := oldUsage - reserved
	if excessBefore < 0 {
		excessBefore = 0
	}
	delta := excessAfter - excessBefore
	if delta < 0 {
		// 用量不会回退；防御负增量（如窗口重置竞态下的脏读）。
		return 0
	}
	return delta
}

// carpoolWeeklyWindowDuration 是拼车周窗口长度（与 NeedsWeeklyReset 的 7 天判定一致）。
const carpoolWeeklyWindowDuration = 7 * 24 * time.Hour

// CarpoolWeeklyWindowGridStart 把过期的拼车周窗口吸附回 7 天网格：
// 在 prev 锚定的网格上取最大的起点 P，满足 P ≤ now < P+7d。
// 发车时全车 weekly_window_start 统一对齐，之后每次周重置都吸附回同一网格，
// 保证全体成员的窗口起点恒等，组级公共池计数器 key 因此一致。
func CarpoolWeeklyWindowGridStart(prev, now time.Time) time.Time {
	if prev.IsZero() || now.Before(prev.Add(carpoolWeeklyWindowDuration)) {
		// 窗口未过期（now < prev+7d，与 NeedsWeeklyReset 的 >= 判定互补）：保持原起点。
		return prev
	}
	periods := int(now.Sub(prev) / carpoolWeeklyWindowDuration)
	if periods < 1 {
		periods = 1
	}
	return prev.Add(time.Duration(periods) * carpoolWeeklyWindowDuration)
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
	Email                  string // 供车主对应到真人收款
	Username               string // 同上，可能为空
	Role                   string
	DeclaredWeeklyQuotaUSD float64
	PrepaidAmountCNY       float64 // 首月预付台账（发车时按发车人数锁定）
	QuotedPrepaidCNY       float64 // 上车当时报给用户的预付（按上车时人数），实际收款依据
	ActualUsageUSD         float64 // 订阅周期内实际用量（月度窗口）
	PeriodDays             float64 // 订阅有效期天数（用于申报→周期地板折算）

	// Cycles 是该成员在本订阅周期内的分周期账目（含当期未关闭的那一周）。
	// 非空时按周计地板：Σ max(周实际, 0.8×申报)；为空时退回按月整体算
	// （老车没有台账），此时 CycleBased 为 false，单据上要标明口径不同。
	Cycles []CarpoolBillingCycle
}

// CarpoolSettlementMember 是结算单中的成员行。
type CarpoolSettlementMember struct {
	UserID                 int64   `json:"user_id"`
	Email                  string  `json:"email,omitempty"`
	Username               string  `json:"username,omitempty"`
	Role                   string  `json:"role"`
	DeclaredWeeklyQuotaUSD float64 `json:"declared_weekly_quota_usd"`
	// CycleBased 为 true 表示计费用量按周逐个算了地板（正确口径）；
	// false 表示该成员没有周期台账，退回按月整体算地板（老车/台账缺失）。
	CycleBased         bool                  `json:"cycle_based"`
	CycleCount         int                   `json:"cycle_count"`           // 计费周期数
	FloorCycles        int                   `json:"floor_cycles"`          // 其中被地板托底的周期数
	Cycles             []CarpoolBillingCycle `json:"cycles,omitempty"`      // 每个周期的明细
	FloorUsageUSD      float64               `json:"floor_usage_usd"`       // 地板合计（按周口径时为 Σ各周保底）
	ActualUsageUSD     float64               `json:"actual_usage_usd"`      // 周期内实际用量
	BillableUsageUSD   float64               `json:"billable_usage_usd"`    // 计费用量 = max(实际, 地板)
	FloorTriggered     bool                  `json:"floor_triggered"`       // 实际未达地板，按地板计费
	PrepaidAmountCNY   float64               `json:"prepaid_amount_cny"`    // 新规则预付 = 最终席位费 + 80%×申报占比额度费
	QuotedPrepaidCNY   float64               `json:"quoted_prepaid_cny"`    // 上车当时报给用户的金额（实际收款依据）
	UsagePrepaidCNY    float64               `json:"usage_prepaid_cny"`     // 预付变动部分 = 80%×变动池×申报/Σ申报
	UsageFinalShareCNY float64               `json:"usage_final_share_cny"` // 最终分摊 = 变动池×计费用量/Σ计费用量
	UsageDeltaCNY      float64               `json:"usage_delta_cny"`       // 变动部分退/补：正=退，负=补
	SeatFeePrepaidCNY  float64               `json:"seat_fee_prepaid_cny"`  // 预付席位费 = 席位费/发车人数
	SeatFeeFinalCNY    float64               `json:"seat_fee_final_cny"`    // 最终席位费 = 席位费/发车人数
	SeatFeeDeltaCNY    float64               `json:"seat_fee_delta_cny"`    // 席位费固定不退补，恒为 0
	TotalDeltaCNY      float64               `json:"total_delta_cny"`       // 用量退/补：正=退，负=补
}

// ComputeCarpoolSettlementMembers 按设计文档 §4.5 计算全车结算单（含 80% 地板规则）。
// 变动池收支恒等：ΣUsageFinalShareCNY = usagePoolCNY（Σ计费用量 > 0 时）。
func ComputeCarpoolSettlementMembers(weeklyLimitUSD, seatFeeCNY, usagePoolCNY, reserveRatio float64, inputs []CarpoolSettlementMemberInput) []CarpoolSettlementMember {
	members := make([]CarpoolSettlementMember, 0, len(inputs))
	if len(inputs) == 0 {
		return members
	}

	billableTotal := 0.0
	declaredTotal := 0.0
	for _, in := range inputs {
		b, _, _, _, _ := memberBillable(in, reserveRatio)
		billableTotal += b
		declaredTotal += in.DeclaredWeeklyQuotaUSD
	}

	seatFeeFinal := seatFeeCNY / float64(len(inputs))
	for _, in := range inputs {
		billable, floor, actual, cycleBased, cycleStats := memberBillable(in, reserveRatio)
		floorTriggered := actual <= floor

		usagePrepaid := 0.0
		if declaredTotal > 0 {
			usagePrepaid = CarpoolUsagePrepayRatio * usagePoolCNY * in.DeclaredWeeklyQuotaUSD / declaredTotal
		}
		usageFinalShare := 0.0
		if billableTotal > 0 {
			usageFinalShare = usagePoolCNY * billable / billableTotal
		}
		quoted := in.QuotedPrepaidCNY
		if quoted <= 0 {
			quoted = in.PrepaidAmountCNY
		}
		prepaid := seatFeeFinal + usagePrepaid

		member := CarpoolSettlementMember{
			UserID:                 in.UserID,
			Email:                  in.Email,
			Username:               in.Username,
			Role:                   in.Role,
			DeclaredWeeklyQuotaUSD: in.DeclaredWeeklyQuotaUSD,
			CycleBased:             cycleBased,
			CycleCount:             cycleStats.count,
			FloorCycles:            cycleStats.floorCycles,
			Cycles:                 in.Cycles,
			FloorUsageUSD:          floor,
			ActualUsageUSD:         actual,
			BillableUsageUSD:       billable,
			FloorTriggered:         floorTriggered,
			PrepaidAmountCNY:       prepaid,
			QuotedPrepaidCNY:       quoted,
			UsagePrepaidCNY:        usagePrepaid,
			UsageFinalShareCNY:     usageFinalShare,
			UsageDeltaCNY:          usagePrepaid - usageFinalShare,
			SeatFeePrepaidCNY:      seatFeeFinal,
			SeatFeeFinalCNY:        seatFeeFinal,
			SeatFeeDeltaCNY:        0,
		}
		member.TotalDeltaCNY = member.UsageDeltaCNY
		members = append(members, member)
	}
	return members
}

// cycleStats 是分周期账目的计数。
type cycleStats struct {
	count       int
	floorCycles int
}

// memberBillable 计算一位成员的计费用量、地板合计与实际用量。
//
// 有周期台账时按**周**逐个算地板再求和，这是正确口径：保底按周刷新、未用完
// 不结转，所以每周各算各的 max(实际, 保底)。
//
// 没有台账时（发车早于台账功能的老车）退回按月整体算 max(Σ实际, 0.8×申报×周数)，
// 并把 cycleBased 置 false——这个口径会让超用的周补贴没用满的周，账单上要标明。
func memberBillable(in CarpoolSettlementMemberInput, reserveRatio float64) (billable, floor, actual float64, cycleBased bool, stats cycleStats) {
	if len(in.Cycles) > 0 {
		for _, c := range in.Cycles {
			actual += c.ActualUsageUSD
			floor += c.ReservedUSD
			billable += CarpoolCycleBillableUSD(c.ActualUsageUSD, c.ReservedUSD)
			stats.count++
			if c.ActualUsageUSD <= c.ReservedUSD {
				stats.floorCycles++
			}
		}
		return billable, floor, actual, true, stats
	}

	floor = reserveRatio * in.DeclaredWeeklyQuotaUSD * in.PeriodDays / 7
	actual = in.ActualUsageUSD
	billable = CarpoolCycleBillableUSD(actual, floor)
	return billable, floor, actual, false, stats
}

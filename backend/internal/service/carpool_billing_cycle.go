package service

import (
	"context"
	"time"
)

// CarpoolBillingCycle 是一个已关闭的拼车计费周期（一周）。
//
// 保底按周刷新、未用完不结转，所以计费也必须按周落地：每周各算各的
// max(实际, 保底)，再在月层面求和。只有把每个关闭的周期存下来，月末才算得出
// 这个和——weekly_usage_usd 到点就被清零，不存就永远丢了。
type CarpoolBillingCycle struct {
	CarpoolID              int64     `json:"carpool_id"`
	UserID                 int64     `json:"user_id"`
	SubscriptionID         *int64    `json:"subscription_id,omitempty"`
	GroupID                *int64    `json:"group_id,omitempty"`
	CycleStart             time.Time `json:"cycle_start"`
	CycleEnd               time.Time `json:"cycle_end"`
	DeclaredWeeklyQuotaUSD float64   `json:"declared_weekly_quota_usd"`
	ReservedUSD            float64   `json:"reserved_usd"`
	ActualUsageUSD         float64   `json:"actual_usage_usd"`
	BillableUsageUSD       float64   `json:"billable_usage_usd"`
	// Open 为 true 表示这是尚未关闭的当期周期（不来自台账，实时读订阅）。
	Open bool `json:"open"`
}

// CarpoolCycleBillableUSD 计算单个周期的计费用量：max(实际, 保底)。
// 这就是"不足申报额 80% 的记为 80%"在**周**这一层的落地。
func CarpoolCycleBillableUSD(actualUSD, reservedUSD float64) float64 {
	if actualUSD > reservedUSD {
		return actualUSD
	}
	return reservedUSD
}

// CarpoolBillingCycleRecorder 落库已关闭的计费周期。
type CarpoolBillingCycleRecorder interface {
	// RecordCycle 把订阅当前所处的周期落成台账，周期结束时刻为 cycleEnd。
	//
	// 刻意只收订阅 ID：用量、保底、窗口起点全部由实现方直接从订阅行读取，
	// 不接受调用方传值。原因是调用点（窗口重置）拿到的 sub 是内存快照，而
	// ValidateAndCheckLimits 为了让预检查不误拒用户，已经把 WeeklyUsageUSD
	// 在内存里清零了——照着它记账会让每个周期的实际用量都变成 0，计费恒等于
	// 保底，重度用户的超出部分全部漏计。
	//
	// 同一订阅同一周期起点重复写入必须幂等（唯一索引保证），因为周重置可能
	// 被并发请求同时触发。
	RecordCycle(ctx context.Context, subscriptionID int64, cycleEnd time.Time) error
	// ListCyclesByCarpool 取回某辆车在给定时间范围内的全部已关闭周期。
	ListCyclesByCarpool(ctx context.Context, carpoolID int64, from, to time.Time) ([]CarpoolBillingCycle, error)
}

// CarpoolMemberCycleSummary 是一位成员在整个订阅周期内的分周期账目。
type CarpoolMemberCycleSummary struct {
	UserID int64                 `json:"user_id"`
	Cycles []CarpoolBillingCycle `json:"cycles"`
	// BillableTotalUSD = Σ 各周期 max(实际, 保底)
	BillableTotalUSD float64 `json:"billable_total_usd"`
	// ActualTotalUSD = Σ 各周期实际用量
	ActualTotalUSD float64 `json:"actual_total_usd"`
	// FloorCycles 是被地板托底的周期数（实际不足保底）。
	FloorCycles int `json:"floor_cycles"`
}

// SummariseMemberCycles 汇总一位成员的分周期账目。
func SummariseMemberCycles(userID int64, cycles []CarpoolBillingCycle) CarpoolMemberCycleSummary {
	summary := CarpoolMemberCycleSummary{UserID: userID, Cycles: cycles}
	for _, c := range cycles {
		summary.ActualTotalUSD += c.ActualUsageUSD
		summary.BillableTotalUSD += c.BillableUsageUSD
		if c.ActualUsageUSD <= c.ReservedUSD {
			summary.FloorCycles++
		}
	}
	return summary
}

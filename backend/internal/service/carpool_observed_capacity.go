package service

import (
	"context"
	"time"
)

const (
	// CarpoolObservedCapacityMinPercent 是采信实测容量所需的最低上游用量百分比。
	//
	// 实测容量 = 全车美元用量 / 上游已用百分比。分母很小时这个商的方差极大
	// （上游只报到小数点后几位，1% 的量化误差在 P=1% 时就是 100% 的容量误差），
	// 所以要等窗口消耗到一定程度才开始采信。
	CarpoolObservedCapacityMinPercent = 10.0

	// CarpoolObservedCapacitySmoothing 是实测容量的指数平滑系数（新样本权重）。
	// 平滑是为了让容量不要随每次采样上下跳动——它决定公共池还剩多少，
	// 抖动会让用户忽而被放行忽而被拒。
	CarpoolObservedCapacitySmoothing = 0.3
)

// CarpoolCapacitySnapshot 是一辆车当前窗口的容量实测结果。
type CarpoolCapacitySnapshot struct {
	// ObservedTotalUSD 是由"全车用量 ÷ 上游已用百分比"推出的整车真实容量。
	ObservedTotalUSD float64
	// ReservedTotalUSD 是全车保底之和 Σr。
	ReservedTotalUSD float64
	// CommonsUSD 是据实测推出的公共池容量 max(0, 实测容量 − Σr)。
	CommonsUSD float64
	// Trusted 为 false 时调用方应退回发车时锁定的公共池容量。
	Trusted bool
	// Oversold 为 true 表示实测容量已经低于全车保底之和——这辆车超卖了，
	// 保底在物理上无法全部兑现，运营需要知道。
	Oversold bool
}

// CarpoolObservedCapacitySource 提供按组的实测容量快照。
type CarpoolObservedCapacitySource interface {
	GroupObservedCapacity(ctx context.Context, groupID int64, windowStart time.Time) (*CarpoolCapacitySnapshot, error)
}

// CarpoolObservedTotalCapacityUSD 由全车美元用量与上游已用百分比反推整车真实容量。
//
// 这样一来 $2400 只是发车时的定价基准，不再兼任执行上限——上游到底给多少额度
// 由实测说了算，也就不用靠人工标定那个数字（设计文档 §7 风险 1）。
// usedPercent 以百分比计（0–100）。百分比过低时返回 trusted=false。
func CarpoolObservedTotalCapacityUSD(totalUsageUSD, usedPercent float64) (float64, bool) {
	if usedPercent < CarpoolObservedCapacityMinPercent || totalUsageUSD <= 0 {
		return 0, false
	}
	capacity := totalUsageUSD / (usedPercent / 100)
	if capacity <= 0 {
		return 0, false
	}
	return capacity, true
}

// CarpoolSmoothObservedCapacity 对实测容量做指数平滑。prev <= 0 表示还没有历史值，
// 直接采用新样本。
func CarpoolSmoothObservedCapacity(prev, sample float64) float64 {
	if prev <= 0 {
		return sample
	}
	return prev*(1-CarpoolObservedCapacitySmoothing) + sample*CarpoolObservedCapacitySmoothing
}

// BuildCarpoolCapacitySnapshot 由实测整车容量与全车保底之和推出公共池容量。
//
// 保底 r 不参与调整——那是对用户的承诺，不能因为实测容量缩水就打折；缩水全部
// 由公共池吸收，这正是"机动部分"该起的作用。实测容量低于 Σr 时公共池归零并
// 标记超卖。
func BuildCarpoolCapacitySnapshot(observedTotalUSD, reservedTotalUSD float64, trusted bool) *CarpoolCapacitySnapshot {
	snapshot := &CarpoolCapacitySnapshot{
		ObservedTotalUSD: observedTotalUSD,
		ReservedTotalUSD: reservedTotalUSD,
		Trusted:          trusted,
	}
	if !trusted {
		return snapshot
	}
	commons := observedTotalUSD - reservedTotalUSD
	if commons <= 0 {
		snapshot.CommonsUSD = 0
		snapshot.Oversold = true
		return snapshot
	}
	snapshot.CommonsUSD = commons
	return snapshot
}

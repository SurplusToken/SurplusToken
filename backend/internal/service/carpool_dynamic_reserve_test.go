package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 容量等于车周限额时，新口径必须与原口径逐位相等——否则这次改动会悄悄
// 改变所有正常车的保底和月底计费。
func TestReservedFromCapacityMatchesLegacyAtNominalCapacity(t *testing.T) {
	for _, declared := range []float64{0, 21.6, 96, 240, 264, 432, 960, 2400} {
		legacy := CarpoolMemberReservedUSD(CarpoolDefaultReserveRatio, declared)
		scaled := CarpoolReservedFromCapacityUSD(CarpoolDefaultWeeklyLimitUSD,
			CarpoolDefaultWeeklyLimitUSD, CarpoolDefaultReserveRatio, declared)
		require.InDelta(t, legacy, scaled, 1e-9,
			"申报 %v：容量等于标称时两种口径必须一致", declared)
	}
}

// 上游被风控缩水时，保底按同一比例下调。
func TestReservedFromCapacityScalesWithCapacity(t *testing.T) {
	// codexpro 实测：容量 1899、Σ申报 2376、车周限额 2400
	const capacity, limit, ratio = 1899.0, 2400.0, 0.8

	// 申报 264（忘忧北萱草）：原保底 211.20，缩水后应为 1899×(264/2400)×0.8 = 167.11
	require.InDelta(t, 167.112, CarpoolReservedFromCapacityUSD(capacity, limit, ratio, 264), 1e-3)
	// 容量掉了 20.9%，保底同比例掉
	require.InDelta(t, capacity/limit,
		CarpoolReservedFromCapacityUSD(capacity, limit, ratio, 264)/
			CarpoolMemberReservedUSD(ratio, 264), 1e-9)
}

// 这是本次改动的全部目的：公共池不再被挤成 0 或负数。
//
// 事故现场：codexpro 实测容量 1899 低于 Σ保底 1920，公共池被算成 0，
// 一位刚越过保底 0.07 的成员当场被 CARPOOL_SHARED_POOL_EXHAUSTED 拒绝。
func TestCommonsStaysPositiveWhenCapacityShrinks(t *testing.T) {
	const limit, ratio = 2400.0, 0.8
	declared := []float64{432, 360, 360, 300, 264, 240, 204, 120, 120} // codexpro 九人，Σ=2400

	for _, capacity := range []float64{2600, 2400, 1899, 1200, 600, 100} {
		var reservedTotal float64
		for _, d := range declared {
			reservedTotal += CarpoolReservedFromCapacityUSD(capacity, limit, ratio, d)
		}
		commons := capacity - reservedTotal
		require.Greater(t, commons, 0.0,
			"容量 %v 时公共池被挤成 %v —— 刚越过保底的成员会被误拦", capacity, commons)
		// Σ申报 恰为车周限额时，公共池恒为容量的 (1−0.8) = 20%
		require.InDelta(t, capacity*0.2, commons, 1e-6)
	}

	// 对照：旧口径在容量 1899 时确实是负的，这正是事故的成因
	var legacyTotal float64
	for _, d := range declared {
		legacyTotal += CarpoolMemberReservedUSD(ratio, d)
	}
	require.Less(t, 1899.0-legacyTotal, 0.0, "旧口径在实测 1899 下公共池为负，事故可复现")
}

// 没有可信容量（上游用量还太低）时退回标称口径，不能算出 0 保底。
func TestReservedFromCapacityFallsBackWithoutCapacity(t *testing.T) {
	legacy := CarpoolMemberReservedUSD(0.8, 264)
	require.InDelta(t, legacy, CarpoolReservedFromCapacityUSD(0, 2400, 0.8, 264), 1e-9)
	require.InDelta(t, legacy, CarpoolReservedFromCapacityUSD(-1, 2400, 0.8, 264), 1e-9)
	require.InDelta(t, legacy, CarpoolReservedFromCapacityUSD(1899, 0, 0.8, 264), 1e-9)
}

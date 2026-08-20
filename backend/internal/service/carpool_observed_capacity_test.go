package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 由"全车美元用量 ÷ 上游已用百分比"反推整车真实容量：
// 用了 $1200、上游说用掉 50% → 这辆车这一周实际有 $2400。
func TestCarpoolObservedTotalCapacityUSD(t *testing.T) {
	capacity, trusted := CarpoolObservedTotalCapacityUSD(1200, 50)
	require.True(t, trusted)
	require.InDelta(t, 2400, capacity, 1e-9)

	// 上游其实只给了 $2000：同样用掉 $1200 时百分比会更高
	capacity, trusted = CarpoolObservedTotalCapacityUSD(1200, 60)
	require.True(t, trusted)
	require.InDelta(t, 2000, capacity, 1e-9)
}

// 百分比太低时商的方差极大，不能采信——上游只报到有限精度，
// P=1% 时 1 个百分点的量化误差就是 100% 的容量误差。
func TestCarpoolObservedTotalCapacityRejectsLowPercent(t *testing.T) {
	_, trusted := CarpoolObservedTotalCapacityUSD(12, CarpoolObservedCapacityMinPercent-0.1)
	require.False(t, trusted)

	_, trusted = CarpoolObservedTotalCapacityUSD(120, CarpoolObservedCapacityMinPercent)
	require.True(t, trusted, "达到下限即可采信")

	// 没有用量时也无从反推
	_, trusted = CarpoolObservedTotalCapacityUSD(0, 50)
	require.False(t, trusted)
}

// 平滑：首个样本直接采用，之后按系数收敛，避免公共池余额随采样跳动。
func TestCarpoolSmoothObservedCapacity(t *testing.T) {
	require.InDelta(t, 2400, CarpoolSmoothObservedCapacity(0, 2400), 1e-9)

	smoothed := CarpoolSmoothObservedCapacity(2400, 2000)
	require.Greater(t, smoothed, 2000.0, "不应一步跳到新样本")
	require.Less(t, smoothed, 2400.0, "但要朝新样本移动")

	// 反复采到同一个值时应收敛过去
	v := 2400.0
	for i := 0; i < 50; i++ {
		v = CarpoolSmoothObservedCapacity(v, 2000)
	}
	require.InDelta(t, 2000, v, 1.0)
}

// 保底 r 不参与调整：容量缩水全部由公共池吸收。
func TestBuildCarpoolCapacitySnapshotAbsorbsShrinkIntoCommons(t *testing.T) {
	// 实测 2400，全车保底 1920 → 公共池 480
	snapshot := BuildCarpoolCapacitySnapshot(2400, 1920, true)
	require.True(t, snapshot.Trusted)
	require.False(t, snapshot.Oversold)
	require.InDelta(t, 480, snapshot.CommonsUSD, 1e-9)

	// 上游其实只给 2100 → 保底不变，公共池缩到 180
	snapshot = BuildCarpoolCapacitySnapshot(2100, 1920, true)
	require.InDelta(t, 180, snapshot.CommonsUSD, 1e-9)
	require.False(t, snapshot.Oversold)
}

// 实测容量跌破全车保底之和 = 这辆车超卖了：公共池归零并标记，
// 保底在物理上已经无法全部兑现，运营需要知道。
func TestBuildCarpoolCapacitySnapshotFlagsOversold(t *testing.T) {
	snapshot := BuildCarpoolCapacitySnapshot(1500, 1920, true)
	require.True(t, snapshot.Trusted)
	require.True(t, snapshot.Oversold)
	require.Zero(t, snapshot.CommonsUSD, "超卖时公共池必须归零，不能是负数")
}

// 不可信的样本不产出容量，调用方据此退回发车时锁定的公共池。
func TestBuildCarpoolCapacitySnapshotUntrusted(t *testing.T) {
	snapshot := BuildCarpoolCapacitySnapshot(0, 1920, false)
	require.False(t, snapshot.Trusted)
	require.False(t, snapshot.Oversold)
	require.Zero(t, snapshot.CommonsUSD)
}

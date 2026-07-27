package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 默认间隔是 15 分钟：订阅到期时刻各车不同，按天扫会让账单最多晚一天才出。
func TestCarpoolSettlementSchedulerDefaultInterval(t *testing.T) {
	require.Equal(t, 15*time.Minute, CarpoolSettlementInterval)

	s := NewCarpoolSettlementScheduler(nil, 0)
	require.Equal(t, CarpoolSettlementInterval, s.interval, "间隔 <= 0 时用默认值")

	s = NewCarpoolSettlementScheduler(nil, time.Minute)
	require.Equal(t, time.Minute, s.interval, "显式间隔优先")
}

// 没有 carpoolService 时不启动，Stop 也不得阻塞或 panic。
func TestCarpoolSettlementSchedulerStartStopIsSafe(t *testing.T) {
	s := NewCarpoolSettlementScheduler(nil, time.Minute)
	require.NotPanics(t, func() {
		s.Start()
		s.Stop()
	})
}

// 每个实例有独立 ID，选主才有意义。
func TestCarpoolSettlementSchedulerHasDistinctInstanceIDs(t *testing.T) {
	a := NewCarpoolSettlementScheduler(nil, time.Minute)
	b := NewCarpoolSettlementScheduler(nil, time.Minute)
	require.NotEmpty(t, a.instanceID)
	require.NotEqual(t, a.instanceID, b.instanceID)
}

// 巡检扫到的车逐一结算；已被别的实例结过的（AlreadySettled）跳过而不中断，
// 这是并发下的正常情况，不该让整轮巡检失败。
func TestSettleExpiredCarpoolsSkipsAlreadySettled(t *testing.T) {
	settledAt := time.Now()
	carpool := &Carpool{
		ID: 7, Status: "active", WeeklyLimitUSD: 2400,
		SeatFeeCNY: 400, UsagePoolCNY: 1000, ReserveRatio: 0.8,
		SettledAt: &settledAt, // 已结算 → SettleCarpool 返回 AlreadySettled
	}
	stub := &carpoolRepoStub{carpool: carpool, expiredUnsettled: []int64{7}}
	svc := NewCarpoolService(stub, nil, nil, nil)

	settled, err := svc.SettleExpiredCarpools(context.Background())
	require.NoError(t, err, "并发下别的实例已结过，不该让整轮巡检报错")
	require.Zero(t, settled)
}

// 没有到期未结算的车时安静返回。
func TestSettleExpiredCarpoolsNoop(t *testing.T) {
	stub := &carpoolRepoStub{}
	svc := NewCarpoolService(stub, nil, nil, nil)

	settled, err := svc.SettleExpiredCarpools(context.Background())
	require.NoError(t, err)
	require.Zero(t, settled)
}

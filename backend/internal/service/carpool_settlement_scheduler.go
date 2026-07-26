package service

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// CarpoolSettlementInterval 是期末结算巡检的间隔。
	//
	// 取 15 分钟而不是一天：订阅到期时刻各车不同、上游窗口也可能在任意时点
	// 推进，按天扫会让账单最多晚一天才出。这个扫描很便宜（只查"已到期且
	// 未结算"的车，正常情况下一辆都没有），高频不构成负担。
	CarpoolSettlementInterval = 15 * time.Minute

	// carpoolSettlementLeaderLockKey 保证多实例部署下只有一个实例出账。
	// 出账本身是幂等的（settled_at IS NULL 守卫），加锁是为了避免 N 个实例
	// 同时扫描、同时对同一辆车发重复邮件。
	carpoolSettlementLeaderLockKey = "carpool:settlement:leader"
	// carpoolSettlementLeaderLockTTL 限定崩溃恢复时间：持锁实例挂了之后，
	// 最多等这么久就会有别的实例接手。
	carpoolSettlementLeaderLockTTL = 10 * time.Minute
)

// CarpoolSettlementScheduler 周期性扫描到期未结算的拼车，冻结结算并发出账单。
type CarpoolSettlementScheduler struct {
	carpoolService *CarpoolService
	interval       time.Duration
	stopCh         chan struct{}
	stopOnce       sync.Once
	wg             sync.WaitGroup

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
}

// NewCarpoolSettlementScheduler 创建期末结算巡检器。interval <= 0 时用默认值。
func NewCarpoolSettlementScheduler(carpoolService *CarpoolService, interval time.Duration) *CarpoolSettlementScheduler {
	if interval <= 0 {
		interval = CarpoolSettlementInterval
	}
	return &CarpoolSettlementScheduler{
		carpoolService: carpoolService,
		interval:       interval,
		stopCh:         make(chan struct{}),
		instanceID:     uuid.NewString(),
	}
}

// SetLeaderLock 注入选主所需的缓存与数据库。两者都为 nil 时不选主，直接跑
// （单实例部署与测试的行为）。
func (s *CarpoolSettlementScheduler) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *CarpoolSettlementScheduler) Start() {
	if s == nil || s.carpoolService == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
	log.Printf("[CarpoolSettlement] scheduler started (interval=%s)", s.interval)
}

func (s *CarpoolSettlementScheduler) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *CarpoolSettlementScheduler) runOnce() {
	if s == nil || s.carpoolService == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 多实例守卫：出账幂等，但没有这把锁会有 N 个实例同时扫描、
	// 对同一辆车重复发账单邮件。
	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db,
		carpoolSettlementLeaderLockKey, s.instanceID, carpoolSettlementLeaderLockTTL)
	if !ok {
		return
	}
	defer release()

	settled, err := s.carpoolService.SettleExpiredCarpools(ctx)
	if err != nil {
		log.Printf("[CarpoolSettlement] scan failed: %v", err)
		return
	}
	if settled > 0 {
		log.Printf("[CarpoolSettlement] settled %d carpool(s)", settled)
	}
}

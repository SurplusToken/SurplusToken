//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newUpstreamQuotaTestCache(t *testing.T) (*carpoolUpstreamQuotaCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &carpoolUpstreamQuotaCache{rdb: rdb}, mr
}

func TestCarpoolUpstreamQuotaCache_GetMiss(t *testing.T) {
	c, _ := newUpstreamQuotaTestCache(t)
	entry, ok, err := c.Get(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if ok || entry != nil {
		t.Errorf("expected miss, got ok=%v entry=%v", ok, entry)
	}
}

func TestCarpoolUpstreamQuotaCache_SetThenGet(t *testing.T) {
	c, mr := newUpstreamQuotaTestCache(t)
	ctx := context.Background()
	usedPercent := 7.0
	entry := &service.CarpoolUpstreamQuotaCacheEntry{
		CprAccountID: "acct_1",
		GeneratedAt:  "2026-10-12T08:00:00Z",
		Weekly: &service.UpstreamQuotaWeekly{
			Available:          true,
			UsedPercent:        &usedPercent,
			UsedPercentDisplay: "7.0%",
			ResetAt:            "2026-10-13T10:00:00Z",
		},
	}
	if err := c.Set(ctx, 7, entry, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	got, ok, err := c.Get(ctx, 7)
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.CprAccountID != "acct_1" || got.GeneratedAt != "2026-10-12T08:00:00Z" {
		t.Errorf("got = %+v", got)
	}
	if got.Weekly == nil || !got.Weekly.Available || got.Weekly.UsedPercent == nil || *got.Weekly.UsedPercent != 7.0 {
		t.Errorf("weekly = %+v", got.Weekly)
	}
	if ttl := mr.TTL("carpool:upstream_quota:7"); ttl <= 0 {
		t.Errorf("snapshot key TTL = %v, want > 0", ttl)
	}
	// 快照必须落在约定前缀下，冷却 key 走另一前缀。
	if len(mr.Keys()) != 1 {
		t.Errorf("keys = %v, want only the snapshot key", mr.Keys())
	}
}

func TestCarpoolUpstreamQuotaCache_RefreshCooldown(t *testing.T) {
	c, mr := newUpstreamQuotaTestCache(t)
	ctx := context.Background()

	ok, err := c.TryAcquireRefreshCooldown(ctx, 7, time.Minute)
	if err != nil || !ok {
		t.Fatalf("first acquire: ok=%v err=%v", ok, err)
	}
	ok, err = c.TryAcquireRefreshCooldown(ctx, 7, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("second acquire must be rejected while the cooldown window is open")
	}
	if ttl := mr.TTL("carpool:upstream_quota:refresh:7"); ttl <= 0 {
		t.Errorf("cooldown key TTL = %v, want > 0", ttl)
	}

	// 窗口过期后可再次抢占；不同车互不影响。
	mr.FastForward(2 * time.Minute)
	if ok, err := c.TryAcquireRefreshCooldown(ctx, 7, time.Minute); err != nil || !ok {
		t.Fatalf("acquire after expiry: ok=%v err=%v", ok, err)
	}
	if ok, err := c.TryAcquireRefreshCooldown(ctx, 8, time.Minute); err != nil || !ok {
		t.Fatalf("other carpool acquire: ok=%v err=%v", ok, err)
	}
}

func TestCarpoolUpstreamQuotaCache_NilRedisReturnsNil(t *testing.T) {
	if NewCarpoolUpstreamQuotaCache(nil) != nil {
		t.Fatal("nil redis must yield a nil cache (feature degraded, service skips it)")
	}
}

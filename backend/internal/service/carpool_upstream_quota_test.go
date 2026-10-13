//go:build unit

package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

const upstreamQuotaForecastBody = `{"code":200,"message":"OK","data":{
	"accountId":"acct_1",
	"generatedAt":"2026-10-12T08:00:00Z",
	"forecasts":[
		{"period":"monthly","targetDays":30,"extrapolated":true,"source":null},
		{"period":"weekly","targetDays":7,"extrapolated":true,
		 "source":{"label":"codex","usedPercent":7.0,"usedPercentDisplay":"7.0%",
			"observedAt":"2026-10-12T07:55:00Z","observedAtDisplay":"10-12 07:55",
			"resetAt":"2026-10-13T10:00:00Z","tokensDisplay":"116.6M","usdDisplay":"$163.06"},
		 "unavailableReason":null,"lowSample":false,"incompleteCost":false,"incompleteTokens":false,
		 "estimatedTokens":1536625298,"estimatedTokensDisplay":"1.5B",
		 "estimatedUsd":2282.92,"estimatedUsdDisplay":"2282.92"}
	]}}`

type upstreamQuotaRepoStub struct {
	carpool      *service.Carpool
	cprAccountID string
}

func (s *upstreamQuotaRepoStub) GetByID(ctx context.Context, carpoolID, userID int64) (*service.Carpool, error) {
	return s.carpool, nil
}

func (s *upstreamQuotaRepoStub) GetCprAccountID(ctx context.Context, carpoolID int64) (string, error) {
	return s.cprAccountID, nil
}

type upstreamQuotaFixture struct {
	svc           *service.CarpoolUpstreamQuotaService
	forecastCalls *atomic.Int32
	refreshCalls  *atomic.Int32
	mini          *miniredis.Miniredis
}

// newUpstreamQuotaFixture 搭一套完整服务：miniredis 缓存 + httptest 假 cpr + stub repo。
func newUpstreamQuotaFixture(t *testing.T, stub *upstreamQuotaRepoStub) *upstreamQuotaFixture {
	t.Helper()
	var forecastCalls, refreshCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/admin/accounts/quota-forecast":
			forecastCalls.Add(1)
			_, _ = w.Write([]byte(upstreamQuotaForecastBody))
		case "/api/admin/accounts/quota/refresh":
			refreshCalls.Add(1)
			_, _ = w.Write([]byte(`{"code":200,"message":"OK","data":{"ok":true}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cfg := &config.Config{}
	cfg.Cpr.AdminBaseURL = srv.URL
	cfg.Cpr.AdminKey = "test-admin-key"
	cfg.Cpr.CacheTTLSeconds = 300
	cfg.Cpr.RefreshCooldownSeconds = 60

	return &upstreamQuotaFixture{
		svc:           service.NewCarpoolUpstreamQuotaService(stub, service.NewCprClient(cfg),
			repository.NewCarpoolUpstreamQuotaCache(rdb), cfg),
		forecastCalls: &forecastCalls,
		refreshCalls:  &refreshCalls,
		mini:          mr,
	}
}

func memberCarpool(id int64) *service.Carpool {
	role := "member"
	return &service.Carpool{
		ID:             id,
		Visibility:     service.CarpoolVisibilityInviteOnly,
		Status:         "active",
		MemberRole:     &role,
		WeeklyLimitUSD: 2400,
	}
}

// 成员首查：打 cpr、组装 weekly、cached:false 并回填缓存；二查走缓存 cached:true 不再打 cpr。
func TestCarpoolUpstreamQuota_GetMemberCacheSecondRead(t *testing.T) {
	f := newUpstreamQuotaFixture(t, &upstreamQuotaRepoStub{carpool: memberCarpool(7), cprAccountID: "acct_1"})
	ctx := context.Background()

	first, err := f.svc.Get(ctx, 7, 11, false)
	require.NoError(t, err)
	require.True(t, first.Configured)
	require.Equal(t, int64(7), first.CarpoolID)
	require.Equal(t, "acct_1", first.CprAccountID)
	require.False(t, first.Cached)
	require.Equal(t, "2026-10-12T08:00:00Z", first.GeneratedAt)
	require.Equal(t, 60, first.Refresh.CooldownSeconds)
	require.NotNil(t, first.Weekly)
	require.True(t, first.Weekly.Available)
	require.NotNil(t, first.Weekly.UsedPercent)
	require.Equal(t, 7.0, *first.Weekly.UsedPercent)
	require.Equal(t, "7.0%", first.Weekly.UsedPercentDisplay)
	require.Equal(t, "2026-10-13T10:00:00Z", first.Weekly.ResetAt)
	require.Equal(t, "116.6M", first.Weekly.TokensDisplay)
	require.Equal(t, "$163.06", first.Weekly.USDDisplay)
	require.NotNil(t, first.Weekly.EstimatedTokens)
	require.Equal(t, uint64(1536625298), *first.Weekly.EstimatedTokens)
	require.EqualValues(t, 1, f.forecastCalls.Load())

	second, err := f.svc.Get(ctx, 7, 11, false)
	require.NoError(t, err)
	require.True(t, second.Cached, "second read must be served from the Redis snapshot")
	require.EqualValues(t, 1, f.forecastCalls.Load(), "cached read must not query cpr again")
}

// 未配置 cpr 账号映射：返回 configured:false 的成功响应，不打 cpr、不写缓存。
func TestCarpoolUpstreamQuota_GetUnconfiguredMapping(t *testing.T) {
	f := newUpstreamQuotaFixture(t, &upstreamQuotaRepoStub{carpool: memberCarpool(7)})
	result, err := f.svc.Get(context.Background(), 7, 11, false)
	require.NoError(t, err)
	require.False(t, result.Configured)
	require.Equal(t, int64(7), result.CarpoolID)
	require.Empty(t, result.CprAccountID)
	require.Nil(t, result.Weekly)
	require.Zero(t, f.forecastCalls.Load())
	require.Empty(t, f.mini.Keys())
}

// 可见性与 GetRoster 同一口径：invite_only 车的非成员（非 admin、非车主）403；
// public 招募中的车未上车者也可见。
func TestCarpoolUpstreamQuota_VisibilityFollowsRosterGate(t *testing.T) {
	stranger := &service.Carpool{
		ID:         7,
		Visibility: service.CarpoolVisibilityInviteOnly,
		Status:     "recruiting",
	}
	f := newUpstreamQuotaFixture(t, &upstreamQuotaRepoStub{carpool: stranger, cprAccountID: "acct_1"})
	_, err := f.svc.Get(context.Background(), 7, 99, false)
	require.ErrorIs(t, err, service.ErrCarpoolForbidden)
	require.Zero(t, f.forecastCalls.Load(), "forbidden read must not query cpr")

	publicRecruiting := &service.Carpool{
		ID:         8,
		Visibility: service.CarpoolVisibilityPublic,
		Status:     "recruiting",
	}
	f2 := newUpstreamQuotaFixture(t, &upstreamQuotaRepoStub{carpool: publicRecruiting, cprAccountID: "acct_1"})
	got, err := f2.svc.Get(context.Background(), 8, 99, false)
	require.NoError(t, err)
	require.True(t, got.Configured, "public recruiting carpool stays readable before boarding")

	ownerID := int64(42)
	owned := &service.Carpool{
		ID:          9,
		Visibility:  service.CarpoolVisibilityInviteOnly,
		Status:      "ended",
		OwnerUserID: &ownerID,
	}
	f3 := newUpstreamQuotaFixture(t, &upstreamQuotaRepoStub{carpool: owned, cprAccountID: "acct_1"})
	got, err = f3.svc.Get(context.Background(), 9, 42, false)
	require.NoError(t, err)
	require.True(t, got.Configured, "owner keeps read access after launch/end")
}

// cpr 未配置（admin_base_url 空）：503 NOT_CONFIGURED，与 Kasm 禁用同一惯例。
func TestCarpoolUpstreamQuota_NotConfigured503(t *testing.T) {
	svc := service.NewCarpoolUpstreamQuotaService(
		&upstreamQuotaRepoStub{carpool: memberCarpool(7), cprAccountID: "acct_1"}, nil, nil, nil)
	_, err := svc.Get(context.Background(), 7, 11, false)
	require.ErrorIs(t, err, service.ErrCarpoolUpstreamQuotaNotConfigured)
	require.Equal(t, http.StatusServiceUnavailable, infraerrors.Code(err))
	_, err = svc.Refresh(context.Background(), 7, 11, false)
	require.ErrorIs(t, err, service.ErrCarpoolUpstreamQuotaNotConfigured)
}

// 强制刷新：触发 cpr refresh + 拉新快照；冷却窗口内二次刷新 429 且不再触发 refresh。
func TestCarpoolUpstreamQuota_RefreshCooldown(t *testing.T) {
	f := newUpstreamQuotaFixture(t, &upstreamQuotaRepoStub{carpool: memberCarpool(7), cprAccountID: "acct_1"})
	ctx := context.Background()

	first, err := f.svc.Refresh(ctx, 7, 11, false)
	require.NoError(t, err)
	require.True(t, first.Configured)
	require.False(t, first.Cached)
	require.True(t, first.Weekly.Available)
	require.EqualValues(t, 1, f.refreshCalls.Load())
	require.EqualValues(t, 1, f.forecastCalls.Load())

	_, err = f.svc.Refresh(ctx, 7, 11, false)
	require.ErrorIs(t, err, service.ErrCarpoolUpstreamQuotaRefreshTooFrequent)
	require.Equal(t, http.StatusTooManyRequests, infraerrors.Code(err))
	require.EqualValues(t, 1, f.refreshCalls.Load(), "cooldown must block a second upstream refresh")

	// 冷却过期后允许再次刷新。
	f.mini.FastForward(2 * time.Minute)
	_, err = f.svc.Refresh(ctx, 7, 11, false)
	require.NoError(t, err)
	require.EqualValues(t, 2, f.refreshCalls.Load())
}

// refresh 之后 GET 应命中刚回填的新快照（不再打 forecast）。
func TestCarpoolUpstreamQuota_RefreshBackfillsCacheForGet(t *testing.T) {
	f := newUpstreamQuotaFixture(t, &upstreamQuotaRepoStub{carpool: memberCarpool(7), cprAccountID: "acct_1"})
	ctx := context.Background()

	_, err := f.svc.Refresh(ctx, 7, 11, false)
	require.NoError(t, err)
	got, err := f.svc.Get(ctx, 7, 11, false)
	require.NoError(t, err)
	require.True(t, got.Cached)
	require.EqualValues(t, 1, f.forecastCalls.Load())
}

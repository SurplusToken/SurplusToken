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

// 三张券：两张 available（10-15 / 10-13 到期）+ 一张已用。最近的 available 到期是 10-13。
const upstreamQuotaResetCreditsBody = `{"code":200,"message":"OK","data":{
	"availableCount":2,
	"credits":[
		{"id":"c1","status":"available","title":"Full reset","expiresAt":"2026-10-15T10:00:00Z","expiresAtDisplay":"10-15 10:00","resetType":"codex_rate_limits"},
		{"id":"c2","status":"available","title":"Full reset","expiresAt":"2026-10-13T10:00:00Z","expiresAtDisplay":"10-13 10:00","resetType":"codex_rate_limits"},
		{"id":"c3","status":"used","title":"Full reset","expiresAt":"2026-10-10T10:00:00Z","expiresAtDisplay":"10-10 10:00","resetType":"codex_rate_limits"}
	]}}`

// subscription 观测：10-15 到期、自动续费。profile 部分客户端不应消费。
const upstreamQuotaPersonalInfoBody = `{"code":200,"message":"OK","data":{
	"subscription":{"startsAt":"2026-09-12T00:00:00Z","expiresAt":"2026-10-15T00:00:00Z",
		"willRenew":true,"billingPeriod":"monthly","billingCurrency":"USD",
		"observedAt":"2026-10-12T08:00:00Z"}}}`

type upstreamQuotaRepoStub struct {
	carpool       *service.Carpool
	cprAccountID  string
	cprAccountIDs map[int64]string
}

func (s *upstreamQuotaRepoStub) GetByID(ctx context.Context, carpoolID, userID int64) (*service.Carpool, error) {
	return s.carpool, nil
}

func (s *upstreamQuotaRepoStub) GetCprAccountID(ctx context.Context, carpoolID int64) (string, error) {
	return s.cprAccountID, nil
}

func (s *upstreamQuotaRepoStub) ListCprAccountIDs(ctx context.Context, carpoolIDs []int64) (map[int64]string, error) {
	return s.cprAccountIDs, nil
}

type upstreamQuotaFixture struct {
	svc               *service.CarpoolUpstreamQuotaService
	forecastCalls     *atomic.Int32
	refreshCalls      *atomic.Int32
	resetCreditsCalls *atomic.Int32
	subscriptionCalls *atomic.Int32
	mini              *miniredis.Miniredis
}

// newUpstreamQuotaFixture 搭一套完整服务：miniredis 缓存 + httptest 假 cpr + stub repo。
func newUpstreamQuotaFixture(t *testing.T, stub *upstreamQuotaRepoStub) *upstreamQuotaFixture {
	t.Helper()
	return newUpstreamQuotaFixtureWithHandlers(t, stub, upstreamQuotaHandlers{})
}

// newUpstreamQuotaFixtureWithResetCredits 允许个别用例替换 reset-credits 的响应
// （如模拟上游故障），其余端点行为与默认 fixture 一致。
func newUpstreamQuotaFixtureWithResetCredits(t *testing.T, stub *upstreamQuotaRepoStub, resetCreditsHandler http.HandlerFunc) *upstreamQuotaFixture {
	t.Helper()
	return newUpstreamQuotaFixtureWithHandlers(t, stub, upstreamQuotaHandlers{resetCredits: resetCreditsHandler})
}

// newUpstreamQuotaFixtureWithPersonalInfo 允许个别用例替换 personal-info 的响应。
func newUpstreamQuotaFixtureWithPersonalInfo(t *testing.T, stub *upstreamQuotaRepoStub, personalInfoHandler http.HandlerFunc) *upstreamQuotaFixture {
	t.Helper()
	return newUpstreamQuotaFixtureWithHandlers(t, stub, upstreamQuotaHandlers{personalInfo: personalInfoHandler})
}

// upstreamQuotaHandlers 覆盖个别 cpr 端点的响应；nil 的端点用默认 body。
type upstreamQuotaHandlers struct {
	resetCredits http.HandlerFunc
	personalInfo http.HandlerFunc
}

func newUpstreamQuotaFixtureWithHandlers(t *testing.T, stub *upstreamQuotaRepoStub, handlers upstreamQuotaHandlers) *upstreamQuotaFixture {
	t.Helper()
	resetCreditsHandler := handlers.resetCredits
	if resetCreditsHandler == nil {
		resetCreditsHandler = func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(upstreamQuotaResetCreditsBody))
		}
	}
	personalInfoHandler := handlers.personalInfo
	if personalInfoHandler == nil {
		personalInfoHandler = func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(upstreamQuotaPersonalInfoBody))
		}
	}
	var forecastCalls, refreshCalls, resetCreditsCalls, subscriptionCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/admin/accounts/quota-forecast":
			forecastCalls.Add(1)
			_, _ = w.Write([]byte(upstreamQuotaForecastBody))
		case "/api/admin/accounts/quota/refresh":
			refreshCalls.Add(1)
			_, _ = w.Write([]byte(`{"code":200,"message":"OK","data":{"ok":true}}`))
		case "/api/admin/accounts/reset-credits":
			resetCreditsCalls.Add(1)
			resetCreditsHandler(w, r)
		case "/api/admin/accounts/personal-info":
			subscriptionCalls.Add(1)
			personalInfoHandler(w, r)
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
		svc: service.NewCarpoolUpstreamQuotaService(stub, service.NewCprClient(cfg),
			repository.NewCarpoolUpstreamQuotaCache(rdb), cfg),
		forecastCalls:     &forecastCalls,
		refreshCalls:      &refreshCalls,
		resetCreditsCalls: &resetCreditsCalls,
		subscriptionCalls: &subscriptionCalls,
		mini:              mr,
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

// cpr 未配置（admin_base_url 空）：503 NOT_CONFIGURED（禁用功能惯例：503 而非 404）。
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

// reset credits 与 forecast 进同一个快照：首查回填 reset_credits，
// 二读命中缓存时字段原样返回且不再打 reset-credits。
func TestCarpoolUpstreamQuota_GetCachesResetCreditsWithSnapshot(t *testing.T) {
	f := newUpstreamQuotaFixture(t, &upstreamQuotaRepoStub{carpool: memberCarpool(7), cprAccountID: "acct_1"})
	ctx := context.Background()

	first, err := f.svc.Get(ctx, 7, 11, false)
	require.NoError(t, err)
	require.NotNil(t, first.ResetCredits)
	require.Equal(t, 2, first.ResetCredits.AvailableCount)
	require.NotNil(t, first.ResetCredits.NextExpiresAt)
	require.Equal(t, "2026-10-13T10:00:00Z", *first.ResetCredits.NextExpiresAt,
		"next expires must be the nearest available credit, ignoring used ones")
	require.EqualValues(t, 1, f.resetCreditsCalls.Load())

	second, err := f.svc.Get(ctx, 7, 11, false)
	require.NoError(t, err)
	require.True(t, second.Cached)
	require.NotNil(t, second.ResetCredits, "cached snapshot must carry reset credits")
	require.Equal(t, 2, second.ResetCredits.AvailableCount)
	require.Equal(t, "2026-10-13T10:00:00Z", *second.ResetCredits.NextExpiresAt)
	require.EqualValues(t, 1, f.resetCreditsCalls.Load(), "cached read must not query reset-credits again")
}

// reset-credits 接口故障是降级路径：主数据照常 200，reset_credits 为 null，
// 且下一次（缓存 TTL 内）读到的是同一份 null 快照。
func TestCarpoolUpstreamQuota_ResetCreditsFailureKeepsWeekly(t *testing.T) {
	f := newUpstreamQuotaFixtureWithResetCredits(t,
		&upstreamQuotaRepoStub{carpool: memberCarpool(7), cprAccountID: "acct_1"},
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"code":500,"message":"reset credits boom"}`))
		})

	got, err := f.svc.Get(context.Background(), 7, 11, false)
	require.NoError(t, err)
	require.True(t, got.Configured)
	require.Nil(t, got.ResetCredits)
	require.NotNil(t, got.Weekly)
	require.True(t, got.Weekly.Available, "weekly forecast must survive a reset-credits outage")
	require.EqualValues(t, 1, f.forecastCalls.Load())
	require.EqualValues(t, 1, f.resetCreditsCalls.Load())

	cached, err := f.svc.Get(context.Background(), 7, 11, false)
	require.NoError(t, err)
	require.True(t, cached.Cached)
	require.Nil(t, cached.ResetCredits)
}

// subscription 与 forecast/reset-credits 进同一个快照：首查回填 account_expiry，
// 二读命中缓存时字段原样返回且不再打 personal-info。
func TestCarpoolUpstreamQuota_GetCachesAccountExpiryWithSnapshot(t *testing.T) {
	f := newUpstreamQuotaFixture(t, &upstreamQuotaRepoStub{carpool: memberCarpool(7), cprAccountID: "acct_1"})
	ctx := context.Background()

	first, err := f.svc.Get(ctx, 7, 11, false)
	require.NoError(t, err)
	require.NotNil(t, first.AccountExpiry)
	require.Equal(t, "2026-10-15T00:00:00Z", first.AccountExpiry.ExpiresAt)
	require.NotNil(t, first.AccountExpiry.WillRenew)
	require.True(t, *first.AccountExpiry.WillRenew)
	require.EqualValues(t, 1, f.subscriptionCalls.Load())

	second, err := f.svc.Get(ctx, 7, 11, false)
	require.NoError(t, err)
	require.True(t, second.Cached)
	require.NotNil(t, second.AccountExpiry, "cached snapshot must carry account expiry")
	require.Equal(t, "2026-10-15T00:00:00Z", second.AccountExpiry.ExpiresAt)
	require.EqualValues(t, 1, f.subscriptionCalls.Load(), "cached read must not query personal-info again")
}

// personal-info 故障是降级路径：主数据照常 200，account_expiry 为 null。
func TestCarpoolUpstreamQuota_SubscriptionFailureKeepsWeekly(t *testing.T) {
	f := newUpstreamQuotaFixtureWithPersonalInfo(t,
		&upstreamQuotaRepoStub{carpool: memberCarpool(7), cprAccountID: "acct_1"},
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"code":500,"message":"personal info boom"}`))
		})

	got, err := f.svc.Get(context.Background(), 7, 11, false)
	require.NoError(t, err)
	require.True(t, got.Configured)
	require.Nil(t, got.AccountExpiry)
	require.NotNil(t, got.Weekly)
	require.True(t, got.Weekly.Available, "weekly forecast must survive a personal-info outage")
	require.EqualValues(t, 1, f.forecastCalls.Load())
	require.EqualValues(t, 1, f.subscriptionCalls.Load())
}

// data.subscription 为 null（账号无订阅观测）不是错误：account_expiry 置 null 且不记 WARN 路径。
func TestCarpoolUpstreamQuota_NullSubscriptionLeavesAccountExpiryEmpty(t *testing.T) {
	f := newUpstreamQuotaFixtureWithPersonalInfo(t,
		&upstreamQuotaRepoStub{carpool: memberCarpool(7), cprAccountID: "acct_1"},
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"code":200,"message":"OK","data":{"subscription":null}}`))
		})

	got, err := f.svc.Get(context.Background(), 7, 11, false)
	require.NoError(t, err)
	require.Nil(t, got.AccountExpiry)
	require.NotNil(t, got.Weekly)
}

// 列表摘要：缓存命中的车直接用快照（不再打 cpr），摘要带出周用量与账号到期。
func TestCarpoolUpstreamQuota_ListSummariesCacheHit(t *testing.T) {
	stub := &upstreamQuotaRepoStub{
		carpool:       memberCarpool(7),
		cprAccountID:  "acct_1",
		cprAccountIDs: map[int64]string{7: "acct_1"},
	}
	f := newUpstreamQuotaFixture(t, stub)
	ctx := context.Background()

	// 先由详情接口回填缓存，模拟「用户刚看过详情」的热路径。
	_, err := f.svc.Get(ctx, 7, 11, false)
	require.NoError(t, err)
	require.EqualValues(t, 1, f.forecastCalls.Load())
	require.EqualValues(t, 1, f.subscriptionCalls.Load())

	summaries, err := f.svc.ListSummaries(ctx, []int64{7})
	require.NoError(t, err)
	summary := summaries[7]
	require.NotNil(t, summary)
	require.True(t, summary.Configured)
	require.NotNil(t, summary.UsedPercent)
	require.Equal(t, 7.0, *summary.UsedPercent)
	require.NotNil(t, summary.ResetAt)
	require.Equal(t, "2026-10-13T10:00:00Z", *summary.ResetAt)
	require.NotNil(t, summary.ExpiresAt)
	require.Equal(t, "2026-10-15T00:00:00Z", *summary.ExpiresAt)
	require.EqualValues(t, 1, f.forecastCalls.Load(), "cache hit must not query cpr again")
	require.EqualValues(t, 1, f.subscriptionCalls.Load(), "cache hit must not query personal-info again")
}

// 列表摘要：未配置 cpr 账号映射的车返回 configured:false、其余字段 null，不打 cpr。
func TestCarpoolUpstreamQuota_ListSummariesUnconfiguredCar(t *testing.T) {
	stub := &upstreamQuotaRepoStub{
		carpool:       memberCarpool(7),
		cprAccountIDs: map[int64]string{}, // 7 号车不在映射里
	}
	f := newUpstreamQuotaFixture(t, stub)

	summaries, err := f.svc.ListSummaries(context.Background(), []int64{7})
	require.NoError(t, err)
	summary := summaries[7]
	require.NotNil(t, summary)
	require.False(t, summary.Configured)
	require.Nil(t, summary.UsedPercent)
	require.Nil(t, summary.ResetAt)
	require.Nil(t, summary.ExpiresAt)
	require.Zero(t, f.forecastCalls.Load(), "unconfigured car must not query cpr")
	require.Zero(t, f.subscriptionCalls.Load())
}

// 列表摘要：缓存 miss 的车串行直查 cpr 并回填缓存（下一辆车/下一次列表读到同一份快照）。
func TestCarpoolUpstreamQuota_ListSummariesMissFetchesAndBackfills(t *testing.T) {
	stub := &upstreamQuotaRepoStub{
		carpool:       memberCarpool(8),
		cprAccountID:  "acct_1",
		cprAccountIDs: map[int64]string{8: "acct_1"},
	}
	f := newUpstreamQuotaFixture(t, stub)
	ctx := context.Background()

	first, err := f.svc.ListSummaries(ctx, []int64{8})
	require.NoError(t, err)
	require.True(t, first[8].Configured)
	require.NotNil(t, first[8].UsedPercent)
	require.NotNil(t, first[8].ExpiresAt)
	require.EqualValues(t, 1, f.forecastCalls.Load())
	require.EqualValues(t, 1, f.subscriptionCalls.Load())

	second, err := f.svc.ListSummaries(ctx, []int64{8})
	require.NoError(t, err)
	require.True(t, second[8].Configured)
	require.EqualValues(t, 1, f.forecastCalls.Load(), "backfilled snapshot must serve the second list read")
}

// 列表摘要：cpr 功能整体未配置（admin_base_url 空）时返回空 map，列表按全部未配置处理。
func TestCarpoolUpstreamQuota_ListSummariesFeatureDisabled(t *testing.T) {
	svc := service.NewCarpoolUpstreamQuotaService(
		&upstreamQuotaRepoStub{carpool: memberCarpool(7), cprAccountID: "acct_1"}, nil, nil, nil)
	summaries, err := svc.ListSummaries(context.Background(), []int64{7})
	require.NoError(t, err)
	require.Empty(t, summaries)
}

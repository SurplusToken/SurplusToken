package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newOpenAIClientRestrictionTestContext(t *testing.T, userAgent string, body []byte) *gin.Context {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	c.Request.Header.Set("User-Agent", userAgent)
	return c
}

func openAIClientRestrictionTestAccounts(groupID int64) []Account {
	return []Account{
		{
			ID:          51001,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
			Extra: map[string]any{
				"codex_cli_only": true,
			},
		},
		{
			ID:          51002,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    10,
			GroupIDs:    []int64{groupID},
		},
	}
}

func TestOpenAIAccountScheduling_ClientRestrictionSkipsStickyAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.4"}`)

	for _, advancedEnabled := range []bool{false, true} {
		advancedEnabled := advancedEnabled
		t.Run(map[bool]string{false: "legacy", true: "advanced"}[advancedEnabled], func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			defer resetOpenAIAdvancedSchedulerSettingCacheForTest()

			ctx := context.Background()
			groupID := int64(510)
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.LoadBatchEnabled = advancedEnabled
			cfg.Gateway.OpenAIWS.LBTopK = 2
			cache := &schedulerTestGatewayCache{}
			svc := &OpenAIGatewayService{
				accountRepo:        schedulerTestOpenAIAccountRepo{accounts: openAIClientRestrictionTestAccounts(groupID)},
				cache:              cache,
				cfg:                cfg,
				rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService(map[bool]string{false: "false", true: "true"}[advancedEnabled]),
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
			}
			require.NoError(t, svc.BindStickySession(ctx, &groupID, "opencode-session", 51001))

			c := newOpenAIClientRestrictionTestContext(t, "opencode/1.17.18 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14", body)
			selection, _, err := svc.SelectAccountWithSchedulerForCapabilityForRequest(
				ctx,
				c,
				body,
				&groupID,
				"",
				"opencode-session",
				"gpt-5.4",
				nil,
				OpenAIUpstreamTransportAny,
				OpenAIEndpointCapabilityChatCompletions,
				false,
				false,
			)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.NotNil(t, selection.Account)
			require.Equal(t, int64(51002), selection.Account.ID)
			require.Equal(t, int64(51002), cache.sessionBindings["openai:opencode-session"])
		})
	}
}

func TestOpenAIAccountScheduling_ClientRestrictionSkipsPreviousResponseAccountBeforeAcquire(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	defer resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(511)
	body := []byte(`{"model":"gpt-5.4"}`)
	cfg := newSchedulerTestOpenAIWSV2Config()
	cfg.Gateway.OpenAIWS.LBTopK = 2
	acquireOrder := make([]int64, 0, 1)
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: openAIClientRestrictionTestAccounts(groupID)},
		cache:            &schedulerTestGatewayCache{},
		cfg:              cfg,
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
			acquireOrder: &acquireOrder,
		}),
	}
	store := svc.getOpenAIWSStateStore()
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_restricted", 51001, time.Hour))

	c := newOpenAIClientRestrictionTestContext(t, "opencode/1.17.18", body)
	selection, decision, err := svc.SelectAccountWithSchedulerForCapabilityForRequest(
		ctx,
		c,
		body,
		&groupID,
		"resp_restricted",
		"",
		"gpt-5.4",
		nil,
		OpenAIUpstreamTransportAny,
		OpenAIEndpointCapabilityChatCompletions,
		false,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(51002), selection.Account.ID)
	require.False(t, decision.StickyPreviousHit)
	require.NotContains(t, acquireOrder, int64(51001))
}

func TestOpenAIAccountScheduling_OfficialCodexCanSelectRestrictedAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	defer resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(512)
	body := []byte(`{"model":"gpt-5.4"}`)
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 2
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: openAIClientRestrictionTestAccounts(groupID)},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}
	c := newOpenAIClientRestrictionTestContext(t, "codex_cli_rs/0.98.0 (Linux; x86_64) unknown", body)
	c.Request.Header.Set("x-codex-window-id", "window-1")

	selection, _, err := svc.SelectAccountWithSchedulerForCapabilityForRequest(
		ctx,
		c,
		body,
		&groupID,
		"",
		"",
		"gpt-5.4",
		nil,
		OpenAIUpstreamTransportAny,
		OpenAIEndpointCapabilityChatCompletions,
		false,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(51001), selection.Account.ID)
}

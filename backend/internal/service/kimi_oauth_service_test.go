package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type kimiOAuthClientStub struct {
	authorization *KimiDeviceAuthorization
	pollResult    *KimiDeviceTokenResult
	refreshResult *KimiTokenInfo
	deviceCode    string
}

func (s *kimiOAuthClientStub) RequestDeviceAuthorization(context.Context, string) (*KimiDeviceAuthorization, error) {
	return s.authorization, nil
}

func (s *kimiOAuthClientStub) PollDeviceToken(_ context.Context, deviceCode, _ string) (*KimiDeviceTokenResult, error) {
	s.deviceCode = deviceCode
	return s.pollResult, nil
}

func (s *kimiOAuthClientStub) RefreshToken(context.Context, string, string) (*KimiTokenInfo, error) {
	return s.refreshResult, nil
}

func TestKimiOAuthServiceDeviceFlow(t *testing.T) {
	now := time.Now()
	client := &kimiOAuthClientStub{
		authorization: &KimiDeviceAuthorization{
			UserCode: "ABCD-EFGH", DeviceCode: "device-secret",
			VerificationURI:         "https://auth.kimi.com/device",
			VerificationURIComplete: "https://auth.kimi.com/device?code=ABCD-EFGH",
			ExpiresIn:               600, Interval: 3,
		},
		pollResult: &KimiDeviceTokenResult{Status: "success", Token: &KimiTokenInfo{
			AccessToken: "access", RefreshToken: "refresh", ExpiresAt: now.Add(time.Hour).Unix(),
		}},
	}
	service := NewKimiOAuthService(nil, client)
	authorization, err := service.StartDeviceAuthorization(context.Background(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, authorization.SessionID)
	require.Equal(t, "ABCD-EFGH", authorization.UserCode)
	require.NotContains(t, authorization.VerificationURIComplete, "device-secret")

	result, err := service.PollDeviceToken(context.Background(), authorization.SessionID)
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.Equal(t, "device-secret", client.deviceCode)
	credentials := service.BuildAccountCredentials(result.Token)
	require.Equal(t, KimiCodeBaseURL, credentials["base_url"])
	require.Equal(t, []string{"chat_completions"}, credentials["openai_capabilities"])
	_, err = service.PollDeviceToken(context.Background(), authorization.SessionID)
	require.Error(t, err, "terminal sessions must be removed")
}

func TestKimiTokenRefresherOnlyHandlesKimiOAuth(t *testing.T) {
	refresher := NewKimiTokenRefresher(nil)
	kimi := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"openai_compatible_provider": "kimi"}}
	openAI := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	require.True(t, refresher.CanRefresh(kimi))
	require.False(t, refresher.CanRefresh(openAI))
	require.False(t, NewOpenAITokenRefresher(nil, nil).CanRefresh(kimi))
}

func TestKimiUsageProgressAcceptsRemainingAndResetTime(t *testing.T) {
	resetAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	progress := kimiUsageProgress(map[string]any{
		"limit": "100", "remaining": "25", "resetTime": resetAt.Format(time.RFC3339),
	})
	require.Equal(t, int64(75), progress.UsedRequests)
	require.Equal(t, int64(100), progress.LimitRequests)
	require.InDelta(t, 75, progress.Utilization, 0.001)
	require.NotNil(t, progress.ResetsAt)
}

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardAsAnthropic_KimiCodeUsesNativeMessagesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"msg_kimi","type":"message","role":"assistant","model":"kimi-for-coding","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":12,"output_tokens":3}}`,
		)),
	}}
	nativeGateway := &GatewayService{
		cfg: &config.Config{Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		}},
		httpUpstream: upstream,
	}
	svc := &OpenAIGatewayService{nativeGateway: nativeGateway}
	account := &Account{
		ID:          120,
		Name:        "kimi",
		Platform:    PlatformKimi,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "kimi-oauth-token",
			"base_url":     "https://api.kimi.com/coding/v1",
		},
		Extra: map[string]any{"openai_compatible_provider": "kimi"},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	body := []byte(`{"model":"claude-sonnet-4-5","stream":false,"max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`)

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "kimi-for-coding")

	require.NoError(t, err)
	require.Equal(t, "https://api.kimi.com/coding/v1/messages", upstream.lastReq.URL.String())
	require.Equal(t, "kimi-for-coding", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "Bearer kimi-oauth-token", getHeaderRaw(upstream.lastReq.Header, "authorization"))
	require.Equal(t, "msg_kimi", gjson.Get(rec.Body.String(), "id").String())
	require.Equal(t, 12, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)
	require.Equal(t, kimiAnthropicMessagesEndpoint, result.UpstreamEndpoint)
}

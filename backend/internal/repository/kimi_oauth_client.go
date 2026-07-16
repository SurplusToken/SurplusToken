package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/imroc/req/v3"
)

const (
	kimiOAuthHost     = "https://auth.kimi.com"
	kimiOAuthClientID = "17e5f671-d194-4dfb-9706-5516cb48c098"
)

type kimiOAuthClient struct{}

func NewKimiOAuthClient() service.KimiOAuthClient {
	return &kimiOAuthClient{}
}

func (c *kimiOAuthClient) RequestDeviceAuthorization(ctx context.Context, proxyURL string) (*service.KimiDeviceAuthorization, error) {
	client, err := createKimiReqClient(proxyURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}
	form := url.Values{"client_id": {kimiOAuthClientID}}
	var result service.KimiDeviceAuthorization
	resp, err := client.R().
		SetContext(ctx).
		SetHeaders(kimiOAuthHeaders()).
		SetFormDataFromValues(form).
		SetSuccessResult(&result).
		Post(kimiOAuthHost + "/api/oauth/device_authorization")
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_REQUEST_FAILED", "device authorization request failed: %v", err)
	}
	if !resp.IsSuccessState() {
		return nil, kimiOAuthStatusError("KIMI_OAUTH_DEVICE_AUTHORIZATION_FAILED", "device authorization failed", resp)
	}
	return &result, nil
}

func (c *kimiOAuthClient) PollDeviceToken(ctx context.Context, deviceCode, proxyURL string) (*service.KimiDeviceTokenResult, error) {
	client, err := createKimiReqClient(proxyURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}
	form := url.Values{
		"client_id":   {kimiOAuthClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	resp, err := client.R().
		SetContext(ctx).
		SetHeaders(kimiOAuthHeaders()).
		SetFormDataFromValues(form).
		Post(kimiOAuthHost + "/api/oauth/token")
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_REQUEST_FAILED", "device token request failed: %v", err)
	}

	payload := map[string]any{}
	if err := json.Unmarshal(resp.Bytes(), &payload); err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_INVALID_TOKEN_RESPONSE", "decode token response: %v", err)
	}
	if resp.IsSuccessState() {
		info, err := kimiTokenInfoFromPayload(payload)
		if err != nil {
			return nil, err
		}
		return &service.KimiDeviceTokenResult{Status: "success", Token: info}, nil
	}

	errorCode, _ := payload["error"].(string)
	description := kimiOAuthErrorDescription(payload)
	switch errorCode {
	case "authorization_pending", "slow_down":
		return &service.KimiDeviceTokenResult{Status: "pending", Error: errorCode, Description: description}, nil
	case "expired_token":
		return &service.KimiDeviceTokenResult{Status: "expired", Error: errorCode, Description: description}, nil
	case "access_denied":
		return &service.KimiDeviceTokenResult{Status: "denied", Error: errorCode, Description: description}, nil
	default:
		return nil, kimiOAuthStatusError("KIMI_OAUTH_TOKEN_EXCHANGE_FAILED", "device token exchange failed", resp)
	}
}

func (c *kimiOAuthClient) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*service.KimiTokenInfo, error) {
	client, err := createKimiReqClient(proxyURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}
	form := url.Values{
		"client_id":     {kimiOAuthClientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	resp, err := client.R().
		SetContext(ctx).
		SetHeaders(kimiOAuthHeaders()).
		SetFormDataFromValues(form).
		Post(kimiOAuthHost + "/api/oauth/token")
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_REQUEST_FAILED", "token refresh request failed: %v", err)
	}
	if !resp.IsSuccessState() {
		return nil, kimiOAuthStatusError("KIMI_OAUTH_TOKEN_REFRESH_FAILED", "token refresh failed", resp)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(resp.Bytes(), &payload); err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_INVALID_TOKEN_RESPONSE", "decode refresh response: %v", err)
	}
	return kimiTokenInfoFromPayload(payload)
}

func createKimiReqClient(proxyURL string) (*req.Client, error) {
	return getSharedReqClient(reqClientOptions{ProxyURL: proxyURL, Timeout: 30 * time.Second})
}

func kimiOAuthHeaders() map[string]string {
	return map[string]string{
		"Accept":             "application/json",
		"User-Agent":         "surplusai-kimi-code/1.0",
		"X-Msh-Platform":     "kimi_code_cli",
		"X-Msh-Version":      "1.0.0",
		"X-Msh-Device-Name":  "SurplusAI",
		"X-Msh-Device-Model": "server",
		"X-Msh-Os-Version":   "server",
		"X-Msh-Device-Id":    "surplusai-server",
	}
}

func kimiTokenInfoFromPayload(payload map[string]any) (*service.KimiTokenInfo, error) {
	accessToken, _ := payload["access_token"].(string)
	refreshToken, _ := payload["refresh_token"].(string)
	expiresIn := int64(0)
	switch value := payload["expires_in"].(type) {
	case float64:
		expiresIn = int64(value)
	case json.Number:
		expiresIn, _ = value.Int64()
	}
	if strings.TrimSpace(accessToken) == "" || expiresIn <= 0 {
		return nil, infraerrors.New(http.StatusBadGateway, "KIMI_OAUTH_INVALID_TOKEN_RESPONSE", "OAuth response missing access_token or expires_in")
	}
	tokenType, _ := payload["token_type"].(string)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	scope, _ := payload["scope"].(string)
	return &service.KimiTokenInfo{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    tokenType,
		Scope:        scope,
		ExpiresIn:    expiresIn,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second).Unix(),
	}, nil
}

func kimiOAuthErrorDescription(payload map[string]any) string {
	for _, key := range []string{"error_description", "message", "detail"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func kimiOAuthStatusError(code, message string, resp *req.Response) error {
	status := http.StatusBadGateway
	upstreamStatus := 0
	body := ""
	if resp != nil {
		upstreamStatus = resp.StatusCode
		body = logredact.RedactText(resp.String())
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			status = http.StatusUnauthorized
		}
	}
	return infraerrors.Newf(status, code, "%s: status %d, body: %s", message, upstreamStatus, body)
}

package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	KimiCodeBaseURL = "https://api.kimi.com/coding/v1"
	kimiSessionTTL  = 15 * time.Minute
)

type KimiDeviceAuthorization struct {
	UserCode                string `json:"user_code"`
	DeviceCode              string `json:"device_code,omitempty"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

type KimiTokenInfo struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	Scope        string `json:"scope,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
}

type KimiDeviceTokenResult struct {
	Status      string         `json:"status"`
	Error       string         `json:"error,omitempty"`
	Description string         `json:"description,omitempty"`
	Token       *KimiTokenInfo `json:"token,omitempty"`
}

type KimiDeviceAuthorizationResult struct {
	SessionID               string `json:"session_id"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

type kimiDeviceSession struct {
	deviceCode string
	proxyURL   string
	expiresAt  time.Time
}

type KimiOAuthService struct {
	proxyRepo   ProxyRepository
	oauthClient KimiOAuthClient
	mu          sync.Mutex
	sessions    map[string]kimiDeviceSession
}

func NewKimiOAuthService(proxyRepo ProxyRepository, oauthClient KimiOAuthClient) *KimiOAuthService {
	return &KimiOAuthService{
		proxyRepo:   proxyRepo,
		oauthClient: oauthClient,
		sessions:    make(map[string]kimiDeviceSession),
	}
}

func (s *KimiOAuthService) StartDeviceAuthorization(ctx context.Context, proxyID *int64) (*KimiDeviceAuthorizationResult, error) {
	if s == nil || s.oauthClient == nil {
		return nil, infraerrors.New(http.StatusServiceUnavailable, "KIMI_OAUTH_NOT_CONFIGURED", "Kimi OAuth is not configured")
	}
	proxyURL, err := s.proxyURL(ctx, proxyID)
	if err != nil {
		return nil, err
	}
	auth, err := s.oauthClient.RequestDeviceAuthorization(ctx, proxyURL)
	if err != nil {
		return nil, err
	}
	if auth == nil || strings.TrimSpace(auth.DeviceCode) == "" || strings.TrimSpace(auth.UserCode) == "" {
		return nil, infraerrors.New(http.StatusBadGateway, "KIMI_OAUTH_INVALID_DEVICE_RESPONSE", "Kimi OAuth returned an incomplete device authorization response")
	}

	sessionID, err := newKimiSessionID()
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "KIMI_OAUTH_SESSION_FAILED", "generate session ID: %v", err)
	}
	expiresIn := auth.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = int64(kimiSessionTTL.Seconds())
	}
	interval := auth.Interval
	if interval <= 0 {
		interval = 5
	}

	s.mu.Lock()
	s.pruneExpiredSessionsLocked(time.Now())
	s.sessions[sessionID] = kimiDeviceSession{
		deviceCode: auth.DeviceCode,
		proxyURL:   proxyURL,
		expiresAt:  time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	s.mu.Unlock()

	return &KimiDeviceAuthorizationResult{
		SessionID:               sessionID,
		UserCode:                auth.UserCode,
		VerificationURI:         auth.VerificationURI,
		VerificationURIComplete: auth.VerificationURIComplete,
		ExpiresIn:               expiresIn,
		Interval:                interval,
	}, nil
}

func (s *KimiOAuthService) PollDeviceToken(ctx context.Context, sessionID string) (*KimiDeviceTokenResult, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "KIMI_OAUTH_SESSION_REQUIRED", "session_id is required")
	}

	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	if ok && time.Now().After(session.expiresAt) {
		delete(s.sessions, sessionID)
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		return nil, infraerrors.New(http.StatusBadRequest, "KIMI_OAUTH_SESSION_NOT_FOUND", "device authorization session not found or expired")
	}

	result, err := s.oauthClient.PollDeviceToken(ctx, session.deviceCode, session.proxyURL)
	if err != nil {
		return nil, err
	}
	if result != nil && result.Status == "success" &&
		(result.Token == nil || strings.TrimSpace(result.Token.RefreshToken) == "") {
		return nil, infraerrors.New(http.StatusBadGateway, "KIMI_OAUTH_INVALID_TOKEN_RESPONSE", "OAuth response missing refresh_token")
	}
	if result != nil && (result.Status == "success" || result.Status == "expired" || result.Status == "denied") {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
	}
	return result, nil
}

func (s *KimiOAuthService) RefreshAccountToken(ctx context.Context, account *Account) (*KimiTokenInfo, error) {
	if account == nil || !account.IsKimiOAuth() {
		return nil, infraerrors.New(http.StatusBadRequest, "KIMI_OAUTH_INVALID_ACCOUNT", "account is not a Kimi OAuth account")
	}
	refreshToken := strings.TrimSpace(account.GetCredential("refresh_token"))
	if refreshToken == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "KIMI_OAUTH_NO_REFRESH_TOKEN", "no refresh token available")
	}
	proxyURL, err := s.proxyURL(ctx, account.ProxyID)
	if err != nil {
		return nil, err
	}
	info, err := s.oauthClient.RefreshToken(ctx, refreshToken, proxyURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(info.RefreshToken) == "" {
		info.RefreshToken = refreshToken
	}
	return info, nil
}

func (s *KimiOAuthService) BuildAccountCredentials(tokenInfo *KimiTokenInfo) map[string]any {
	if tokenInfo == nil {
		return nil
	}
	credentials := map[string]any{
		"access_token":        tokenInfo.AccessToken,
		"expires_at":          time.Unix(tokenInfo.ExpiresAt, 0).UTC().Format(time.RFC3339),
		"base_url":            KimiCodeBaseURL,
		"openai_capabilities": []string{string(OpenAIEndpointCapabilityChatCompletions)},
	}
	if tokenInfo.RefreshToken != "" {
		credentials["refresh_token"] = tokenInfo.RefreshToken
	}
	if tokenInfo.TokenType != "" {
		credentials["token_type"] = tokenInfo.TokenType
	}
	if tokenInfo.Scope != "" {
		credentials["scope"] = tokenInfo.Scope
	}
	return credentials
}

func (s *KimiOAuthService) proxyURL(ctx context.Context, proxyID *int64) (string, error) {
	if proxyID == nil || s.proxyRepo == nil {
		return "", nil
	}
	proxy, err := s.proxyRepo.GetByID(ctx, *proxyID)
	if err != nil {
		return "", err
	}
	if proxy == nil {
		return "", errors.New("proxy not found")
	}
	return proxy.URL(), nil
}

func (s *KimiOAuthService) pruneExpiredSessionsLocked(now time.Time) {
	for id, session := range s.sessions {
		if now.After(session.expiresAt) {
			delete(s.sessions, id)
		}
	}
}

func newKimiSessionID() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func applyKimiCodeHeaders(header http.Header, accountID int64) {
	header.Set("User-Agent", "surplusai-kimi-code/1.0")
	header.Set("X-Msh-Platform", "kimi_code_cli")
	header.Set("X-Msh-Version", "1.0.0")
	header.Set("X-Msh-Device-Name", "SurplusAI")
	header.Set("X-Msh-Device-Model", "server")
	header.Set("X-Msh-Os-Version", "server")
	header.Set("X-Msh-Device-Id", fmt.Sprintf("surplusai-account-%d", accountID))
}

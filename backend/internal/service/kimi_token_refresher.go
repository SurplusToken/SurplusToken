package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

const kimiTokenRefreshSkew = time.Hour

type KimiTokenRefresher struct {
	oauthService KimiOAuthTokenService
}

func NewKimiTokenRefresher(oauthService KimiOAuthTokenService) *KimiTokenRefresher {
	return &KimiTokenRefresher{oauthService: oauthService}
}

func (r *KimiTokenRefresher) CacheKey(account *Account) string {
	if account == nil {
		return "kimi:account:0"
	}
	return "kimi:account:" + strconv.FormatInt(account.ID, 10)
}

func (r *KimiTokenRefresher) CanRefresh(account *Account) bool {
	return account != nil && account.IsKimiOAuth()
}

func (r *KimiTokenRefresher) NeedsRefresh(account *Account, refreshWindow time.Duration) bool {
	if account == nil || strings.TrimSpace(account.GetCredential("refresh_token")) == "" {
		return false
	}
	if refreshWindow < kimiTokenRefreshSkew {
		refreshWindow = kimiTokenRefreshSkew
	}
	expiresAt := account.GetCredentialAsTime("expires_at")
	return expiresAt == nil || time.Until(*expiresAt) < refreshWindow
}

func (r *KimiTokenRefresher) Refresh(ctx context.Context, account *Account) (map[string]any, error) {
	if r == nil || r.oauthService == nil {
		return nil, errors.New("Kimi OAuth service is not configured")
	}
	info, err := r.oauthService.RefreshAccountToken(ctx, account)
	if err != nil {
		return nil, err
	}
	credentials := MergeCredentials(account.Credentials, r.oauthService.BuildAccountCredentials(info))
	if baseURL := strings.TrimSpace(account.GetCredential("base_url")); baseURL != "" {
		credentials["base_url"] = baseURL
	}
	return credentials, nil
}

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// CprClient talks to a codex-proxy-rs (cpr) admin API. Auth is the
// `x-api-key: <admin key>` header (NOT Authorization Bearer). Every response
// uses the unified envelope {"code":200,"message":"OK","data":...}; code != 200
// is an error whose message is safe to surface to operators.
type CprClient struct {
	adminBaseURL string
	adminKey     string
	httpClient   *http.Client
}

// CprQuotaForecastData is the `data` field of GET /api/admin/accounts/quota-forecast.
type CprQuotaForecastData struct {
	AccountID   string             `json:"accountId"`
	GeneratedAt string             `json:"generatedAt"`
	Forecasts   []CprQuotaForecast `json:"forecasts"`
}

// CprQuotaForecast is one forecast period entry (period "weekly" | "monthly").
// Source may be null when cpr has no observation for the window; the *Display
// fields are cpr's pre-formatted strings and are passed through as-is.
type CprQuotaForecast struct {
	Period                 string                  `json:"period"`
	TargetDays             float64                 `json:"targetDays"`
	Extrapolated           bool                    `json:"extrapolated"`
	Source                 *CprQuotaForecastSource `json:"source"`
	UnavailableReason      *string                 `json:"unavailableReason"`
	LowSample              bool                    `json:"lowSample"`
	IncompleteCost         bool                    `json:"incompleteCost"`
	IncompleteTokens       bool                    `json:"incompleteTokens"`
	EstimatedTokens        *uint64                 `json:"estimatedTokens"`
	EstimatedTokensDisplay string                  `json:"estimatedTokensDisplay"`
	EstimatedUSD           *float64                `json:"estimatedUsd"`
	EstimatedUSDDisplay    string                  `json:"estimatedUsdDisplay"`
}

// CprResetCreditsData is the `data` field of GET /api/admin/accounts/reset-credits.
type CprResetCreditsData struct {
	AvailableCount int              `json:"availableCount"`
	Credits        []CprResetCredit `json:"credits"`
}

// CprResetCredit is one reset credit entry. Only status=="available" entries count
// towards the remaining resets; ExpiresAt is RFC3339.
type CprResetCredit struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Title     string `json:"title"`
	ExpiresAt string `json:"expiresAt"`
	ResetType string `json:"resetType"`
}

// CprPersonalInfoData is the `data` field of GET /api/admin/accounts/personal-info.
// Only the subscription block is consumed; the rest of the profile is ignored.
type CprPersonalInfoData struct {
	Subscription *CprSubscription `json:"subscription"`
}

// CprSubscription is the account's subscription observation from cpr
// (RFC3339 timestamps). WillRenew is a pointer because "no subscription on file"
// (null subscription) and "renewal state unknown" must stay distinguishable.
type CprSubscription struct {
	StartsAt        string `json:"startsAt"`
	ExpiresAt       string `json:"expiresAt"`
	WillRenew       *bool  `json:"willRenew"`
	BillingPeriod   string `json:"billingPeriod"`
	BillingCurrency string `json:"billingCurrency"`
	ObservedAt      string `json:"observedAt"`
}

// CprAccountSubscription 是拼车展示需要的 subscription 摘要。
type CprAccountSubscription struct {
	ExpiresAt string
	WillRenew *bool
}

// CprQuotaForecastSource is the observation a forecast extrapolates from.
type CprQuotaForecastSource struct {
	Label              string   `json:"label"`
	UsedPercent        *float64 `json:"usedPercent"`
	UsedPercentDisplay string   `json:"usedPercentDisplay"`
	ObservedAt         string   `json:"observedAt"`
	ObservedAtDisplay  string   `json:"observedAtDisplay"`
	ResetAt            string   `json:"resetAt"`
	TokensDisplay      string   `json:"tokensDisplay"`
	USDDisplay         string   `json:"usdDisplay"`
}

// NewCprClient builds a CprClient from app config. Returns nil when the feature
// is not configured (empty cpr.admin_base_url) so callers can treat it as disabled.
func NewCprClient(cfg *config.Config) *CprClient {
	if cfg == nil {
		return nil
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.Cpr.AdminBaseURL), "/")
	if base == "" {
		return nil
	}
	return &CprClient{
		adminBaseURL: base,
		adminKey:     strings.TrimSpace(cfg.Cpr.AdminKey),
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// cprEnvelope is the unified cpr response envelope.
type cprEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// do sends one admin-API request and validates the envelope. out (may be nil)
// receives the envelope's data field.
func (c *CprClient) do(ctx context.Context, method, path string, body any, out any) error {
	if c == nil {
		return fmt.Errorf("cpr client not configured")
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("cpr marshal %s: %w", path, err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.adminBaseURL+path, reader)
	if err != nil {
		return fmt.Errorf("cpr new request %s: %w", path, err)
	}
	req.Header.Set("x-api-key", c.adminKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cpr do %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Cap the body to avoid unbounded reads; the quota-forecast payload is a
	// small JSON object, and the refresh response is large but never parsed.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("cpr read %s: %w", path, err)
	}

	var envelope cprEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("cpr decode %s: status %d: %w", path, resp.StatusCode, err)
	}
	if envelope.Code != http.StatusOK {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = fmt.Sprintf("unexpected status %d", resp.StatusCode)
		}
		return fmt.Errorf("cpr %s: %s", path, message)
	}
	if out != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("cpr decode %s data: %w", path, err)
		}
	}
	return nil
}

// QuotaForecast fetches the quota forecast for one cpr account. The caller picks
// the period=="weekly" entry from Forecasts.
func (c *CprClient) QuotaForecast(ctx context.Context, accountID string) (*CprQuotaForecastData, error) {
	if c == nil {
		return nil, fmt.Errorf("cpr client not configured")
	}
	path := "/api/admin/accounts/quota-forecast?accountId=" + url.QueryEscape(accountID)
	var out CprQuotaForecastData
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResetCredits fetches the account's rate-limit reset credits. This is a
// best-effort companion of QuotaForecast: callers should degrade to a null
// field instead of failing the whole request when this errors.
func (c *CprClient) ResetCredits(ctx context.Context, accountID string) (*CprResetCreditsData, error) {
	if c == nil {
		return nil, fmt.Errorf("cpr client not configured")
	}
	path := "/api/admin/accounts/reset-credits?accountId=" + url.QueryEscape(accountID)
	var out CprResetCreditsData
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PersonalInfo fetches the account's personal-info payload (GET /api/admin/accounts/personal-info).
// Only the subscription block is consumed by callers; the profile part is ignored.
func (c *CprClient) PersonalInfo(ctx context.Context, accountID string) (*CprPersonalInfoData, error) {
	if c == nil {
		return nil, fmt.Errorf("cpr client not configured")
	}
	path := "/api/admin/accounts/personal-info?accountId=" + url.QueryEscape(accountID)
	var out CprPersonalInfoData
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Subscription 拉 personal-info 并取出 subscription 块。账号没有订阅观测
// （data.subscription 为 null）时返回 (nil, nil)，不算错误——调用方据此把
// account_expiry 置空，而不是当作拉取失败。
func (c *CprClient) Subscription(ctx context.Context, accountID string) (*CprAccountSubscription, error) {
	data, err := c.PersonalInfo(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if data == nil || data.Subscription == nil {
		return nil, nil
	}
	return &CprAccountSubscription{
		ExpiresAt: data.Subscription.ExpiresAt,
		WillRenew: data.Subscription.WillRenew,
	}, nil
}

// RefreshQuota asks cpr to re-observe the account's upstream quota. The response
// body is huge and never parsed — the envelope code is the only signal.
func (c *CprClient) RefreshQuota(ctx context.Context, accountID string) error {
	if c == nil {
		return fmt.Errorf("cpr client not configured")
	}
	return c.do(ctx, http.MethodPost, "/api/admin/accounts/quota/refresh",
		map[string]any{"accountId": accountID}, nil)
}

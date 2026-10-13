package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestNewCprClientDisabled(t *testing.T) {
	if NewCprClient(nil) != nil {
		t.Fatal("nil config must yield a nil client")
	}
	if NewCprClient(&config.Config{}) != nil {
		t.Fatal("empty admin_base_url must yield a nil client (feature disabled)")
	}
	cfg := &config.Config{}
	cfg.Cpr.AdminBaseURL = " http://cpr:8321/ "
	client := NewCprClient(cfg)
	if client == nil {
		t.Fatal("configured admin_base_url must yield a client")
	}
	if client.adminBaseURL != "http://cpr:8321" {
		t.Errorf("adminBaseURL = %q, want trimmed base without trailing slash", client.adminBaseURL)
	}
}

// newCprTestClient returns a client pinned at the httptest server plus call counters.
func newCprTestClient(t *testing.T, handler http.HandlerFunc) (*CprClient, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return &CprClient{adminBaseURL: srv.URL, adminKey: "test-admin-key", httpClient: srv.Client()}, &calls
}

func TestCprQuotaForecastRoundTrip(t *testing.T) {
	client, calls := newCprTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/admin/accounts/quota-forecast" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("accountId") != "acct_1" {
			t.Errorf("accountId query = %q", r.URL.Query().Get("accountId"))
		}
		if r.Header.Get("x-api-key") != "test-admin-key" {
			t.Errorf("x-api-key header = %q", r.Header.Get("x-api-key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"OK","data":{
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
			]}}`))
	})

	data, err := client.QuotaForecast(context.Background(), "acct_1")
	if err != nil {
		t.Fatal(err)
	}
	if data.AccountID != "acct_1" || data.GeneratedAt != "2026-10-12T08:00:00Z" {
		t.Errorf("data = %+v", data)
	}
	if len(data.Forecasts) != 2 {
		t.Fatalf("forecasts = %d, want 2", len(data.Forecasts))
	}
	weekly := data.Forecasts[1]
	if weekly.Period != "weekly" || weekly.Source == nil || weekly.Source.UsedPercent == nil {
		t.Fatalf("weekly forecast = %+v", weekly)
	}
	if *weekly.Source.UsedPercent != 7.0 || weekly.Source.UsedPercentDisplay != "7.0%" {
		t.Errorf("usedPercent = %v / %q", weekly.Source.UsedPercent, weekly.Source.UsedPercentDisplay)
	}
	if weekly.EstimatedTokens == nil || *weekly.EstimatedTokens != 1536625298 {
		t.Errorf("estimatedTokens = %v", weekly.EstimatedTokens)
	}
	if weekly.EstimatedUSD == nil || *weekly.EstimatedUSD != 2282.92 {
		t.Errorf("estimatedUsd = %v", weekly.EstimatedUSD)
	}
	if calls.Load() != 1 {
		t.Errorf("cpr calls = %d, want 1", calls.Load())
	}
}

func TestCprQuotaForecastNullSource(t *testing.T) {
	client, _ := newCprTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":200,"message":"OK","data":{
			"accountId":"acct_2","generatedAt":"2026-10-12T08:00:00Z",
			"forecasts":[{"period":"weekly","targetDays":7,"extrapolated":false,"source":null,
				"unavailableReason":"no observation yet","lowSample":true,
				"estimatedTokens":null,"estimatedTokensDisplay":"-",
				"estimatedUsd":null,"estimatedUsdDisplay":"-"}]}}`))
	})
	data, err := client.QuotaForecast(context.Background(), "acct_2")
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Forecasts) != 1 || data.Forecasts[0].Source != nil {
		t.Fatalf("forecasts = %+v, want null source", data.Forecasts)
	}
	if data.Forecasts[0].UnavailableReason == nil || *data.Forecasts[0].UnavailableReason != "no observation yet" {
		t.Errorf("unavailableReason = %v", data.Forecasts[0].UnavailableReason)
	}
}

func TestCprEnvelopeErrorCarriesMessage(t *testing.T) {
	client, _ := newCprTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // cpr 逻辑错误也回 HTTP 200
		_, _ = w.Write([]byte(`{"code":404,"message":"account not found","data":null}`))
	})
	_, err := client.QuotaForecast(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "account not found") {
		t.Fatalf("err = %v, want cpr message surfaced", err)
	}
}

func TestCprNonJSONErrorResponse(t *testing.T) {
	client, _ := newCprTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`<html>bad gateway</html>`))
	})
	if _, err := client.QuotaForecast(context.Background(), "acct_1"); err == nil {
		t.Fatal("expected error for non-2xx non-JSON response")
	}
}

func TestCprRefreshQuota(t *testing.T) {
	client, calls := newCprTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/admin/accounts/quota/refresh" {
			t.Errorf("refresh request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-admin-key" {
			t.Errorf("x-api-key header = %q", r.Header.Get("x-api-key"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("refresh body decode: %v", err)
		}
		if body["accountId"] != "acct_1" {
			t.Errorf("refresh body = %v", body)
		}
		// 真实 refresh 响应体很大；这里只要求信封 code 被校验。
		_, _ = w.Write([]byte(`{"code":200,"message":"OK","data":{"ignored":true}}`))
	})

	if err := client.RefreshQuota(context.Background(), "acct_1"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Errorf("cpr calls = %d, want 1", calls.Load())
	}
}

func TestCprRefreshQuotaEnvelopeError(t *testing.T) {
	client, _ := newCprTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":500,"message":"refresh failed upstream"}`))
	})
	err := client.RefreshQuota(context.Background(), "acct_1")
	if err == nil || !strings.Contains(err.Error(), "refresh failed upstream") {
		t.Fatalf("err = %v, want cpr message surfaced", err)
	}
}

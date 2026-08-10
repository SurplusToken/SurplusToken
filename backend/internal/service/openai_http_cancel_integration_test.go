package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type localOpenAIHTTPUpstream struct {
	client   *http.Client
	endpoint *url.URL
	seen     chan context.Context
}

func (u *localOpenAIHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.seen <- req.Context()
	localReq := req.Clone(req.Context())
	localReq.URL.Scheme = u.endpoint.Scheme
	localReq.URL.Host = u.endpoint.Host
	localReq.Host = u.endpoint.Host
	return u.client.Do(localReq)
}

func (u *localOpenAIHTTPUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, "", 0, 0)
}

func TestOpenAIForwardClientCancelReleasesAccountHTTPConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var hits atomic.Int32

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			close(firstStarted)
			select {
			case <-r.Context().Done():
				return
			case <-releaseFirst:
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_second\",\"model\":\"gpt-5.6-sol\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n")
	}))
	defer upstreamServer.Close()
	defer close(releaseFirst)

	endpoint, err := url.Parse(upstreamServer.URL)
	require.NoError(t, err)
	transport := &http.Transport{MaxConnsPerHost: 1, MaxIdleConnsPerHost: 1}
	defer transport.CloseIdleConnections()
	seenRequestContexts := make(chan context.Context, 2)
	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}},
		httpUpstream: &localOpenAIHTTPUpstream{
			client:   &http.Client{Transport: transport},
			endpoint: endpoint,
			seen:     seenRequestContexts,
		},
	}
	account := &Account{
		ID: 1, Name: "fixed-oauth-account", Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token", "chatgpt_account_id": "test-account"},
	}
	body := []byte(`{"model":"gpt-5.6-sol","stream":true,"reasoning":{"effort":"high"},"input":"hello"}`)

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstRecorder := httptest.NewRecorder()
	firstGinCtx, _ := gin.CreateTestContext(firstRecorder)
	firstGinCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body)).WithContext(firstCtx)
	firstDone := make(chan error, 1)
	go func() {
		_, forwardErr := svc.Forward(firstCtx, firstGinCtx, account, body)
		firstDone <- forwardErr
	}()

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first request did not reach the local upstream")
	}
	cancelFirst()
	firstUpstreamCtx := <-seenRequestContexts
	require.Eventually(t, func() bool { return errors.Is(firstUpstreamCtx.Err(), context.Canceled) }, 500*time.Millisecond, 10*time.Millisecond)

	select {
	case forwardErr := <-firstDone:
		require.ErrorIs(t, forwardErr, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("canceled first forward did not return")
	}

	secondCtx, cancelSecond := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelSecond()
	secondRecorder := httptest.NewRecorder()
	secondGinCtx, _ := gin.CreateTestContext(secondRecorder)
	secondGinCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body)).WithContext(secondCtx)
	result, err := svc.Forward(secondCtx, secondGinCtx, account, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.EqualValues(t, 2, hits.Load())
}

//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type updatingProxyRepoStub struct {
	*proxyRepoStub
	proxy       *Proxy
	updateCalls int
}

func (s *updatingProxyRepoStub) GetByID(context.Context, int64) (*Proxy, error) {
	copy := *s.proxy
	return &copy, nil
}

func (s *updatingProxyRepoStub) Update(_ context.Context, proxy *Proxy) error {
	s.updateCalls++
	copy := *proxy
	s.proxy = &copy
	return nil
}

func TestAdminProxyUpdateUsesRepositoryUpdateBoundary(t *testing.T) {
	repo := &updatingProxyRepoStub{
		proxyRepoStub: &proxyRepoStub{},
		proxy: &Proxy{
			ID:             9,
			Protocol:       "http",
			Host:           "old.example",
			Port:           8080,
			Status:         StatusActive,
			FallbackMode:   FallbackModeNone,
			ExpiryWarnDays: 7,
		},
	}
	svc := &adminServiceImpl{proxyRepo: repo}
	warnDays := 7

	_, err := svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{
		Host:           "new.example",
		FallbackMode:   FallbackModeNone,
		ExpiryWarnDays: &warnDays,
	})

	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, "new.example", repo.proxy.Host)
}

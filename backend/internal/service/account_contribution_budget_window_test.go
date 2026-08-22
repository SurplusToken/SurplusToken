package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestContributionWeeklyBudgetWindowStartFollowsUpstreamReset(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, time.August, 9, 10, 0, 0, 0, location)
	resetAt := time.Date(2026, time.August, 16, 4, 31, 0, 0, location)
	account := &Account{Extra: map[string]any{
		"codex_7d_reset_at":       resetAt.Format(time.RFC3339),
		"codex_7d_window_minutes": 10080.0,
	}}

	expected := time.Date(2026, time.August, 9, 4, 31, 0, 0, location)
	require.True(t, expected.Equal(account.contributionWeeklyBudgetWindowStart(now)))
}

func TestContributionWeeklyBudgetWindowStartAdvancesExpiredBoundary(t *testing.T) {
	resetAt := time.Date(2026, time.August, 9, 4, 31, 0, 0, time.UTC)
	now := resetAt.Add(8*24*time.Hour + time.Hour)
	account := &Account{Extra: map[string]any{
		"codex_7d_reset_at":       resetAt.Format(time.RFC3339),
		"codex_7d_window_minutes": 10080.0,
	}}

	require.Equal(t, resetAt.Add(7*24*time.Hour), account.contributionWeeklyBudgetWindowStart(now))
}

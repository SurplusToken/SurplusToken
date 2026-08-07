package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEvaluateContributionProtectionBudgetModeExposesUsageWithoutPercentGate(t *testing.T) {
	now := time.Now()
	spend := 36.42
	account := &Account{
		OthersWeeklySpend: &spend,
		Extra: map[string]any{
			"contribution_share_mode":             ContributionShareModeBudget,
			"contribution_weekly_share_budget":    400.0,
			"contribution_5h_reserve_percent":     20.0,
			"contribution_weekly_reserve_percent": 70.0,
			"codex_5h_used_percent":               95.0,
			"codex_5h_reset_at":                   now.Add(4 * time.Hour).Format(time.RFC3339),
			"codex_7d_used_percent":               38.0,
			"codex_7d_reset_at":                   now.Add(5 * 24 * time.Hour).Format(time.RFC3339),
		},
	}

	evaluation := account.EvaluateContributionProtection()

	require.NotNil(t, evaluation.FiveHourUsagePercent)
	require.InDelta(t, 95.0, *evaluation.FiveHourUsagePercent, 0.001)
	require.NotNil(t, evaluation.WeeklyUsagePercent)
	require.InDelta(t, 38.0, *evaluation.WeeklyUsagePercent, 0.001)
	require.False(t, evaluation.Blocked)
	require.Empty(t, evaluation.Reason)
}

func TestEvaluateContributionProtectionBudgetModeKeepsUsageWhenBudgetExhausted(t *testing.T) {
	now := time.Now()
	spend := 400.0
	account := &Account{
		OthersWeeklySpend: &spend,
		Extra: map[string]any{
			"contribution_share_mode":          ContributionShareModeBudget,
			"contribution_weekly_share_budget": 400.0,
			"codex_7d_used_percent":            38.0,
			"codex_7d_reset_at":                now.Add(5 * 24 * time.Hour).Format(time.RFC3339),
		},
	}

	evaluation := account.EvaluateContributionProtection()

	require.NotNil(t, evaluation.WeeklyUsagePercent)
	require.InDelta(t, 38.0, *evaluation.WeeklyUsagePercent, 0.001)
	require.True(t, evaluation.Blocked)
	require.Equal(t, "weekly_budget_exhausted", evaluation.Reason)
}

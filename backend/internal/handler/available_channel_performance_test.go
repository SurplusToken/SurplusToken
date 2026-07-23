package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildUserModelPerformance_AggregatesWeightedStats(t *testing.T) {
	latencyFast, latencySlow := 100, 900
	ttftFast, ttftSlow := 40, 200
	got := buildUserModelPerformance([]service.ModelPerformanceStat{
		{
			Model: "gpt-5", GroupID: 1,
			SuccessCount: 9, FailureCount: 1,
			LatencySampleCount: 9, TTFTSampleCount: 9,
			AvgLatencyMs: &latencyFast, AvgTTFTMs: &ttftFast,
		},
		{
			Model: "gpt-5", GroupID: 2,
			SuccessCount: 1, FailureCount: 1,
			LatencySampleCount: 1, TTFTSampleCount: 1,
			AvgLatencyMs: &latencySlow, AvgTTFTMs: &ttftSlow,
		},
	})

	require.NotNil(t, got)
	require.Equal(t, int64(12), got.SampleCount)
	require.Equal(t, 83.33, got.SuccessRate)
	require.Equal(t, 180, *got.AvgLatencyMs)
	require.Equal(t, 56, *got.AvgTTFTMs)
	require.Len(t, got.GroupBreakdown, 2)
}

func TestAttachModelPerformanceStats_OnlyUsesSectionGroups(t *testing.T) {
	channels := []userAvailableChannel{{
		Platforms: []userChannelPlatformSection{{
			Groups:          []userAvailableGroup{{ID: 7}},
			SupportedModels: []userSupportedModel{{Name: "GPT-5", Platform: "openai"}},
		}},
	}}
	stats := []service.ModelPerformanceStat{
		{Model: "gpt-5", GroupID: 7, SuccessCount: 4, FailureCount: 1},
		{Model: "gpt-5", GroupID: 8, SuccessCount: 100},
	}

	attachModelPerformanceStats(channels, stats)
	got := channels[0].Platforms[0].SupportedModels[0].Performance
	require.NotNil(t, got)
	require.Equal(t, int64(5), got.SampleCount)
	require.Equal(t, 80.0, got.SuccessRate)
	require.Len(t, got.GroupBreakdown, 1)
	require.Equal(t, int64(7), got.GroupBreakdown[0].GroupID)
}

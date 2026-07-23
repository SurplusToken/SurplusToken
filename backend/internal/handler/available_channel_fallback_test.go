package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFilterUncoveredGroups_KeepsGroupsWithoutChannels(t *testing.T) {
	groups := []service.Group{
		{ID: 29, Name: "dynamic-kimi", Platform: service.PlatformKimi},
		{ID: 7, Name: "pro", Platform: service.PlatformOpenAI},
		{ID: 16, Name: "grok", Platform: service.PlatformGrok},
	}

	got := filterUncoveredGroups(groups, map[int64]struct{}{29: {}})

	require.Len(t, got, 2)
	require.Equal(t, int64(7), got[0].ID)
	require.Equal(t, int64(16), got[1].ID)
}

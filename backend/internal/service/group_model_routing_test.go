package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupGetRoutingAccountIDs_DeterministicMostSpecificWildcard(t *testing.T) {
	group := &Group{
		ModelRoutingEnabled: true,
		ModelRouting: map[string][]int64{
			"claude-*":      {1},
			"claude-opus-*": {2},
			"claude-opus-4": {3},
		},
	}

	for range 100 {
		require.Equal(t, []int64{3}, group.GetRoutingAccountIDs("claude-opus-4"), "exact match must win")
		require.Equal(t, []int64{2}, group.GetRoutingAccountIDs("claude-opus-4-20250514"), "longest wildcard prefix must win")
		require.Equal(t, []int64{1}, group.GetRoutingAccountIDs("claude-sonnet-4"))
		require.Nil(t, group.GetRoutingAccountIDs("gpt-5"))
	}
}

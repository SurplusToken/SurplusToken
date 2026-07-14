package repository

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// The api-key auth query loads the group with an explicit field projection
// (WithGroup + Select). Omitting dynamic_sharing_pool defaults it to false on
// the loaded group, which silently disables sharing-rate billing for every
// consumer (shouldApplySharingRateBilling can never see a dynamic pool). Guard
// against re-dropping the field from that projection.
func TestAPIKeyGroupProjectionIncludesDynamicSharingPool(t *testing.T) {
	src, err := os.ReadFile("api_key_repo.go")
	require.NoError(t, err)
	require.Contains(t, string(src), "group.FieldDynamicSharingPool",
		"api-key auth group projection must select dynamic_sharing_pool, otherwise dynamic-pool sharing-rate billing is silently disabled")
}

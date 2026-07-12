package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration175AddsConstrainedDynamicSharingPoolFlag(t *testing.T) {
	content, err := FS.ReadFile("175_group_dynamic_sharing_pool.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "dynamic_sharing_pool boolean not null default false")
	require.Contains(t, sql, "subscription_type = 'standard'")
	require.Contains(t, sql, "rate_multiplier = 1.0")
}

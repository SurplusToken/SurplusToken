package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration181AddsPerGroupSharingRateRanges(t *testing.T) {
	content, err := FS.ReadFile("181_user_group_sharing_rate_ranges.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Contains(t, sql, "add column if not exists sharing_rate_min decimal(10,4)")
	require.Contains(t, sql, "add column if not exists sharing_rate_max decimal(10,4)")
	require.Contains(t, sql, "on conflict (user_id, group_id) do update")
	require.Contains(t, sql, "groups.dynamic_sharing_pool = true")
	require.Contains(t, sql, "sharing_rate_min <= sharing_rate_max")
}

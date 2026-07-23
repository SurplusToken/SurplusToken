package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountModelPricingOverrideMigrationScopesRulesToBoundAccounts(t *testing.T) {
	content, err := FS.ReadFile("185_account_model_pricing_overrides.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE")
	require.Contains(t, sql, "account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE")
	require.Contains(t, sql, "account_model_pricing_override_intervals")
	require.Contains(t, sql, "CHECK (min_tokens >= 0 AND (max_tokens IS NULL OR max_tokens > min_tokens))")
}

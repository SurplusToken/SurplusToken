package migrations

import (
	"strings"
	"testing"
)

func TestCarpoolRuntimeMigrationDefinesAtomicLaunchRelations(t *testing.T) {
	raw, err := FS.ReadFile("182_carpool_runtime.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS carpools",
		"CREATE TABLE IF NOT EXISTS carpool_members",
		"REFERENCES user_subscriptions(id)",
		"usage_pool_cny_per_account",
		"idx_carpools_name_live_unique",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}

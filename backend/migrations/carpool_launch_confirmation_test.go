package migrations

import (
	"strings"
	"testing"
)

func TestCarpoolLaunchConfirmationMigrationDefinesColumns(t *testing.T) {
	raw, err := FS.ReadFile("188_carpool_launch_confirmation.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, fragment := range []string{
		"group_qr_code BYTEA NULL",
		"group_qr_code_content_type VARCHAR(50) NULL",
		"added_admin_wechat BOOLEAN NOT NULL DEFAULT false",
		"launch_notified_at TIMESTAMPTZ NULL",
		"confirmed_at TIMESTAMPTZ NULL",
		"confirmed_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL",
		"joined_wechat_group BOOLEAN NOT NULL DEFAULT false",
		"DROP CONSTRAINT IF EXISTS carpools_status_check",
		"CHECK (status IN ('recruiting', 'confirmed', 'starting', 'active', 'cancelled', 'ended'))",
		"DROP INDEX IF EXISTS idx_carpools_name_live_unique",
		"WHERE status IN ('recruiting', 'confirmed', 'starting', 'active')",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}

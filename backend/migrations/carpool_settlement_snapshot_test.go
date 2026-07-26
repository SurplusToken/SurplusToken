package migrations

import (
	"strings"
	"testing"
)

// 191 让 187 留下的 settled_at 真正有用：结算时把每一行金额冻结下来。
// 没有这些列，结算单每次读都按当下用量重算——车主按 A 收的钱，第二天读到 B。
func TestCarpoolSettlementSnapshotMigration(t *testing.T) {
	raw, err := FS.ReadFile("191_carpool_settlement_snapshot.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, fragment := range []string{
		"ALTER TABLE carpool_members",
		"ADD COLUMN IF NOT EXISTS settled_by_user_id BIGINT NULL",
		"ADD COLUMN IF NOT EXISTS settled_floor_usage_usd DECIMAL(20, 10) NULL",
		"ADD COLUMN IF NOT EXISTS settled_actual_usage_usd DECIMAL(20, 10) NULL",
		"ADD COLUMN IF NOT EXISTS settled_billable_usage_usd DECIMAL(20, 10) NULL",
		"ADD COLUMN IF NOT EXISTS settled_usage_share_cny DECIMAL(20, 2) NULL",
		"ADD COLUMN IF NOT EXISTS settled_seat_fee_cny DECIMAL(20, 2) NULL",
		"ADD COLUMN IF NOT EXISTS settled_total_delta_cny DECIMAL(20, 2) NULL",
		"ALTER TABLE carpools",
		"ADD COLUMN IF NOT EXISTS settled_at TIMESTAMPTZ NULL",
		"CREATE INDEX IF NOT EXISTS idx_carpools_settled_at",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}

package migrations

import (
	"strings"
	"testing"
)

func TestCarpoolQuotaReservationMigrationDefinesReservationColumns(t *testing.T) {
	raw, err := FS.ReadFile("187_carpool_quota_reservation.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, fragment := range []string{
		"weekly_limit_usd DECIMAL(20, 10) NOT NULL DEFAULT 2400",
		"seat_fee_cny DECIMAL(20, 2) NOT NULL DEFAULT 400",
		"usage_pool_cny DECIMAL(20, 2) NOT NULL DEFAULT 1000",
		"reserve_ratio DECIMAL(3, 2) NOT NULL DEFAULT 0.80",
		"launch_min_ratio DECIMAL(4, 3) NOT NULL DEFAULT 0.950",
		"launch_max_ratio DECIMAL(4, 3) NOT NULL DEFAULT 1.050",
		"declared_weekly_quota_usd DECIMAL(20, 10) NOT NULL DEFAULT 0",
		"prepaid_amount_cny DECIMAL(20, 2) NULL",
		"settled_at TIMESTAMPTZ NULL",
		"ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS weekly_limit_usd DECIMAL(20, 10) NULL",
		"ALTER TABLE carpools ALTER COLUMN capacity DROP NOT NULL",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}

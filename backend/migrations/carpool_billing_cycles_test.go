package migrations

import (
	"strings"
	"testing"
)

// 193 是按周计地板的前提：weekly_usage_usd 每周被清零，不落台账就永远拿不回
// 历史周用量，只能退回按月整体算——那会让超用的周补贴没用满的周。
func TestCarpoolBillingCyclesMigration(t *testing.T) {
	raw, err := FS.ReadFile("193_carpool_billing_cycles.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS carpool_billing_cycles",
		"cycle_start               TIMESTAMPTZ NOT NULL",
		"cycle_end                 TIMESTAMPTZ NOT NULL",
		"declared_weekly_quota_usd",
		"reserved_usd",
		"actual_usage_usd",
		"billable_usage_usd",
		// 幂等守卫：周重置可能被并发触发，没有唯一索引会重复计费
		"CREATE UNIQUE INDEX IF NOT EXISTS uq_carpool_billing_cycles_sub_start",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}

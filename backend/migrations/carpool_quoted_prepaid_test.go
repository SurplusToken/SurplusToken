package migrations

import (
	"strings"
	"testing"
)

// 190 必须新增报价列并回填历史行：没有这一列时，结算里的席位费"找齐"分支
// 恒等于 0（发车会把 prepaid_amount_cny 按发车人数重写掉）。
func TestCarpoolQuotedPrepaidMigration(t *testing.T) {
	raw, err := FS.ReadFile("190_carpool_quoted_prepaid.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, fragment := range []string{
		"ALTER TABLE carpool_members",
		"ADD COLUMN IF NOT EXISTS quoted_prepaid_amount_cny DECIMAL(20, 2) NULL",
		"SET quoted_prepaid_amount_cny = prepaid_amount_cny",
		"WHERE quoted_prepaid_amount_cny IS NULL",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}

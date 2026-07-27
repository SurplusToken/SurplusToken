package migrations

import (
	"strings"
	"testing"
)

// 192 把升级前建立的车全部划入 custom（人工结算），并从各车自己的老字段
// 生成规则说明。缺了它，老车会被按额度预约制出账——成员申报恒为 0，
// 每人凭空多出几百块"补款"。
func TestCarpoolPricingModelMigration(t *testing.T) {
	raw, err := FS.ReadFile("192_carpool_pricing_model.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, fragment := range []string{
		"ADD COLUMN IF NOT EXISTS pricing_model VARCHAR(16)",
		"ADD COLUMN IF NOT EXISTS rule_note TEXT",
		// 存量行全部归入 custom
		"UPDATE carpools SET pricing_model = 'custom' WHERE pricing_model IS NULL",
		// 规则说明来自各车自己的老字段，而不是一句写死的话
		"capacity::text",
		"base_fee_cny",
		"usage_pool_cny_per_account",
		// 之后新建的车默认走额度预约制
		"ALTER COLUMN pricing_model SET DEFAULT 'quota'",
		"ALTER COLUMN pricing_model SET NOT NULL",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}

	// 回填必须只作用于尚未写过说明的行，重复执行不得覆盖人工填写的规则。
	if !strings.Contains(sql, "AND rule_note IS NULL") {
		t.Fatal("rule_note backfill must be guarded by IS NULL so it stays idempotent")
	}
}

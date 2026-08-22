package migrations

import (
	"strings"
	"testing"
)

// 227 把废弃的席位制 car_type VARCHAR('small'/'large') 重建为整数车型列，
// 并按 0/1/2 规则回填存量车；同时给 carpool_members 加 acknowledged_risk。
// 回填判错车型的代价是真金白银：quota 老车被刷成 3 会按新计价（50/1200）解读
// 既有账本，custom 车被刷成 quota 会重新被自动结算那套数学沾上。
func TestCarpoolCarTypeMigration(t *testing.T) {
	raw, err := FS.ReadFile("227_carpool_car_type.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, fragment := range []string{
		// 旧 VARCHAR 列仅在仍非 integer 时删除（重复执行不会误删新列）
		"ALTER TABLE carpools DROP COLUMN car_type",
		"data_type <> 'integer'",
		"ADD COLUMN IF NOT EXISTS car_type INTEGER",
		// 回填 0：custom 自定义规则车
		"WHEN COALESCE(pricing_model, 'quota') = 'custom' THEN 0",
		// 回填 2：从未发车的存量 quota 车（创建时就是现行 quota 规则）
		"WHEN launched_at IS NULL THEN 2",
		// 回填 2：有保底机制的 quota 车（成员订阅带 weekly_reserved_usd）
		"s.weekly_reserved_usd IS NOT NULL",
		// 回填 1：发过车却无保底机制的老车
		"ELSE 1",
		// 之后新建的车默认 type 3
		"ALTER COLUMN car_type SET DEFAULT 3",
		"ALTER COLUMN car_type SET NOT NULL",
		// 上车风险确认列（参照 188 joined_wechat_group 的做法）
		"ADD COLUMN IF NOT EXISTS acknowledged_risk BOOLEAN NOT NULL DEFAULT false",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}

	// 回填必须只作用于尚未判型的行，重复执行不得覆盖已写入的车型（含新车 3）。
	if !strings.Contains(sql, "WHERE car_type IS NULL") {
		t.Fatal("car_type backfill must be guarded by IS NULL so it stays idempotent")
	}

	// 判型顺序：custom 判定必须最先（否则 pricing_model='custom' 且从未发车的车
	// 会被 launched_at IS NULL 分支抢去刷成 2）。
	customIdx := strings.Index(sql, "= 'custom' THEN 0")
	neverLaunchedIdx := strings.Index(sql, "WHEN launched_at IS NULL THEN 2")
	reservedIdx := strings.Index(sql, "s.weekly_reserved_usd IS NOT NULL")
	elseIdx := strings.Index(sql, "ELSE 1")
	if !(customIdx >= 0 && customIdx < neverLaunchedIdx && neverLaunchedIdx < reservedIdx && reservedIdx < elseIdx) {
		t.Fatal("car_type backfill branches must be ordered: custom → never-launched → has-reserve → else")
	}
}

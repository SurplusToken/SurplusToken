package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 一辆车的周期必须由组内优先级最高的账号说了算，绝不能再按 reset_at 最晚的账号取。
//
// 事故经过：gogogo 的主账号额度被限，运营加了一个副账号补额度。副账号的
// reset_at 比主账号晚两天，于是"取 reset_at 最晚者"当场把全车周期改跟副账号走，
// 11 名成员的周用量被清零——主账号窗口内已经用掉的 773.74 USD 就此从计数里
// 消失，等于凭空多发了一轮额度。副账号是来补额度的，不该决定周期。
//
// 这条断言直接读源码，因为选取逻辑在 SQL 里，单测没有库可跑。
func TestGroupUpstreamWindowAnchorsOnGroupPriority(t *testing.T) {
	src, err := os.ReadFile("carpool_upstream_window_repo.go")
	require.NoError(t, err)
	code := string(src)

	i := strings.Index(code, "func (r *carpoolUpstreamWindowRepository) GroupUpstreamWeeklyWindow")
	require.GreaterOrEqual(t, i, 0, "找不到 GroupUpstreamWeeklyWindow")
	j := strings.Index(code[i:], "\n}\n")
	require.Greater(t, j, 0)
	body := code[i : i+j]

	require.Contains(t, body, "ORDER BY ag.priority ASC, a.priority ASC, a.id ASC",
		"窗口锚定账号必须按组内优先级取（与调度器优先派发的账号一致）")
	require.NotContains(t, body, "ORDER BY (a.extra->>'codex_7d_reset_at')::timestamptz DESC",
		"按 reset_at 最晚取会让补充额度的副账号劫持全车周期并清零用量")
}

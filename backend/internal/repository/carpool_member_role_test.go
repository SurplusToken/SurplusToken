package repository

import (
	"strings"
	"testing"
)

// 下车后成员行是 status='left'，但 member_role 子查询若不排除它，前端会一直
// 认为用户还在车上：「已上车」不消失、「上车」按钮不出现、车也退不出「我的拼车」。
// 而后端其实允许重新上车（Join 只拒绝 joined/active），所以是纯粹的展示错误
// 把人锁在了外面。
func TestCarpoolSelectExcludesLeftMembersFromMemberRole(t *testing.T) {
	idx := strings.Index(carpoolSelectSQL, "current_member.role")
	if idx < 0 {
		t.Fatal("member_role 子查询不见了")
	}
	// 截取该子查询片段（到下一个右括号为止）
	rest := carpoolSelectSQL[idx:]
	end := strings.Index(rest, "LIMIT 1")
	if end < 0 {
		t.Fatal("member_role 子查询缺少 LIMIT 1")
	}
	sub := rest[:end]

	if !strings.Contains(sub, "current_member.status <> 'left'") {
		t.Fatalf("member_role 必须排除已下车的成员，实际片段:\n%s", sub)
	}
}

// 取消整车会把成员置成 'cancelled'。那个状态要保留在 member_role 里，
// 否则已取消的车会从成员的历史里彻底消失（状态筛选选「已取消」也看不到）。
func TestCarpoolSelectKeepsCancelledMembersInMemberRole(t *testing.T) {
	idx := strings.Index(carpoolSelectSQL, "current_member.role")
	rest := carpoolSelectSQL[idx:]
	end := strings.Index(rest, "LIMIT 1")
	sub := rest[:end]

	if strings.Contains(sub, "cancelled") {
		t.Fatalf("不该把 cancelled 一并排除，实际片段:\n%s", sub)
	}
	// 也不该收紧成白名单，那样会连 cancelled 一起挡掉
	if strings.Contains(sub, "IN ('joined'") || strings.Contains(sub, "IN ('active'") {
		t.Fatalf("不该用 joined/active 白名单，会把 cancelled 挡掉，实际片段:\n%s", sub)
	}
}

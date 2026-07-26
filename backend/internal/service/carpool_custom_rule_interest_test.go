package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 自定义规则咨询：只发给拼车运营联系人（CarpoolAdminEmail），不群发 users 表里的
// role=admin。邮件失败（SMTP 未配置）不影响接口成功，正文含发起人用户 ID 与邮箱。
func TestNotifyCustomRuleInterestEmailsCarpoolAdminWithDegradation(t *testing.T) {
	sender := &recordingSender{fail: true}
	dir := &stubUserDirectory{
		users:  map[int64]*User{12: {ID: 12, Email: "fan@example.com"}},
		admins: []User{{ID: 1, Email: "a1@example.com"}, {ID: 2, Email: "a2@example.com"}},
	}
	svc := newLaunchFlowService(&launchFlowRepoStub{}, sender, dir)

	svc.NotifyCustomRuleInterest(context.Background(), 12, "想要更大的额度池")

	require.Equal(t, []string{CarpoolAdminEmail}, sender.to,
		"拼车咨询只发运营联系人，平台其他 admin 不该收到")
	require.Len(t, sender.subject, 1)
	require.Contains(t, sender.subject[0], "自定义拼车规则")
	require.Len(t, sender.body, 1)
	require.Contains(t, sender.body[0], "#12", "正文应含发起人用户 ID")
	require.Contains(t, sender.body[0], "fan@example.com", "正文应含发起人邮箱")
	require.Contains(t, sender.body[0], "想要更大的额度池", "正文应含用户备注")
}

// 平台里一个 admin 都没有也照发——收件人与 users 表的 admin 角色无关。
func TestNotifyCustomRuleInterestWithoutPlatformAdmins(t *testing.T) {
	sender := &recordingSender{}
	dir := &stubUserDirectory{users: map[int64]*User{12: {ID: 12, Email: "fan@example.com"}}}
	svc := newLaunchFlowService(&launchFlowRepoStub{}, sender, dir)

	svc.NotifyCustomRuleInterest(context.Background(), 12, "")

	require.Equal(t, []string{CarpoolAdminEmail}, sender.to)
}

// 邮件链路未注入（如测试构造）时只记日志降级，不 panic。
func TestNotifyCustomRuleInterestWithoutEmailPipeline(t *testing.T) {
	svc := newLaunchFlowService(&launchFlowRepoStub{}, nil, nil)
	require.NotPanics(t, func() {
		svc.NotifyCustomRuleInterest(context.Background(), 12, "note")
	})
}

// 发起人资料查询失败（甚至 userDirectory 未注入）时仍按用户 ID 发信，不阻塞通知。
func TestNotifyCustomRuleInterestWithUnknownInitiator(t *testing.T) {
	sender := &recordingSender{}
	dir := &stubUserDirectory{users: map[int64]*User{}}
	svc := newLaunchFlowService(&launchFlowRepoStub{}, sender, dir)

	svc.NotifyCustomRuleInterest(context.Background(), 99, "")

	require.Equal(t, []string{CarpoolAdminEmail}, sender.to)
	require.Len(t, sender.body, 1)
	require.Contains(t, sender.body[0], "#99")
}

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 自定义规则咨询：给全部 admin 发提示邮件；邮件失败（SMTP 未配置）不影响接口成功，
// 且每个 admin 都记录了发送尝试，正文含发起人用户 ID 与邮箱。
func TestNotifyCustomRuleInterestEmailsAdminsWithDegradation(t *testing.T) {
	sender := &recordingSender{fail: true}
	dir := &stubUserDirectory{
		users:  map[int64]*User{12: {ID: 12, Email: "fan@example.com"}},
		admins: []User{{ID: 1, Email: "a1@example.com"}, {ID: 2, Email: "a2@example.com"}},
	}
	svc := newLaunchFlowService(&launchFlowRepoStub{}, sender, dir)

	svc.NotifyCustomRuleInterest(context.Background(), 12, "想要更大的额度池")

	require.ElementsMatch(t, []string{"a1@example.com", "a2@example.com"}, sender.to)
	require.Len(t, sender.subject, 2)
	require.Contains(t, sender.subject[0], "自定义拼车规则")
	require.Len(t, sender.body, 2)
	for _, body := range sender.body {
		require.Contains(t, body, "#12", "正文应含发起人用户 ID")
		require.Contains(t, body, "fan@example.com", "正文应含发起人邮箱")
		require.Contains(t, body, "想要更大的额度池", "正文应含用户备注")
	}
}

// 无 admin 时不发送也不报错。
func TestNotifyCustomRuleInterestWithoutAdmins(t *testing.T) {
	sender := &recordingSender{}
	dir := &stubUserDirectory{users: map[int64]*User{12: {ID: 12, Email: "fan@example.com"}}}
	svc := newLaunchFlowService(&launchFlowRepoStub{}, sender, dir)

	require.NotPanics(t, func() {
		svc.NotifyCustomRuleInterest(context.Background(), 12, "")
	})
	require.Empty(t, sender.to)
}

// 邮件链路未注入（如测试构造）时只记日志降级，不 panic。
func TestNotifyCustomRuleInterestWithoutEmailPipeline(t *testing.T) {
	svc := newLaunchFlowService(&launchFlowRepoStub{}, nil, nil)
	require.NotPanics(t, func() {
		svc.NotifyCustomRuleInterest(context.Background(), 12, "note")
	})
}

// 发起人资料查询失败时仍按用户 ID 发信，不阻塞通知。
func TestNotifyCustomRuleInterestWithUnknownInitiator(t *testing.T) {
	sender := &recordingSender{}
	dir := &stubUserDirectory{
		users:  map[int64]*User{},
		admins: []User{{ID: 1, Email: "a1@example.com"}},
	}
	svc := newLaunchFlowService(&launchFlowRepoStub{}, sender, dir)

	svc.NotifyCustomRuleInterest(context.Background(), 99, "")

	require.Equal(t, []string{"a1@example.com"}, sender.to)
	require.Len(t, sender.body, 1)
	require.Contains(t, sender.body[0], "#99")
}

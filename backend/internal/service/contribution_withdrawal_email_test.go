package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type contributionWithdrawalRecordingSender struct {
	fail     bool
	to       []string
	subjects []string
	bodies   []string
}

func (s *contributionWithdrawalRecordingSender) SendEmail(_ context.Context, to, subject, body string) error {
	s.to = append(s.to, to)
	s.subjects = append(s.subjects, subject)
	s.bodies = append(s.bodies, body)
	if s.fail {
		return errors.New("smtp unavailable")
	}
	return nil
}

func TestNotifyContributionWithdrawalCreatedUsesFixedOperatorEmail(t *testing.T) {
	t.Parallel()
	sender := &contributionWithdrawalRecordingSender{}
	svc := &ContributionService{emailSender: sender}
	withdrawal := &ContributionWithdrawal{
		ID:             41,
		UserID:         9,
		UserEmail:      "applicant@example.com",
		Username:       `<img src=x onerror=alert(1)>`,
		Amount:         12.3456789,
		PaymentMethod:  "alipay",
		PaymentAccount: "payee@example.com",
		PayeeName:      "Example User",
		RequestNote:    "please process",
	}

	svc.NotifyWithdrawalCreated(context.Background(), withdrawal)

	require.Equal(t, []string{ContributionWithdrawalAdminEmail}, sender.to)
	require.Equal(t, []string{"有新的贡献提现申请，请及时处理"}, sender.subjects)
	require.Len(t, sender.bodies, 1)
	require.Contains(t, sender.bodies[0], "#41")
	require.Contains(t, sender.bodies[0], "applicant@example.com")
	require.Contains(t, sender.bodies[0], "12.3456789")
	require.Contains(t, sender.bodies[0], "payee@example.com")
	require.NotContains(t, sender.bodies[0], "<img src=x")
	require.Contains(t, sender.bodies[0], "&lt;img src=x")
}

func TestNotifyContributionWithdrawalPaidEmailsApplicant(t *testing.T) {
	t.Parallel()
	sender := &contributionWithdrawalRecordingSender{}
	svc := &ContributionService{emailSender: sender}
	withdrawal := &ContributionWithdrawal{
		ID:               42,
		UserID:           10,
		UserEmail:        " applicant@example.com ",
		Amount:           8.5,
		Status:           ContributionWithdrawalStatusPaid,
		PaymentReference: `<trade-123>`,
	}

	svc.NotifyWithdrawalPaid(context.Background(), withdrawal)

	require.Equal(t, []string{"applicant@example.com"}, sender.to)
	require.Equal(t, []string{"您的贡献提现申请已处理完成"}, sender.subjects)
	require.Len(t, sender.bodies, 1)
	require.Contains(t, sender.bodies[0], "#42")
	require.Contains(t, sender.bodies[0], "已经处理完毕")
	require.Contains(t, sender.bodies[0], "8.5")
	require.Contains(t, sender.bodies[0], "&lt;trade-123&gt;")
}

func TestNotifyContributionWithdrawalPaidSkipsNonPaidAndMissingEmail(t *testing.T) {
	t.Parallel()
	sender := &contributionWithdrawalRecordingSender{}
	svc := &ContributionService{emailSender: sender}

	svc.NotifyWithdrawalPaid(context.Background(), &ContributionWithdrawal{
		ID: 1, UserEmail: "applicant@example.com", Status: ContributionWithdrawalStatusRejected,
	})
	svc.NotifyWithdrawalPaid(context.Background(), &ContributionWithdrawal{
		ID: 2, Status: ContributionWithdrawalStatusPaid,
	})

	require.Empty(t, sender.to)
}

func TestContributionWithdrawalEmailFailureDoesNotPanic(t *testing.T) {
	t.Parallel()
	sender := &contributionWithdrawalRecordingSender{fail: true}
	svc := &ContributionService{emailSender: sender}

	require.NotPanics(t, func() {
		svc.NotifyWithdrawalCreated(context.Background(), &ContributionWithdrawal{ID: 41, UserID: 9})
		svc.NotifyWithdrawalPaid(context.Background(), &ContributionWithdrawal{
			ID: 42, UserID: 9, UserEmail: "applicant@example.com", Status: ContributionWithdrawalStatusPaid,
		})
	})
	require.Len(t, sender.to, 2)
}

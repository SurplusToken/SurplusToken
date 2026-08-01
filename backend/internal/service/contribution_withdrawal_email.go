package service

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
)

// ContributionWithdrawalAdminEmail is the fixed operator mailbox for new payout requests.
const ContributionWithdrawalAdminEmail = "studyz@mail.surplustoken.com"

// ContributionWithdrawalEmailSender is the SMTP delivery surface used by withdrawal notifications.
type ContributionWithdrawalEmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// NotifyWithdrawalCreated notifies the fixed payout operator after a new
// withdrawal has been committed. Delivery failure must not roll back or hide
// the successfully created withdrawal.
func (s *ContributionService) NotifyWithdrawalCreated(ctx context.Context, withdrawal *ContributionWithdrawal) {
	if s == nil || s.emailSender == nil || withdrawal == nil {
		return
	}

	subject := "有新的贡献提现申请，请及时处理"
	body := buildContributionWithdrawalCreatedEmailBody(withdrawal)
	if err := s.emailSender.SendEmail(ctx, ContributionWithdrawalAdminEmail, subject, body); err != nil {
		slog.Warn("contribution withdrawal created email failed",
			"withdrawal_id", withdrawal.ID,
			"user_id", withdrawal.UserID,
			"error", err,
		)
	}
}

// NotifyWithdrawalPaid informs the applicant only after the admin review has
// successfully transitioned the withdrawal from pending to paid.
func (s *ContributionService) NotifyWithdrawalPaid(ctx context.Context, withdrawal *ContributionWithdrawal) {
	if s == nil || s.emailSender == nil || withdrawal == nil || withdrawal.Status != ContributionWithdrawalStatusPaid {
		return
	}
	recipient := strings.TrimSpace(withdrawal.UserEmail)
	if recipient == "" {
		slog.Warn("contribution withdrawal paid email skipped: applicant email is empty",
			"withdrawal_id", withdrawal.ID,
			"user_id", withdrawal.UserID,
		)
		return
	}

	subject := "您的贡献提现申请已处理完成"
	body := buildContributionWithdrawalPaidEmailBody(withdrawal)
	if err := s.emailSender.SendEmail(ctx, recipient, subject, body); err != nil {
		slog.Warn("contribution withdrawal paid email failed",
			"withdrawal_id", withdrawal.ID,
			"user_id", withdrawal.UserID,
			"error", err,
		)
	}
}

func buildContributionWithdrawalCreatedEmailBody(withdrawal *ContributionWithdrawal) string {
	username := strings.TrimSpace(withdrawal.Username)
	if username == "" {
		username = "未设置"
	}
	requestNote := strings.TrimSpace(withdrawal.RequestNote)
	if requestNote == "" {
		requestNote = "无"
	}

	return fmt.Sprintf(
		`<p>有新的贡献提现申请，请及时处理。</p>`+
			`<ul>`+
			`<li>申请编号：#%d</li>`+
			`<li>用户 ID：%d</li>`+
			`<li>用户名：%s</li>`+
			`<li>用户邮箱：%s</li>`+
			`<li>提现金额：%s</li>`+
			`<li>收款方式：%s</li>`+
			`<li>收款账户：%s</li>`+
			`<li>收款人姓名：%s</li>`+
			`<li>申请备注：%s</li>`+
			`</ul>`+
			`<p>请登录管理后台的“贡献提现”页面完成审核和打款。</p>`,
		withdrawal.ID,
		withdrawal.UserID,
		html.EscapeString(username),
		html.EscapeString(strings.TrimSpace(withdrawal.UserEmail)),
		formatContributionWithdrawalAmount(withdrawal.Amount),
		html.EscapeString(strings.TrimSpace(withdrawal.PaymentMethod)),
		html.EscapeString(strings.TrimSpace(withdrawal.PaymentAccount)),
		html.EscapeString(strings.TrimSpace(withdrawal.PayeeName)),
		html.EscapeString(requestNote),
	)
}

func buildContributionWithdrawalPaidEmailBody(withdrawal *ContributionWithdrawal) string {
	paymentReference := strings.TrimSpace(withdrawal.PaymentReference)
	if paymentReference == "" {
		paymentReference = "未提供"
	}
	return fmt.Sprintf(
		`<p>您的贡献提现申请 #%d 已经处理完毕，款项已打出，请注意查收。</p>`+
			`<ul>`+
			`<li>提现金额：%s</li>`+
			`<li>支付流水号：%s</li>`+
			`</ul>`,
		withdrawal.ID,
		formatContributionWithdrawalAmount(withdrawal.Amount),
		html.EscapeString(paymentReference),
	)
}

func formatContributionWithdrawalAmount(amount float64) string {
	formatted := fmt.Sprintf("%.8f", amount)
	formatted = strings.TrimRight(formatted, "0")
	return strings.TrimRight(formatted, ".")
}

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type contributionQueryExecer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type contributionRepository struct {
	client *dbent.Client
}

func NewContributionRepository(client *dbent.Client, _ *sql.DB) service.ContributionRepository {
	return &contributionRepository{client: client}
}

func (r *contributionRepository) EnsureUserContribution(ctx context.Context, userID int64) (*service.ContributionSummary, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	if r == nil || r.client == nil {
		return nil, errors.New("contribution repository client is nil")
	}
	client := clientFromContext(ctx, r.client)
	return ensureUserContributionWithClient(ctx, client, userID)
}

func (r *contributionRepository) ThawFrozenQuota(ctx context.Context, userID int64) (float64, error) {
	if userID <= 0 {
		return 0, service.ErrUserNotFound
	}
	if r == nil || r.client == nil {
		return 0, errors.New("contribution repository client is nil")
	}
	var thawed float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var err error
		thawed, err = thawContributionFrozenQuotaTx(txCtx, txClient, userID)
		return err
	})
	return thawed, err
}

func (r *contributionRepository) TransferQuotaToBalance(ctx context.Context, userID int64) (*service.ContributionTransferResponse, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	if r == nil || r.client == nil {
		return nil, errors.New("contribution repository client is nil")
	}
	var out service.ContributionTransferResponse

	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserContributionWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		if _, err := thawContributionFrozenQuotaTx(txCtx, txClient, userID); err != nil {
			return fmt.Errorf("thaw contribution before transfer: %w", err)
		}

		rows, err := txClient.QueryContext(txCtx, `
WITH claimed AS (
	SELECT contribution_quota::double precision AS amount
	FROM user_contributions
	WHERE user_id = $1
	  AND contribution_quota > 0
	FOR UPDATE
),
cleared AS (
	UPDATE user_contributions uc
	SET contribution_quota = 0,
	    updated_at = NOW()
	FROM claimed c
	WHERE uc.user_id = $1
	RETURNING c.amount
)
SELECT amount
FROM cleared`, userID)
		if err != nil {
			return fmt.Errorf("claim contribution quota: %w", err)
		}
		if !rows.Next() {
			_ = rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			return service.ErrContributionQuotaEmpty
		}
		if err := rows.Scan(&out.TransferredQuota); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if out.TransferredQuota <= 0 {
			return service.ErrContributionQuotaEmpty
		}

		affected, err := txClient.User.Update().
			Where(user.IDEQ(userID)).
			AddBalance(out.TransferredQuota).
			AddTotalRecharged(out.TransferredQuota).
			Save(txCtx)
		if err != nil {
			return fmt.Errorf("credit user balance by contribution quota: %w", err)
		}
		if affected == 0 {
			return service.ErrUserNotFound
		}

		snapshot, err := queryContributionTransferSnapshot(txCtx, txClient, userID)
		if err != nil {
			return err
		}
		out.Balance = snapshot.BalanceAfter
		out.ContributionQuota = snapshot.AvailableQuotaAfter
		out.ContributionFrozenQuota = snapshot.FrozenQuotaAfter
		out.ContributionHistoryQuota = snapshot.HistoryQuotaAfter

		if _, err = txClient.ExecContext(txCtx, `
INSERT INTO user_contribution_ledger (
    user_id,
    action,
    amount,
    balance_after,
    contribution_quota_after,
    contribution_frozen_quota_after,
    contribution_history_quota_after,
    created_at,
    updated_at
)
VALUES ($1, 'transfer', $2, $3, $4, $5, $6, NOW(), NOW())`,
			userID,
			out.TransferredQuota,
			snapshot.BalanceAfter,
			snapshot.AvailableQuotaAfter,
			snapshot.FrozenQuotaAfter,
			snapshot.HistoryQuotaAfter,
		); err != nil {
			return fmt.Errorf("insert contribution transfer ledger: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &out, nil
}

func (r *contributionRepository) CreateWithdrawal(ctx context.Context, userID int64, req service.CreateContributionWithdrawalRequest) (*service.ContributionWithdrawal, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	var withdrawalID int64
	newlyCreated := false
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserContributionWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		if _, err := thawContributionFrozenQuotaTx(txCtx, txClient, userID); err != nil {
			return fmt.Errorf("thaw contribution before withdrawal: %w", err)
		}

		available, frozen, history, err := lockContributionSummaryTx(txCtx, txClient, userID)
		if err != nil {
			return err
		}
		existingID, existingFingerprint, found, err := queryWithdrawalIdempotencyTx(txCtx, txClient, userID, req.IdempotencyKey)
		if err != nil {
			return err
		}
		if found {
			if existingFingerprint != req.RequestFingerprint {
				return service.ErrContributionWithdrawalIdempotencyKey
			}
			withdrawalID = existingID
			return nil
		}
		monthlyCount, err := monthlyWithdrawalCountTx(txCtx, txClient, userID)
		if err != nil {
			return err
		}
		if monthlyCount >= service.ContributionWithdrawalMonthlyLimit {
			return service.ErrContributionWithdrawalMonthlyLimit
		}
		pending, err := hasPendingWithdrawalTx(txCtx, txClient, userID)
		if err != nil {
			return err
		}
		if pending {
			return service.ErrContributionWithdrawalPendingExists
		}
		if available+1e-9 < req.Amount {
			return service.ErrContributionWithdrawalInsufficient
		}

		rows, err := txClient.QueryContext(txCtx, `
UPDATE user_contributions
SET contribution_quota = contribution_quota - $1,
    updated_at = NOW()
WHERE user_id = $2 AND contribution_quota >= $1
RETURNING contribution_quota::double precision`, req.Amount, userID)
		if err != nil {
			return fmt.Errorf("reserve contribution withdrawal: %w", err)
		}
		var availableAfter float64
		if !rows.Next() {
			_ = rows.Close()
			return service.ErrContributionWithdrawalInsufficient
		}
		if err := rows.Scan(&availableAfter); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}

		rows, err = txClient.QueryContext(txCtx, `
INSERT INTO contribution_withdrawals (
    user_id, amount, status, payment_method, payment_account, payee_name,
    request_note, payment_qr_code, idempotency_key, request_fingerprint, requested_at, created_at, updated_at
)
VALUES ($1, $2, 'pending', $3, $4, $5, $6, $7, $8, $9, NOW(), NOW(), NOW())
RETURNING id`, userID, req.Amount, req.PaymentMethod, req.PaymentAccount, req.PayeeName, req.RequestNote, req.PaymentQRCode, req.IdempotencyKey, req.RequestFingerprint)
		if err != nil {
			return fmt.Errorf("create contribution withdrawal: %w", err)
		}
		if !rows.Next() {
			_ = rows.Close()
			return errors.New("create contribution withdrawal returned no id")
		}
		if err := rows.Scan(&withdrawalID); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		newlyCreated = true

		_, err = txClient.ExecContext(txCtx, `
INSERT INTO user_contribution_ledger (
    user_id, action, amount, withdrawal_id,
    contribution_quota_after, contribution_frozen_quota_after,
    contribution_history_quota_after, created_at, updated_at
)
VALUES ($1, 'withdraw_hold', $2, $3, $4, $5, $6, NOW(), NOW())`,
			userID, req.Amount, withdrawalID, availableAfter, frozen, history)
		if err != nil {
			return fmt.Errorf("insert contribution withdrawal hold ledger: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result, err := r.getWithdrawalByID(ctx, withdrawalID)
	if result != nil {
		result.NewlyCreated = newlyCreated
	}
	return result, err
}

func (r *contributionRepository) ListWithdrawals(ctx context.Context, userID int64, page, pageSize int) ([]service.ContributionWithdrawal, int64, error) {
	return r.listWithdrawals(ctx, userID, "", "", page, pageSize)
}

func (r *contributionRepository) ListWithdrawalsAdmin(ctx context.Context, status, search string, page, pageSize int) ([]service.ContributionWithdrawal, int64, error) {
	return r.listWithdrawals(ctx, 0, status, search, page, pageSize)
}

func (r *contributionRepository) CancelWithdrawal(ctx context.Context, userID, withdrawalID int64) (*service.ContributionWithdrawal, error) {
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		amount, status, ownerID, err := lockWithdrawalTx(txCtx, txClient, withdrawalID)
		if err != nil {
			return err
		}
		if ownerID != userID {
			return service.ErrContributionWithdrawalNotFound
		}
		if status != service.ContributionWithdrawalStatusPending {
			return service.ErrContributionWithdrawalInvalidState
		}
		if err := refundContributionWithdrawalTx(txCtx, txClient, withdrawalID, userID, amount, 0, service.ContributionWithdrawalStatusCancelled, "", ""); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.getWithdrawalByID(ctx, withdrawalID)
}

func (r *contributionRepository) ReviewWithdrawal(ctx context.Context, req service.ReviewContributionWithdrawalRequest) (*service.ContributionWithdrawal, error) {
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		amount, status, ownerID, err := lockWithdrawalTx(txCtx, txClient, req.WithdrawalID)
		if err != nil {
			return err
		}
		if status != service.ContributionWithdrawalStatusPending {
			return service.ErrContributionWithdrawalInvalidState
		}
		if req.Status == service.ContributionWithdrawalStatusRejected {
			return refundContributionWithdrawalTx(txCtx, txClient, req.WithdrawalID, ownerID, amount, req.AdminUserID, req.Status, req.ReviewNote, "")
		}

		result, err := txClient.ExecContext(txCtx, `
UPDATE contribution_withdrawals
SET status = 'paid', review_note = $1, payment_reference = $2,
    payment_qr_code = '', reviewed_by = $3, reviewed_at = NOW(), paid_at = NOW(), updated_at = NOW()
WHERE id = $4 AND status = 'pending'`, req.ReviewNote, req.PaymentReference, req.AdminUserID, req.WithdrawalID)
		if err != nil {
			return fmt.Errorf("mark contribution withdrawal paid: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read paid contribution withdrawal result: %w", err)
		}
		if affected != 1 {
			return service.ErrContributionWithdrawalInvalidState
		}
		available, frozen, history, err := lockContributionSummaryTx(txCtx, txClient, ownerID)
		if err != nil {
			return err
		}
		if _, err := txClient.ExecContext(txCtx, `
INSERT INTO user_contribution_ledger (
    user_id, action, amount, withdrawal_id,
    contribution_quota_after, contribution_frozen_quota_after,
    contribution_history_quota_after, created_at, updated_at
)
VALUES ($1, 'withdraw_complete', $2, $3, $4, $5, $6, NOW(), NOW())`,
			ownerID, amount, req.WithdrawalID, available, frozen, history); err != nil {
			return fmt.Errorf("insert contribution withdrawal completion ledger: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.getWithdrawalByID(ctx, req.WithdrawalID)
}

func (r *contributionRepository) GetWithdrawalQRCodeData(ctx context.Context, requesterUserID, withdrawalID int64, admin bool) (string, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT payment_qr_code
FROM contribution_withdrawals
WHERE id = $1 AND ($2 OR user_id = $3)
LIMIT 1`, withdrawalID, admin, requesterUserID)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return "", service.ErrContributionWithdrawalNotFound
	}
	var dataURL string
	if err := rows.Scan(&dataURL); err != nil {
		return "", err
	}
	if strings.TrimSpace(dataURL) == "" {
		return "", service.ErrContributionWithdrawalNotFound
	}
	return dataURL, rows.Err()
}

func (r *contributionRepository) getWithdrawalByID(ctx context.Context, withdrawalID int64) (*service.ContributionWithdrawal, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, contributionWithdrawalSelect+`
WHERE w.id = $1
LIMIT 1`, withdrawalID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, service.ErrContributionWithdrawalNotFound
	}
	out, err := scanContributionWithdrawal(rows)
	if err != nil {
		return nil, err
	}
	return out, rows.Err()
}

func (r *contributionRepository) listWithdrawals(ctx context.Context, userID int64, status, search string, page, pageSize int) ([]service.ContributionWithdrawal, int64, error) {
	client := clientFromContext(ctx, r.client)
	userFilter := userID > 0
	where := `WHERE ($1 = '' OR w.status = $1)
  AND ($2 = '' OR LOWER(COALESCE(u.email, '')) LIKE LOWER('%' || $2 || '%')
       OR LOWER(COALESCE(u.username, '')) LIKE LOWER('%' || $2 || '%')
       OR CAST(w.id AS TEXT) = $2)`
	args := []any{status, search}
	if userFilter {
		where = `WHERE w.user_id = $1`
		args = []any{userID}
	}

	countRows, err := client.QueryContext(ctx, `SELECT COUNT(*) FROM contribution_withdrawals w JOIN users u ON u.id = w.user_id `+where, args...)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if countRows.Next() {
		err = countRows.Scan(&total)
	}
	closeErr := countRows.Close()
	if err != nil {
		return nil, 0, err
	}
	if closeErr != nil {
		return nil, 0, closeErr
	}

	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := client.QueryContext(ctx, contributionWithdrawalSelect+where+
		fmt.Sprintf(" ORDER BY w.requested_at DESC, w.id DESC LIMIT $%d OFFSET $%d", limitPos, offsetPos), queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.ContributionWithdrawal, 0, pageSize)
	for rows.Next() {
		item, err := scanContributionWithdrawal(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

const contributionWithdrawalSelect = `
SELECT w.id, w.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
       w.amount::double precision, w.status, w.payment_method,
       w.payment_account, w.payee_name, w.request_note, w.review_note,
       w.payment_reference, (w.payment_qr_code <> ''), w.reviewed_by, w.requested_at, w.reviewed_at,
       w.paid_at, w.cancelled_at, w.created_at, w.updated_at
FROM contribution_withdrawals w
JOIN users u ON u.id = w.user_id
`

type contributionWithdrawalScanner interface {
	Scan(dest ...any) error
}

func scanContributionWithdrawal(scanner contributionWithdrawalScanner) (*service.ContributionWithdrawal, error) {
	var out service.ContributionWithdrawal
	var reviewedBy sql.NullInt64
	var reviewedAt, paidAt, cancelledAt sql.NullTime
	if err := scanner.Scan(
		&out.ID, &out.UserID, &out.UserEmail, &out.Username,
		&out.Amount, &out.Status, &out.PaymentMethod, &out.PaymentAccount,
		&out.PayeeName, &out.RequestNote, &out.ReviewNote, &out.PaymentReference, &out.HasPaymentQRCode,
		&reviewedBy, &out.RequestedAt, &reviewedAt, &paidAt, &cancelledAt,
		&out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if reviewedBy.Valid {
		out.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		out.ReviewedAt = &reviewedAt.Time
	}
	if paidAt.Valid {
		out.PaidAt = &paidAt.Time
	}
	if cancelledAt.Valid {
		out.CancelledAt = &cancelledAt.Time
	}
	return &out, nil
}

func lockContributionSummaryTx(ctx context.Context, client *dbent.Client, userID int64) (available, frozen, history float64, err error) {
	rows, err := client.QueryContext(ctx, `
SELECT contribution_quota::double precision,
       contribution_frozen_quota::double precision,
       contribution_history_quota::double precision
FROM user_contributions
WHERE user_id = $1
FOR UPDATE`, userID)
	if err != nil {
		return 0, 0, 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, 0, 0, service.ErrUserNotFound
	}
	if err := rows.Scan(&available, &frozen, &history); err != nil {
		return 0, 0, 0, err
	}
	return available, frozen, history, rows.Err()
}

func queryWithdrawalIdempotencyTx(ctx context.Context, client *dbent.Client, userID int64, key string) (id int64, fingerprint string, found bool, err error) {
	rows, err := client.QueryContext(ctx, `
SELECT id, request_fingerprint
FROM contribution_withdrawals
WHERE user_id = $1 AND idempotency_key = $2
LIMIT 1`, userID, key)
	if err != nil {
		return 0, "", false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, "", false, rows.Err()
	}
	if err := rows.Scan(&id, &fingerprint); err != nil {
		return 0, "", false, err
	}
	return id, fingerprint, true, rows.Err()
}

func hasPendingWithdrawalTx(ctx context.Context, client *dbent.Client, userID int64) (bool, error) {
	rows, err := client.QueryContext(ctx, `SELECT 1 FROM contribution_withdrawals WHERE user_id = $1 AND status = 'pending' LIMIT 1`, userID)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	return rows.Next(), rows.Err()
}

func monthlyWithdrawalCountTx(ctx context.Context, client *dbent.Client, userID int64) (int, error) {
	rows, err := client.QueryContext(ctx, `
SELECT COUNT(*)
FROM contribution_withdrawals
WHERE user_id = $1
  AND requested_at >= date_trunc('month', NOW())
  AND requested_at < date_trunc('month', NOW()) + INTERVAL '1 month'`, userID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	return count, rows.Err()
}

func lockWithdrawalTx(ctx context.Context, client *dbent.Client, withdrawalID int64) (amount float64, status string, userID int64, err error) {
	rows, err := client.QueryContext(ctx, `
SELECT amount::double precision, status, user_id
FROM contribution_withdrawals
WHERE id = $1
FOR UPDATE`, withdrawalID)
	if err != nil {
		return 0, "", 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, "", 0, service.ErrContributionWithdrawalNotFound
	}
	if err := rows.Scan(&amount, &status, &userID); err != nil {
		return 0, "", 0, err
	}
	return amount, status, userID, rows.Err()
}

func refundContributionWithdrawalTx(ctx context.Context, client *dbent.Client, withdrawalID, userID int64, amount float64, adminID int64, status, reviewNote, paymentReference string) error {
	available, frozen, history, err := lockContributionSummaryTx(ctx, client, userID)
	if err != nil {
		return err
	}
	availableAfter := available + amount
	if _, err := client.ExecContext(ctx, `
UPDATE user_contributions
SET contribution_quota = contribution_quota + $1, updated_at = NOW()
WHERE user_id = $2`, amount, userID); err != nil {
		return fmt.Errorf("refund contribution withdrawal: %w", err)
	}

	var result sql.Result
	if status == service.ContributionWithdrawalStatusCancelled {
		result, err = client.ExecContext(ctx, `
UPDATE contribution_withdrawals
SET status = 'cancelled', payment_qr_code = '', cancelled_at = NOW(), updated_at = NOW()
WHERE id = $1 AND status = 'pending'`, withdrawalID)
	} else {
		result, err = client.ExecContext(ctx, `
UPDATE contribution_withdrawals
SET status = 'rejected', review_note = $1, payment_reference = $2,
    payment_qr_code = '', reviewed_by = $3, reviewed_at = NOW(), updated_at = NOW()
WHERE id = $4 AND status = 'pending'`, reviewNote, paymentReference, adminID, withdrawalID)
	}
	if err != nil {
		return fmt.Errorf("update contribution withdrawal refund status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read contribution withdrawal refund result: %w", err)
	}
	if affected != 1 {
		return service.ErrContributionWithdrawalInvalidState
	}

	if _, err := client.ExecContext(ctx, `
INSERT INTO user_contribution_ledger (
    user_id, action, amount, withdrawal_id,
    contribution_quota_after, contribution_frozen_quota_after,
    contribution_history_quota_after, created_at, updated_at
)
VALUES ($1, 'withdraw_cancel', $2, $3, $4, $5, $6, NOW(), NOW())`,
		userID, amount, withdrawalID, availableAfter, frozen, history); err != nil {
		return fmt.Errorf("insert contribution withdrawal cancellation ledger: %w", err)
	}
	return nil
}

func (r *contributionRepository) withTx(ctx context.Context, fn func(txCtx context.Context, txClient *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin contribution transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit contribution transaction: %w", err)
	}
	return nil
}

func ensureUserContributionWithClient(ctx context.Context, client contributionQueryExecer, userID int64) (*service.ContributionSummary, error) {
	summary, err := queryContributionByUserID(ctx, client, userID)
	if err == nil {
		return summary, nil
	}
	if !errors.Is(err, service.ErrUserNotFound) {
		return nil, err
	}
	if _, err := client.ExecContext(ctx, `
INSERT INTO user_contributions (user_id, created_at, updated_at)
VALUES ($1, NOW(), NOW())
ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return nil, err
	}
	return queryContributionByUserID(ctx, client, userID)
}

func queryContributionByUserID(ctx context.Context, client contributionQueryExecer, userID int64) (*service.ContributionSummary, error) {
	rows, err := client.QueryContext(ctx, `
SELECT user_id,
       contribution_quota::double precision,
       contribution_frozen_quota::double precision,
       contribution_history_quota::double precision,
       COALESCE((
           SELECT SUM(amount)::double precision
           FROM contribution_withdrawals
           WHERE user_id = user_contributions.user_id AND status = 'pending'
       ), 0),
       COALESCE((
           SELECT SUM(amount)::double precision
           FROM contribution_withdrawals
           WHERE user_id = user_contributions.user_id AND status = 'paid'
       ), 0),
       COALESCE((
           SELECT SUM(amount)::double precision
           FROM user_contribution_ledger
           WHERE user_id = user_contributions.user_id AND action = 'transfer'
       ), 0),
	   (
	       SELECT COUNT(*)
	       FROM contribution_withdrawals
	       WHERE user_id = user_contributions.user_id
	         AND requested_at >= date_trunc('month', NOW())
	         AND requested_at < date_trunc('month', NOW()) + INTERVAL '1 month'
	   ),
       created_at,
       updated_at
FROM user_contributions
WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrUserNotFound
	}

	var out service.ContributionSummary
	if err := rows.Scan(
		&out.UserID,
		&out.ContributionQuota,
		&out.ContributionFrozenQuota,
		&out.ContributionHistoryQuota,
		&out.ContributionPendingWithdrawalQuota,
		&out.ContributionWithdrawnQuota,
		&out.ContributionTransferredQuota,
		&out.ContributionMonthlyWithdrawalCount,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &out, rows.Err()
}

func thawContributionFrozenQuotaTx(txCtx context.Context, txClient *dbent.Client, userID int64) (float64, error) {
	rows, err := txClient.QueryContext(txCtx, `
WITH matured AS (
    UPDATE user_contribution_ledger
    SET frozen_until = NULL, updated_at = NOW()
    WHERE user_id = $1
      AND frozen_until IS NOT NULL
      AND frozen_until <= NOW()
    RETURNING amount
)
SELECT COALESCE(SUM(amount), 0) FROM matured`, userID)
	if err != nil {
		return 0, fmt.Errorf("thaw contribution quota: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var thawed float64
	if rows.Next() {
		if err := rows.Scan(&thawed); err != nil {
			return 0, err
		}
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if thawed <= 0 {
		return 0, nil
	}

	_, err = txClient.ExecContext(txCtx, `
UPDATE user_contributions
SET contribution_quota = contribution_quota + $1,
    contribution_frozen_quota = GREATEST(contribution_frozen_quota - $1, 0),
    updated_at = NOW()
WHERE user_id = $2`, thawed, userID)
	if err != nil {
		return 0, fmt.Errorf("move thawed contribution quota: %w", err)
	}
	return thawed, nil
}

type contributionTransferSnapshot struct {
	BalanceAfter        float64
	AvailableQuotaAfter float64
	FrozenQuotaAfter    float64
	HistoryQuotaAfter   float64
}

func queryContributionTransferSnapshot(ctx context.Context, client contributionQueryExecer, userID int64) (*contributionTransferSnapshot, error) {
	rows, err := client.QueryContext(ctx, `
SELECT u.balance::double precision,
       uc.contribution_quota::double precision,
       uc.contribution_frozen_quota::double precision,
       uc.contribution_history_quota::double precision
FROM users u
JOIN user_contributions uc ON uc.user_id = u.id
WHERE u.id = $1
LIMIT 1`, userID)
	if err != nil {
		return nil, fmt.Errorf("query contribution transfer snapshot: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrUserNotFound
	}

	var snapshot contributionTransferSnapshot
	if err := rows.Scan(
		&snapshot.BalanceAfter,
		&snapshot.AvailableQuotaAfter,
		&snapshot.FrozenQuotaAfter,
		&snapshot.HistoryQuotaAfter,
	); err != nil {
		return nil, err
	}
	return &snapshot, rows.Err()
}

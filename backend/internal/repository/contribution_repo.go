package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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

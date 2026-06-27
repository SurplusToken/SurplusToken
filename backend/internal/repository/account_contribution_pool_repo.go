package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// accountContributionPoolRepository implements service.AccountContributionPoolRepository
// with raw SQL over the account_contribution_pools + user_contributions/ledger tables
// (mirrors the remote_sessions raw-SQL repo style).
type accountContributionPoolRepository struct {
	sql *sql.DB
}

func NewAccountContributionPoolRepository(sqlDB *sql.DB) service.AccountContributionPoolRepository {
	return &accountContributionPoolRepository{sql: sqlDB}
}

func (r *accountContributionPoolRepository) GetPool(ctx context.Context, accountID int64) (float64, error) {
	rows, err := r.sql.QueryContext(ctx, `SELECT COALESCE(pool_amount, 0)::double precision FROM account_contribution_pools WHERE account_id = $1`, accountID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	var amount float64
	if rows.Next() {
		if err := rows.Scan(&amount); err != nil {
			return 0, err
		}
	}
	return amount, rows.Err()
}

func (r *accountContributionPoolRepository) ResolveDisplayNames(ctx context.Context, userIDs []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT id, COALESCE(username, ''), COALESCE(email, '') FROM users WHERE id = ANY($1)`, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var username, email string
		if err := rows.Scan(&id, &username, &email); err != nil {
			return nil, err
		}
		name := username
		if name == "" {
			name = email
		}
		out[id] = name
	}
	return out, rows.Err()
}

func (r *accountContributionPoolRepository) Distribute(ctx context.Context, accountID int64, allocations []service.PoolAllocation) (err error) {
	var total float64
	for _, a := range allocations {
		if a.UserID > 0 && a.Amount > 0 {
			total += a.Amount
		}
	}
	if total <= 0 {
		return service.ErrContributionPoolEmpty
	}

	tx, err := r.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Lock the pool row and verify it covers the distribution.
	var pool float64
	if scanErr := tx.QueryRowContext(ctx, `SELECT COALESCE(pool_amount, 0)::double precision FROM account_contribution_pools WHERE account_id = $1 FOR UPDATE`, accountID).Scan(&pool); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			err = service.ErrContributionPoolEmpty
			return err
		}
		err = scanErr
		return err
	}
	if total > pool+1e-9 {
		err = service.ErrContributionPoolInsufficient
		return err
	}

	if _, err = tx.ExecContext(ctx, `UPDATE account_contribution_pools SET pool_amount = pool_amount - $1, updated_at = NOW() WHERE account_id = $2`, total, accountID); err != nil {
		return err
	}

	for _, a := range allocations {
		if a.UserID <= 0 || a.Amount <= 0 {
			continue
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO user_contributions (user_id, created_at, updated_at) VALUES ($1, NOW(), NOW()) ON CONFLICT (user_id) DO NOTHING`, a.UserID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE user_contributions SET contribution_quota = contribution_quota + $1, contribution_history_quota = contribution_history_quota + $1, updated_at = NOW() WHERE user_id = $2`, a.Amount, a.UserID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `
INSERT INTO user_contribution_ledger
  (user_id, action, amount, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, image_count, total_cost, account_rate_multiplier, reward_rate, account_id, created_at, updated_at)
VALUES ($1, 'accrue', $2, 0, 0, 0, 0, 0, 0, 1, 0, $3, NOW(), NOW())`, a.UserID, a.Amount, accountID); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}

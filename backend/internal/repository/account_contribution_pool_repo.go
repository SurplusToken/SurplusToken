package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"

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

func loadContributionPoolPrimaryOwner(ctx context.Context, tx *sql.Tx, accountID int64, forUpdate bool) (int64, error) {
	query := `SELECT owner_user_id FROM accounts WHERE id = $1 AND deleted_at IS NULL`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var owner sql.NullInt64
	if err := tx.QueryRowContext(ctx, query, accountID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, service.ErrAccountNotFound
		}
		return 0, err
	}
	if !owner.Valid || owner.Int64 <= 0 {
		return 0, service.ErrContributionNotOwner
	}
	return owner.Int64, nil
}

func loadContributionPoolCoOwners(ctx context.Context, tx *sql.Tx, accountID int64, forShare bool) ([]int64, error) {
	query := `SELECT user_id FROM account_co_owners WHERE account_id = $1 ORDER BY user_id`
	if forShare {
		query += ` FOR SHARE`
	}
	rows, err := tx.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	coOwners := make([]int64, 0)
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		if userID > 0 {
			coOwners = append(coOwners, userID)
		}
	}
	return coOwners, rows.Err()
}

func contributionPoolOwnerSet(primaryOwner int64, coOwners []int64) ([]int64, map[int64]struct{}) {
	owners := make([]int64, 0, len(coOwners)+1)
	ownerSet := make(map[int64]struct{}, len(coOwners)+1)
	if primaryOwner > 0 {
		owners = append(owners, primaryOwner)
		ownerSet[primaryOwner] = struct{}{}
	}
	for _, userID := range coOwners {
		if userID <= 0 {
			continue
		}
		if _, duplicate := ownerSet[userID]; duplicate {
			continue
		}
		ownerSet[userID] = struct{}{}
		owners = append(owners, userID)
	}
	return owners, ownerSet
}

func sameContributionPoolOwnerSet(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	set := make(map[int64]struct{}, len(left))
	for _, userID := range left {
		set[userID] = struct{}{}
	}
	for _, userID := range right {
		if _, ok := set[userID]; !ok {
			return false
		}
	}
	return true
}

func lockLiveContributionPoolOwners(ctx context.Context, tx *sql.Tx, owners []int64) error {
	rows, err := tx.QueryContext(ctx, `
SELECT id
FROM users
WHERE id = ANY($1) AND deleted_at IS NULL
ORDER BY id
FOR SHARE`, pq.Array(owners))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != len(owners) {
		return service.ErrContributionBadRecipient
	}
	return nil
}

func (r *accountContributionPoolRepository) Distribute(ctx context.Context, req service.PoolDistributionRequest) (result *service.PoolDistributionResult, err error) {
	tx, err := r.sql.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Phase 1 is deliberately lock-free: discover the candidate owner set, then
	// lock those users in stable ID order. User deletion and co-owner replacement
	// use the same user->account order, avoiding account<->user deadlocks.
	initialPrimaryOwner, loadErr := loadContributionPoolPrimaryOwner(ctx, tx, req.AccountID, false)
	if loadErr != nil {
		return nil, loadErr
	}
	if initialPrimaryOwner != req.RequesterUserID {
		return nil, service.ErrContributionNotOwner
	}
	initialCoOwners, loadErr := loadContributionPoolCoOwners(ctx, tx, req.AccountID, false)
	if loadErr != nil {
		return nil, loadErr
	}
	initialOwners, _ := contributionPoolOwnerSet(initialPrimaryOwner, initialCoOwners)
	if lockErr := lockLiveContributionPoolOwners(ctx, tx, initialOwners); lockErr != nil {
		return nil, lockErr
	}

	// Phase 2 locks the account and re-reads the relation. SetAccountCoOwners also
	// needs this account lock, so the verified set remains stable through commit.
	primaryOwner, loadErr := loadContributionPoolPrimaryOwner(ctx, tx, req.AccountID, true)
	if loadErr != nil {
		return nil, loadErr
	}
	if primaryOwner != req.RequesterUserID {
		return nil, service.ErrContributionNotOwner
	}
	currentCoOwners, loadErr := loadContributionPoolCoOwners(ctx, tx, req.AccountID, true)
	if loadErr != nil {
		return nil, loadErr
	}
	owners, ownerSet := contributionPoolOwnerSet(primaryOwner, currentCoOwners)
	if !sameContributionPoolOwnerSet(initialOwners, owners) {
		return nil, service.ErrContributionOwnerSetChanged
	}

	var existingFingerprint string
	var existingTotal float64
	replayErr := tx.QueryRowContext(ctx, `
SELECT request_fingerprint, total_amount::double precision
FROM account_contribution_pool_distributions
WHERE account_id = $1 AND idempotency_key = $2`, req.AccountID, req.IdempotencyKey).Scan(&existingFingerprint, &existingTotal)
	switch {
	case replayErr == nil:
		if existingFingerprint != req.RequestFingerprint {
			return nil, service.ErrIdempotencyKeyConflict
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return nil, commitErr
		}
		return &service.PoolDistributionResult{Replayed: true, TotalAmount: existingTotal}, nil
	case !errors.Is(replayErr, sql.ErrNoRows):
		return nil, replayErr
	}

	// Lock the pool only after the account lock so accrual, distribution, and
	// deletion use one lock order and cannot deadlock each other.
	var pool float64
	if scanErr := tx.QueryRowContext(ctx, `SELECT COALESCE(pool_amount, 0)::double precision FROM account_contribution_pools WHERE account_id = $1 FOR UPDATE`, req.AccountID).Scan(&pool); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return nil, service.ErrContributionPoolEmpty
		}
		return nil, scanErr
	}
	if pool <= 0 || math.IsNaN(pool) || math.IsInf(pool, 0) {
		return nil, service.ErrContributionPoolEmpty
	}

	allocations := req.Allocations
	if req.Mode == "even" {
		allocations = splitContributionPoolEvenly(pool, owners)
		if len(allocations) == 0 {
			return nil, service.ErrContributionPoolEmpty
		}
	} else if req.Mode == "custom" {
		for _, allocation := range allocations {
			if _, eligible := ownerSet[allocation.UserID]; !eligible {
				return nil, service.ErrContributionBadRecipient
			}
		}
	} else {
		return nil, service.ErrContributionPoolDistributionModeInvalid
	}

	var total float64
	for _, allocation := range allocations {
		if allocation.UserID <= 0 || allocation.Amount <= 0 || math.IsNaN(allocation.Amount) || math.IsInf(allocation.Amount, 0) {
			return nil, service.ErrContributionBadAllocation
		}
		total += allocation.Amount
	}
	total = roundPoolAmount(total)
	if total <= 0 || math.IsNaN(total) || math.IsInf(total, 0) {
		return nil, service.ErrContributionPoolEmpty
	}
	if total > pool+1e-9 {
		return nil, service.ErrContributionPoolInsufficient
	}

	var remaining float64
	if debitErr := tx.QueryRowContext(ctx, `
UPDATE account_contribution_pools
SET pool_amount = pool_amount - $1, updated_at = NOW()
WHERE account_id = $2 AND pool_amount >= $1
RETURNING pool_amount::double precision`, total, req.AccountID).Scan(&remaining); debitErr != nil {
		if errors.Is(debitErr, sql.ErrNoRows) {
			return nil, service.ErrContributionPoolInsufficient
		}
		return nil, debitErr
	}
	_ = remaining

	allocationSnapshot, marshalErr := json.Marshal(allocations)
	if marshalErr != nil {
		return nil, marshalErr
	}
	var distributionID int64
	if insertErr := tx.QueryRowContext(ctx, `
INSERT INTO account_contribution_pool_distributions
  (account_id, idempotency_key, request_fingerprint, mode, total_amount, allocations, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
RETURNING id`, req.AccountID, req.IdempotencyKey, req.RequestFingerprint, req.Mode, total, string(allocationSnapshot), req.RequesterUserID).Scan(&distributionID); insertErr != nil {
		return nil, insertErr
	}

	for _, allocation := range allocations {
		if _, err = tx.ExecContext(ctx, `INSERT INTO user_contributions (user_id, created_at, updated_at) VALUES ($1, NOW(), NOW()) ON CONFLICT (user_id) DO NOTHING`, allocation.UserID); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE user_contributions SET contribution_quota = contribution_quota + $1, contribution_history_quota = contribution_history_quota + $1, updated_at = NOW() WHERE user_id = $2`, allocation.Amount, allocation.UserID); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `
INSERT INTO user_contribution_ledger
  (user_id, action, amount, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, image_count, total_cost, account_rate_multiplier, reward_rate, account_id, created_at, updated_at)
VALUES ($1, 'accrue', $2, 0, 0, 0, 0, 0, 0, 1, 0, $3, NOW(), NOW())`, allocation.UserID, allocation.Amount, req.AccountID); err != nil {
			return nil, err
		}
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO account_contribution_pool_ledger
  (account_id, direction, amount, distribution_id, created_at)
VALUES ($1, 'distribute', $2, $3, NOW())`, req.AccountID, total, distributionID); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &service.PoolDistributionResult{TotalAmount: total}, nil
}

func splitContributionPoolEvenly(pool float64, owners []int64) []service.PoolAllocation {
	if pool <= 0 || len(owners) == 0 || math.IsNaN(pool) || math.IsInf(pool, 0) {
		return nil
	}
	// user_contributions has 8 decimal places. Split integer 1e-8 units so
	// rounding can never make the final remainder zero or overdraw the pool.
	unitsFloat := math.Round(pool * 1e8)
	if unitsFloat < 1 || unitsFloat > 9e18 {
		return nil
	}
	totalUnits := int64(unitsFloat)
	ownerCount := int64(len(owners))
	baseUnits := totalUnits / ownerCount
	remainderUnits := totalUnits % ownerCount
	allocations := make([]service.PoolAllocation, 0, len(owners))
	for index, userID := range owners {
		units := baseUnits
		if int64(index) < remainderUnits {
			units++
		}
		if units == 0 {
			continue
		}
		allocations = append(allocations, service.PoolAllocation{
			UserID: userID,
			Amount: float64(units) / 1e8,
		})
	}
	return allocations
}

func roundPoolAmount(value float64) float64 {
	return math.Round(value*1e8) / 1e8
}

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Account-contribution-pool errors (mapped to HTTP by response.ErrorFrom).
var (
	ErrContributionNotOwner         = infraerrors.Forbidden("CONTRIBUTION_NOT_OWNER", "only the account's primary owner can view/distribute its reward pool")
	ErrContributionPoolEmpty        = infraerrors.BadRequest("CONTRIBUTION_POOL_EMPTY", "no reward in the pool to distribute")
	ErrContributionPoolInsufficient = infraerrors.BadRequest("CONTRIBUTION_POOL_INSUFFICIENT", "distribution exceeds the available pool")
	ErrContributionBadRecipient     = infraerrors.BadRequest("CONTRIBUTION_BAD_RECIPIENT", "a recipient is not an owner of this account")
	ErrContributionBadAllocation    = infraerrors.BadRequest("CONTRIBUTION_BAD_ALLOCATION", "allocations must contain unique positive recipients and finite positive amounts")
	ErrContributionOwnerSetChanged  = infraerrors.Conflict("CONTRIBUTION_OWNER_SET_CHANGED", "the account owner set changed during distribution; retry the request")
)

// PoolAllocation is one recipient's slice of a pool distribution.
type PoolAllocation struct {
	UserID int64   `json:"user_id"`
	Amount float64 `json:"amount"`
}

// PoolDistributionRequest is the repository boundary for one idempotent pool
// distribution. The repository revalidates ownership and the current owner set
// under the same transaction that debits the pool.
type PoolDistributionRequest struct {
	AccountID          int64
	RequesterUserID    int64
	IdempotencyKey     string
	RequestFingerprint string
	Mode               string
	Allocations        []PoolAllocation
}

// PoolDistributionResult reports whether this call applied a new distribution
// or replayed a previously committed request with the same idempotency key.
type PoolDistributionResult struct {
	Replayed    bool
	TotalAmount float64
}

// AccountContributionPoolRepository persists per-account held reward pools and applies
// distributions atomically (Model B).
type AccountContributionPoolRepository interface {
	// GetPool returns the held (undistributed) reward for the account (0 if none).
	GetPool(ctx context.Context, accountID int64) (float64, error)
	// ResolveDisplayNames maps user ids to a display name (username, else email, else "").
	ResolveDisplayNames(ctx context.Context, userIDs []int64) (map[int64]string, error)
	// Distribute atomically revalidates the primary owner and owner set, deduplicates
	// the request, debits the pool, and credits recipients (available immediately,
	// no freeze), writing both distribution and pool-ledger records.
	Distribute(ctx context.Context, req PoolDistributionRequest) (*PoolDistributionResult, error)
}

// AccountContributionService implements the account-level "reward pool + 分发" feature:
// the primary owner views the held pool and distributes it to the account's owner set
// (primary owner + admin-assigned co-owners), evenly or with custom per-recipient amounts.
type AccountContributionService struct {
	accountSvc *AccountService
	poolRepo   AccountContributionPoolRepository
}

func NewAccountContributionService(accountSvc *AccountService, poolRepo AccountContributionPoolRepository) *AccountContributionService {
	return &AccountContributionService{accountSvc: accountSvc, poolRepo: poolRepo}
}

// PoolRecipient is one eligible distribution recipient (an owner of the account).
type PoolRecipient struct {
	UserID         int64  `json:"user_id"`
	DisplayName    string `json:"display_name"`
	IsPrimaryOwner bool   `json:"is_primary_owner"`
}

// AccountContributionPoolView is the GET response.
type AccountContributionPoolView struct {
	AccountID  int64           `json:"account_id"`
	PoolAmount float64         `json:"pool_amount"`
	Recipients []PoolRecipient `json:"recipients"`
}

// ownerSetFor returns the primary owner (first) + co-owners for the account, and
// verifies the requester is the primary owner. Only the primary owner manages the pool.
func (s *AccountContributionService) ownerSetFor(ctx context.Context, requesterID, accountID int64) (*Account, []int64, error) {
	account, err := s.accountSvc.GetByID(ctx, accountID)
	if err != nil {
		return nil, nil, err
	}
	if account == nil {
		return nil, nil, ErrAccountNotFound
	}
	if account.OwnerUserID == nil || *account.OwnerUserID != requesterID {
		return nil, nil, ErrContributionNotOwner
	}
	coOwners, err := s.accountSvc.accountRepo.ListCoOwnerUserIDsByAccount(ctx, accountID)
	if err != nil {
		return nil, nil, err
	}
	owners := make([]int64, 0, len(coOwners)+1)
	owners = append(owners, *account.OwnerUserID)
	seen := map[int64]struct{}{*account.OwnerUserID: {}}
	for _, id := range coOwners {
		if id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		owners = append(owners, id)
	}
	return account, owners, nil
}

// GetAccountPool returns the held pool + the eligible recipients (owner-only).
func (s *AccountContributionService) GetAccountPool(ctx context.Context, requesterID, accountID int64) (*AccountContributionPoolView, error) {
	account, owners, err := s.ownerSetFor(ctx, requesterID, accountID)
	if err != nil {
		return nil, err
	}
	pool, err := s.poolRepo.GetPool(ctx, accountID)
	if err != nil {
		return nil, err
	}
	names, err := s.poolRepo.ResolveDisplayNames(ctx, owners)
	if err != nil {
		return nil, err
	}
	primary := *account.OwnerUserID
	recipients := make([]PoolRecipient, 0, len(owners))
	for _, id := range owners {
		recipients = append(recipients, PoolRecipient{
			UserID:         id,
			DisplayName:    names[id],
			IsPrimaryOwner: id == primary,
		})
	}
	return &AccountContributionPoolView{AccountID: accountID, PoolAmount: pool, Recipients: recipients}, nil
}

// DistributeAccountPool distributes held rewards to the current owner set. The
// service validates and canonicalizes the request; all state-dependent checks
// and mutations happen atomically in the repository.
func (s *AccountContributionService) DistributeAccountPool(ctx context.Context, requesterID, accountID int64, idempotencyKey, mode string, allocations []PoolAllocation) error {
	key, err := NormalizeIdempotencyKey(idempotencyKey)
	if err != nil {
		return err
	}
	if key == "" {
		return ErrIdempotencyKeyRequired
	}

	mode = strings.TrimSpace(mode)
	if mode != "even" && mode != "custom" {
		return ErrContributionPoolDistributionModeInvalid
	}

	clean := make([]PoolAllocation, 0, len(allocations))
	if mode == "even" {
		if len(allocations) != 0 {
			return ErrContributionBadAllocation
		}
	} else {
		if len(allocations) == 0 {
			return ErrContributionBadAllocation
		}
		seen := make(map[int64]struct{}, len(allocations))
		for _, allocation := range allocations {
			if allocation.UserID <= 0 || allocation.Amount <= 0 || math.IsNaN(allocation.Amount) || math.IsInf(allocation.Amount, 0) {
				return ErrContributionBadAllocation
			}
			if _, duplicate := seen[allocation.UserID]; duplicate {
				return ErrContributionBadAllocation
			}
			seen[allocation.UserID] = struct{}{}
			amount := roundTo(allocation.Amount, 8)
			if amount <= 0 || math.IsInf(amount, 0) {
				return ErrContributionBadAllocation
			}
			clean = append(clean, PoolAllocation{UserID: allocation.UserID, Amount: amount})
		}
		sort.Slice(clean, func(i, j int) bool { return clean[i].UserID < clean[j].UserID })
	}

	fingerprint := poolDistributionFingerprint(mode, clean)
	_, err = s.poolRepo.Distribute(ctx, PoolDistributionRequest{
		AccountID:          accountID,
		RequesterUserID:    requesterID,
		IdempotencyKey:     key,
		RequestFingerprint: fingerprint,
		Mode:               mode,
		Allocations:        clean,
	})
	return err
}

func poolDistributionFingerprint(mode string, allocations []PoolAllocation) string {
	canonical := mode
	for _, allocation := range allocations {
		canonical += fmt.Sprintf("|%d:%.8f", allocation.UserID, allocation.Amount)
	}
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

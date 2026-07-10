package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Account-contribution-pool errors (mapped to HTTP by response.ErrorFrom).
var (
	ErrContributionNotOwner         = infraerrors.Forbidden("CONTRIBUTION_NOT_OWNER", "only the account's primary owner can view/distribute its reward pool")
	ErrContributionPoolEmpty        = infraerrors.BadRequest("CONTRIBUTION_POOL_EMPTY", "no reward in the pool to distribute")
	ErrContributionPoolInsufficient = infraerrors.BadRequest("CONTRIBUTION_POOL_INSUFFICIENT", "distribution exceeds the available pool")
	ErrContributionBadRecipient     = infraerrors.BadRequest("CONTRIBUTION_BAD_RECIPIENT", "a recipient is not an owner of this account")
)

// PoolAllocation is one recipient's slice of a pool distribution.
type PoolAllocation struct {
	UserID int64
	Amount float64
}

// AccountContributionPoolRepository persists per-account held reward pools and applies
// distributions atomically (Model B).
type AccountContributionPoolRepository interface {
	// GetPool returns the held (undistributed) reward for the account (0 if none).
	GetPool(ctx context.Context, accountID int64) (float64, error)
	// ResolveDisplayNames maps user ids to a display name (username, else email, else "").
	ResolveDisplayNames(ctx context.Context, userIDs []int64) (map[int64]string, error)
	// Distribute atomically debits the pool and credits each recipient's contribution
	// balance (available immediately, no freeze), writing a ledger row per recipient.
	Distribute(ctx context.Context, accountID int64, allocations []PoolAllocation) error
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

// DistributeAccountPool distributes the pool to the owner set (owner-only). mode=="even"
// splits the whole pool equally across all owners; otherwise the explicit allocations are
// used (each recipient must be an owner, amount>0, sum<=pool). No freeze — recipients can
// use/transfer the credited contribution balance immediately.
func (s *AccountContributionService) DistributeAccountPool(ctx context.Context, requesterID, accountID int64, mode string, allocations []PoolAllocation) error {
	_, owners, err := s.ownerSetFor(ctx, requesterID, accountID)
	if err != nil {
		return err
	}
	ownerSet := make(map[int64]struct{}, len(owners))
	for _, id := range owners {
		ownerSet[id] = struct{}{}
	}

	if mode == "even" {
		pool, err := s.poolRepo.GetPool(ctx, accountID)
		if err != nil {
			return err
		}
		if pool <= 0 || len(owners) == 0 {
			return ErrContributionPoolEmpty
		}
		per := roundTo(pool/float64(len(owners)), 8)
		if per <= 0 {
			return ErrContributionPoolEmpty
		}
		allocations = make([]PoolAllocation, 0, len(owners))
		var assigned float64
		for i, id := range owners {
			amt := per
			if i == len(owners)-1 {
				// last recipient absorbs the rounding remainder so the pool clears exactly.
				amt = roundTo(pool-assigned, 8)
			}
			if amt <= 0 {
				continue
			}
			assigned += amt
			allocations = append(allocations, PoolAllocation{UserID: id, Amount: amt})
		}
	} else {
		clean := make([]PoolAllocation, 0, len(allocations))
		for _, a := range allocations {
			if a.Amount <= 0 {
				continue
			}
			if _, ok := ownerSet[a.UserID]; !ok {
				return ErrContributionBadRecipient
			}
			clean = append(clean, PoolAllocation{UserID: a.UserID, Amount: roundTo(a.Amount, 8)})
		}
		if len(clean) == 0 {
			return ErrContributionPoolEmpty
		}
		allocations = clean
	}

	return s.poolRepo.Distribute(ctx, accountID, allocations)
}

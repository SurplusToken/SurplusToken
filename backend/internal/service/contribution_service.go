package service

import (
	"context"
	"math"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var ErrContributionQuotaEmpty = infraerrors.BadRequest("CONTRIBUTION_QUOTA_EMPTY", "no contribution quota available to transfer")

type ContributionSummary struct {
	UserID                   int64     `json:"user_id"`
	ContributionQuota        float64   `json:"contribution_quota"`
	ContributionFrozenQuota  float64   `json:"contribution_frozen_quota"`
	ContributionHistoryQuota float64   `json:"contribution_history_quota"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type ContributionTransferResponse struct {
	TransferredQuota         float64 `json:"transferred_quota"`
	Balance                  float64 `json:"balance"`
	ContributionQuota        float64 `json:"contribution_quota"`
	ContributionFrozenQuota  float64 `json:"contribution_frozen_quota"`
	ContributionHistoryQuota float64 `json:"contribution_history_quota"`
}

type ContributionRepository interface {
	EnsureUserContribution(ctx context.Context, userID int64) (*ContributionSummary, error)
	ThawFrozenQuota(ctx context.Context, userID int64) (float64, error)
	TransferQuotaToBalance(ctx context.Context, userID int64) (*ContributionTransferResponse, error)
}

type ContributionService struct {
	repo                 ContributionRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
}

func NewContributionService(repo ContributionRepository, authCacheInvalidator APIKeyAuthCacheInvalidator, billingCacheService *BillingCacheService) *ContributionService {
	return &ContributionService{
		repo:                 repo,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
	}
}

func (s *ContributionService) GetSummary(ctx context.Context, userID int64) (*ContributionSummary, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "contribution service unavailable")
	}
	_, _ = s.repo.ThawFrozenQuota(ctx, userID)
	return s.repo.EnsureUserContribution(ctx, userID)
}

func (s *ContributionService) TransferContributionQuota(ctx context.Context, userID int64) (*ContributionTransferResponse, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "contribution service unavailable")
	}
	result, err := s.repo.TransferQuotaToBalance(ctx, userID)
	if err != nil {
		return nil, err
	}
	if result != nil && result.TransferredQuota > 0 {
		s.invalidateContributionCaches(ctx, userID)
	}
	return result, nil
}

func (s *ContributionService) invalidateContributionCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			logger.LegacyPrintf("service.contribution", "[Contribution] Failed to invalidate billing cache for user %d: %v", userID, err)
		}
	}
}

// excludeUserID returns ids with every occurrence of exclude removed, preserving
// order. Used to drop the consumer from an account's owner set so they never
// receive a self-reward.
func excludeUserID(ids []int64, exclude int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == exclude {
			continue
		}
		out = append(out, id)
	}
	return out
}

func clampContributionRewardRate(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return ContributionRewardRateDefault
	}
	if value < ContributionRewardRateMin {
		return ContributionRewardRateMin
	}
	if value > ContributionRewardRateMax {
		return ContributionRewardRateMax
	}
	return value
}

func normalizeContributionFreezeHours(hours int) int {
	if hours < 0 {
		return ContributionRewardFreezeHoursDefault
	}
	if hours > ContributionRewardFreezeHoursMax {
		return ContributionRewardFreezeHoursMax
	}
	return hours
}

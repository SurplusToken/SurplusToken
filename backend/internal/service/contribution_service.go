package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var ErrContributionQuotaEmpty = infraerrors.BadRequest("CONTRIBUTION_QUOTA_EMPTY", "no contribution quota available to transfer")

var (
	ErrContributionWithdrawalNotFound       = infraerrors.NotFound("CONTRIBUTION_WITHDRAWAL_NOT_FOUND", "contribution withdrawal not found")
	ErrContributionWithdrawalPendingExists  = infraerrors.Conflict("CONTRIBUTION_WITHDRAWAL_PENDING_EXISTS", "a pending contribution withdrawal already exists")
	ErrContributionWithdrawalInvalidState   = infraerrors.Conflict("CONTRIBUTION_WITHDRAWAL_INVALID_STATE", "contribution withdrawal is no longer pending")
	ErrContributionWithdrawalInsufficient   = infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_INSUFFICIENT", "insufficient withdrawable contribution balance")
	ErrContributionWithdrawalIdempotencyKey = infraerrors.Conflict("CONTRIBUTION_WITHDRAWAL_IDEMPOTENCY_CONFLICT", "idempotency key was already used for a different withdrawal")
)

const (
	ContributionWithdrawalStatusPending   = "pending"
	ContributionWithdrawalStatusPaid      = "paid"
	ContributionWithdrawalStatusRejected  = "rejected"
	ContributionWithdrawalStatusCancelled = "cancelled"
)

type ContributionSummary struct {
	UserID                             int64     `json:"user_id"`
	ContributionQuota                  float64   `json:"contribution_quota"`
	ContributionFrozenQuota            float64   `json:"contribution_frozen_quota"`
	ContributionHistoryQuota           float64   `json:"contribution_history_quota"`
	ContributionPendingWithdrawalQuota float64   `json:"contribution_pending_withdrawal_quota"`
	ContributionWithdrawnQuota         float64   `json:"contribution_withdrawn_quota"`
	ContributionTransferredQuota       float64   `json:"contribution_transferred_quota"`
	CreatedAt                          time.Time `json:"created_at"`
	UpdatedAt                          time.Time `json:"updated_at"`
}

type ContributionWithdrawal struct {
	ID               int64      `json:"id"`
	UserID           int64      `json:"user_id"`
	UserEmail        string     `json:"user_email,omitempty"`
	Username         string     `json:"username,omitempty"`
	Amount           float64    `json:"amount"`
	Status           string     `json:"status"`
	PaymentMethod    string     `json:"payment_method"`
	PaymentAccount   string     `json:"payment_account"`
	PayeeName        string     `json:"payee_name"`
	RequestNote      string     `json:"request_note"`
	ReviewNote       string     `json:"review_note"`
	PaymentReference string     `json:"payment_reference"`
	ReviewedBy       *int64     `json:"reviewed_by,omitempty"`
	RequestedAt      time.Time  `json:"requested_at"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	CancelledAt      *time.Time `json:"cancelled_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type CreateContributionWithdrawalRequest struct {
	Amount             float64
	PaymentMethod      string
	PaymentAccount     string
	PayeeName          string
	RequestNote        string
	IdempotencyKey     string
	RequestFingerprint string
}

type ReviewContributionWithdrawalRequest struct {
	WithdrawalID     int64
	AdminUserID      int64
	Status           string
	ReviewNote       string
	PaymentReference string
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
	CreateWithdrawal(ctx context.Context, userID int64, req CreateContributionWithdrawalRequest) (*ContributionWithdrawal, error)
	ListWithdrawals(ctx context.Context, userID int64, page, pageSize int) ([]ContributionWithdrawal, int64, error)
	ListWithdrawalsAdmin(ctx context.Context, status, search string, page, pageSize int) ([]ContributionWithdrawal, int64, error)
	CancelWithdrawal(ctx context.Context, userID, withdrawalID int64) (*ContributionWithdrawal, error)
	ReviewWithdrawal(ctx context.Context, req ReviewContributionWithdrawalRequest) (*ContributionWithdrawal, error)
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

func (s *ContributionService) CreateWithdrawal(ctx context.Context, userID int64, req CreateContributionWithdrawalRequest) (*ContributionWithdrawal, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "contribution service unavailable")
	}
	if err := normalizeContributionWithdrawalRequest(&req); err != nil {
		return nil, err
	}
	result, err := s.repo.CreateWithdrawal(ctx, userID, req)
	if err == nil {
		s.invalidateContributionCaches(ctx, userID)
	}
	return result, err
}

func (s *ContributionService) ListWithdrawals(ctx context.Context, userID int64, page, pageSize int) ([]ContributionWithdrawal, int64, error) {
	if userID <= 0 {
		return nil, 0, ErrUserNotFound
	}
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "contribution service unavailable")
	}
	page, pageSize = normalizeContributionWithdrawalPagination(page, pageSize)
	return s.repo.ListWithdrawals(ctx, userID, page, pageSize)
}

func (s *ContributionService) ListWithdrawalsAdmin(ctx context.Context, status, search string, page, pageSize int) ([]ContributionWithdrawal, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "contribution service unavailable")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && !isContributionWithdrawalStatus(status) {
		return nil, 0, infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_STATUS_INVALID", "invalid contribution withdrawal status")
	}
	search = strings.TrimSpace(search)
	if len(search) > 100 {
		search = search[:100]
	}
	page, pageSize = normalizeContributionWithdrawalPagination(page, pageSize)
	return s.repo.ListWithdrawalsAdmin(ctx, status, search, page, pageSize)
}

func (s *ContributionService) CancelWithdrawal(ctx context.Context, userID, withdrawalID int64) (*ContributionWithdrawal, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	if withdrawalID <= 0 {
		return nil, ErrContributionWithdrawalNotFound
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "contribution service unavailable")
	}
	result, err := s.repo.CancelWithdrawal(ctx, userID, withdrawalID)
	if err == nil {
		s.invalidateContributionCaches(ctx, userID)
	}
	return result, err
}

func (s *ContributionService) ReviewWithdrawal(ctx context.Context, req ReviewContributionWithdrawalRequest) (*ContributionWithdrawal, error) {
	if req.AdminUserID <= 0 {
		return nil, ErrUserNotFound
	}
	if req.WithdrawalID <= 0 {
		return nil, ErrContributionWithdrawalNotFound
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "contribution service unavailable")
	}
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.ReviewNote = strings.TrimSpace(req.ReviewNote)
	req.PaymentReference = strings.TrimSpace(req.PaymentReference)
	if len(req.ReviewNote) > 500 {
		return nil, infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_REVIEW_NOTE_INVALID", "review_note must not exceed 500 characters")
	}
	switch req.Status {
	case ContributionWithdrawalStatusPaid:
		if req.PaymentReference == "" {
			return nil, infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_REFERENCE_REQUIRED", "payment_reference is required when marking a withdrawal paid")
		}
		if len(req.PaymentReference) > 255 {
			return nil, infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_REFERENCE_INVALID", "payment_reference must not exceed 255 characters")
		}
	case ContributionWithdrawalStatusRejected:
		if req.ReviewNote == "" {
			return nil, infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_REVIEW_NOTE_REQUIRED", "review_note is required when rejecting a withdrawal")
		}
	default:
		return nil, infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_STATUS_INVALID", "admin status must be paid or rejected")
	}
	result, err := s.repo.ReviewWithdrawal(ctx, req)
	if err == nil && result != nil {
		s.invalidateContributionCaches(ctx, result.UserID)
	}
	return result, err
}

func normalizeContributionWithdrawalRequest(req *CreateContributionWithdrawalRequest) error {
	if req == nil {
		return infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_INVALID", "withdrawal request is required")
	}
	if math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) || req.Amount < 0.01 {
		return infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_AMOUNT_INVALID", "withdrawal amount must be at least 0.01")
	}
	rounded := math.Round(req.Amount*1e8) / 1e8
	if math.Abs(req.Amount-rounded) > 1e-10 {
		return infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_AMOUNT_INVALID", "withdrawal amount supports at most 8 decimal places")
	}
	req.Amount = rounded
	req.PaymentMethod = strings.ToLower(strings.TrimSpace(req.PaymentMethod))
	switch req.PaymentMethod {
	case "alipay", "wechat", "bank", "other":
	default:
		return infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_METHOD_INVALID", "unsupported payment_method")
	}
	req.PaymentAccount = strings.TrimSpace(req.PaymentAccount)
	req.PayeeName = strings.TrimSpace(req.PayeeName)
	req.RequestNote = strings.TrimSpace(req.RequestNote)
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if req.PaymentAccount == "" || len(req.PaymentAccount) > 255 {
		return infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_ACCOUNT_INVALID", "payment_account is required and must not exceed 255 characters")
	}
	if req.PayeeName == "" || len(req.PayeeName) > 100 {
		return infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_PAYEE_INVALID", "payee_name is required and must not exceed 100 characters")
	}
	if len(req.RequestNote) > 500 {
		return infraerrors.BadRequest("CONTRIBUTION_WITHDRAWAL_NOTE_INVALID", "request_note must not exceed 500 characters")
	}
	if req.IdempotencyKey == "" || len(req.IdempotencyKey) > 128 {
		return infraerrors.BadRequest("IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key is required and must not exceed 128 characters")
	}
	fingerprintInput := fmt.Sprintf("%.8f\x00%s\x00%s\x00%s\x00%s", req.Amount, req.PaymentMethod, req.PaymentAccount, req.PayeeName, req.RequestNote)
	digest := sha256.Sum256([]byte(fingerprintInput))
	req.RequestFingerprint = hex.EncodeToString(digest[:])
	return nil
}

func normalizeContributionWithdrawalPagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func isContributionWithdrawalStatus(status string) bool {
	switch status {
	case ContributionWithdrawalStatusPending, ContributionWithdrawalStatusPaid, ContributionWithdrawalStatusRejected, ContributionWithdrawalStatusCancelled:
		return true
	default:
		return false
	}
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

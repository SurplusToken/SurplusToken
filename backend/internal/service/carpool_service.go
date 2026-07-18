package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	CarpoolTypeSmall = "small"
	CarpoolTypeLarge = "large"

	CarpoolVisibilityPublic     = "public"
	CarpoolVisibilityInviteOnly = "invite_only"
)

var (
	ErrCarpoolNotFound       = infraerrors.NotFound("CARPOOL_NOT_FOUND", "carpool not found")
	ErrCarpoolUnavailable    = infraerrors.Conflict("CARPOOL_UNAVAILABLE", "carpool is not accepting members")
	ErrCarpoolAlreadyJoined  = infraerrors.Conflict("CARPOOL_ALREADY_JOINED", "user already joined this carpool")
	ErrCarpoolForbidden      = infraerrors.Forbidden("CARPOOL_FORBIDDEN", "operation is not allowed for this carpool")
	ErrCarpoolInviteInvalid  = infraerrors.NotFound("CARPOOL_INVITE_INVALID", "carpool invite is invalid or expired")
	ErrCarpoolNameConflict   = infraerrors.Conflict("CARPOOL_NAME_CONFLICT", "an active group already uses this carpool name")
	ErrCarpoolInvalidRequest = infraerrors.BadRequest("CARPOOL_INVALID_REQUEST", "invalid carpool request")
)

type Carpool struct {
	ID                     int64      `json:"id"`
	Name                   string     `json:"name"`
	Description            string     `json:"description"`
	Organizer              string     `json:"organizer"`
	OwnerUserID            *int64     `json:"owner_user_id,omitempty"`
	Platform               string     `json:"platform"`
	PlanType               string     `json:"plan_type"`
	CarType                string     `json:"car_type"`
	Level                  int        `json:"level"`
	Capacity               int        `json:"capacity"`
	MemberCount            int        `json:"member_count"`
	BaseFeeCNY             float64    `json:"base_fee_cny"`
	UsagePoolPerAccountCNY float64    `json:"usage_pool_cny_per_account"`
	Visibility             string     `json:"visibility"`
	Status                 string     `json:"status"`
	JoinLocked             bool       `json:"join_locked"`
	ScheduledStartAt       *time.Time `json:"scheduled_start_at,omitempty"`
	LaunchedAt             *time.Time `json:"launched_at,omitempty"`
	GroupID                *int64     `json:"group_id,omitempty"`
	GroupName              *string    `json:"group_name,omitempty"`
	MemberRole             *string    `json:"member_role"`
	CreatedAt              time.Time  `json:"created_at"`
}

type CreateCarpoolInput struct {
	Name             string
	Description      string
	CarType          string
	Level            int
	Visibility       string
	ScheduledStartAt *time.Time
}

type CarpoolMutationResult struct {
	Carpool          *Carpool `json:"carpool"`
	InviteToken      string   `json:"invite_token,omitempty"`
	ActivatedUserIDs []int64  `json:"-"`
	ActivatedGroupID int64    `json:"-"`
}

type CarpoolRepository interface {
	List(ctx context.Context, userID int64) ([]Carpool, error)
	GetByInvite(ctx context.Context, userID int64, tokenHash string) (*Carpool, error)
	Create(ctx context.Context, ownerUserID int64, input CreateCarpoolInput, inviteHash, inviteHint string) (*CarpoolMutationResult, error)
	CreateInvite(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool, inviteHash, inviteHint string) error
	Join(ctx context.Context, carpoolID, userID int64, inviteHash *string) (*CarpoolMutationResult, error)
	Cancel(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error
	SetJoinLocked(ctx context.Context, carpoolID, actorUserID int64, locked bool) error
}

type CarpoolService struct {
	repo                CarpoolRepository
	subscriptionService *SubscriptionService
}

func NewCarpoolService(repo CarpoolRepository, subscriptionService *SubscriptionService) *CarpoolService {
	return &CarpoolService{repo: repo, subscriptionService: subscriptionService}
}

func (s *CarpoolService) List(ctx context.Context, userID int64) ([]Carpool, error) {
	return s.repo.List(ctx, userID)
}

func (s *CarpoolService) ResolveInvite(ctx context.Context, userID int64, token string) (*Carpool, error) {
	return s.repo.GetByInvite(ctx, userID, hashInviteToken(token))
}

func (s *CarpoolService) Create(ctx context.Context, ownerUserID int64, input CreateCarpoolInput) (*CarpoolMutationResult, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" || len(input.Name) > 100 || len(input.Description) > 300 || input.Level < 1 || input.Level > 10 {
		return nil, ErrCarpoolInvalidRequest
	}
	if input.CarType != CarpoolTypeSmall && input.CarType != CarpoolTypeLarge {
		return nil, ErrCarpoolInvalidRequest
	}
	if input.Visibility != CarpoolVisibilityPublic && input.Visibility != CarpoolVisibilityInviteOnly {
		return nil, ErrCarpoolInvalidRequest
	}
	token, hash, hint, err := newInviteToken()
	if err != nil {
		return nil, fmt.Errorf("generate carpool invite: %w", err)
	}
	result, err := s.repo.Create(ctx, ownerUserID, input, hash, hint)
	if err != nil {
		return nil, err
	}
	result.InviteToken = token
	return result, nil
}

func (s *CarpoolService) CreateInvite(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (string, error) {
	token, hash, hint, err := newInviteToken()
	if err != nil {
		return "", fmt.Errorf("generate carpool invite: %w", err)
	}
	if err := s.repo.CreateInvite(ctx, carpoolID, actorUserID, isAdmin, hash, hint); err != nil {
		return "", err
	}
	return token, nil
}

func (s *CarpoolService) Join(ctx context.Context, carpoolID, userID int64) (*CarpoolMutationResult, error) {
	result, err := s.repo.Join(ctx, carpoolID, userID, nil)
	if err != nil {
		return nil, err
	}
	s.invalidateLaunchedSubscriptions(result)
	return result, nil
}

func (s *CarpoolService) JoinByInvite(ctx context.Context, token string, userID int64) (*CarpoolMutationResult, error) {
	hash := hashInviteToken(token)
	result, err := s.repo.Join(ctx, 0, userID, &hash)
	if err != nil {
		return nil, err
	}
	s.invalidateLaunchedSubscriptions(result)
	return result, nil
}

func (s *CarpoolService) Cancel(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error {
	return s.repo.Cancel(ctx, carpoolID, actorUserID, isAdmin)
}

func (s *CarpoolService) SetJoinLocked(ctx context.Context, carpoolID, actorUserID int64, isAdmin, locked bool) error {
	if !isAdmin {
		return ErrCarpoolForbidden
	}
	return s.repo.SetJoinLocked(ctx, carpoolID, actorUserID, locked)
}

func (s *CarpoolService) invalidateLaunchedSubscriptions(result *CarpoolMutationResult) {
	if result == nil || result.ActivatedGroupID <= 0 || s.subscriptionService == nil {
		return
	}
	for _, userID := range result.ActivatedUserIDs {
		_ = s.subscriptionService.invalidateSubscriptionCaches(userID, result.ActivatedGroupID)
	}
}

func newInviteToken() (token, hash, hint string, err error) {
	raw := make([]byte, 24)
	if _, err = rand.Read(raw); err != nil {
		return "", "", "", err
	}
	token = hex.EncodeToString(raw)
	hash = hashInviteToken(token)
	hint = token[:8]
	return token, hash, hint, nil
}

func hashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

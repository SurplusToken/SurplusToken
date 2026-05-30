//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type accountPoolRepoStub struct {
	accountRepoStub

	created      *Account
	accounts     []Account
	pagination   *pagination.PaginationResult
	accountsByID map[int64]*Account
	updated      *Account
	extraUpdates map[string]any
	boundGroups  []int64
}

func (s *accountPoolRepoStub) Create(ctx context.Context, account *Account) error {
	copied := *account
	if copied.ID == 0 {
		copied.ID = 101
	}
	if copied.CreatedAt.IsZero() {
		copied.CreatedAt = time.Now()
	}
	copied.UpdatedAt = copied.CreatedAt
	s.created = &copied
	*account = copied
	return nil
}

func (s *accountPoolRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	if s.pagination != nil {
		return s.accounts, s.pagination, nil
	}
	return s.accounts, &pagination.PaginationResult{
		Total:    int64(len(s.accounts)),
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    1,
	}, nil
}

func (s *accountPoolRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	if s.accountsByID == nil {
		return nil, ErrAccountNotFound
	}
	account := s.accountsByID[id]
	if account == nil {
		return nil, ErrAccountNotFound
	}
	copied := *account
	if account.Extra != nil {
		copied.Extra = make(map[string]any, len(account.Extra))
		for k, v := range account.Extra {
			copied.Extra[k] = v
		}
	}
	return &copied, nil
}

func (s *accountPoolRepoStub) Update(ctx context.Context, account *Account) error {
	copied := *account
	s.updated = &copied
	if s.accountsByID != nil {
		accountCopy := copied
		s.accountsByID[account.ID] = &accountCopy
	}
	return nil
}

func (s *accountPoolRepoStub) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	s.extraUpdates = make(map[string]any, len(updates))
	for k, v := range updates {
		s.extraUpdates[k] = v
	}
	return nil
}

func (s *accountPoolRepoStub) BindGroups(ctx context.Context, accountID int64, groupIDs []int64) error {
	s.boundGroups = append([]int64(nil), groupIDs...)
	if s.accountsByID != nil {
		if account := s.accountsByID[accountID]; account != nil {
			account.GroupIDs = append([]int64(nil), groupIDs...)
		}
	}
	return nil
}

func TestAccountServiceCreateUserOAuthAccount(t *testing.T) {
	repo := &accountPoolRepoStub{}
	svc := &AccountService{accountRepo: repo}

	item, err := svc.CreateUserOAuthAccount(context.Background(), 42, CreateUserOAuthAccountRequest{
		Name:                               "  Team OpenAI  ",
		Platform:                           PlatformOpenAI,
		Type:                               AccountTypeOAuth,
		Credentials:                        map[string]any{"refresh_token": "secret"},
		Extra:                              map[string]any{"window_cost_limit": 50.0, "quota_weekly_min_remaining": 20.0},
		ContributionFiveHourReservePercent: accountPoolFloatPtr(20),
		ContributionWeeklyReservePercent:   accountPoolFloatPtr(30),
		ContributionProbeFailurePolicy:     accountPoolStringPtr(ContributionProbeFailurePolicyPause),
	})

	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, int64(42), *repo.created.OwnerUserID)
	require.Equal(t, AccountTypeOAuth, repo.created.Type)
	require.Equal(t, "Team OpenAI", repo.created.Name)
	require.True(t, repo.created.Schedulable)
	require.Equal(t, 20.0, repo.created.Extra["contribution_5h_reserve_percent"])
	require.Equal(t, 30.0, repo.created.Extra["contribution_weekly_reserve_percent"])
	require.Equal(t, ContributionProbeFailurePolicyPause, repo.created.Extra["contribution_probe_failure_policy"])
	require.NotContains(t, repo.created.Extra, "window_cost_limit")
	require.NotContains(t, repo.created.Extra, "quota_weekly_min_remaining")
	require.True(t, item.IsMine)
}

func TestAccountServiceCreateUserOAuthAccountRejectsNonOAuthType(t *testing.T) {
	repo := &accountPoolRepoStub{}
	svc := &AccountService{accountRepo: repo}

	_, err := svc.CreateUserOAuthAccount(context.Background(), 42, CreateUserOAuthAccountRequest{
		Name:        "bad",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "secret"},
	})

	require.ErrorIs(t, err, ErrUserOAuthOnly)
	require.Nil(t, repo.created)
}

func TestAccountServiceCreateUserOAuthAccountRejectsNonOpenAIPlatform(t *testing.T) {
	repo := &accountPoolRepoStub{}
	svc := &AccountService{accountRepo: repo}

	_, err := svc.CreateUserOAuthAccount(context.Background(), 42, CreateUserOAuthAccountRequest{
		Name:        "gemini",
		Platform:    PlatformGemini,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"refresh_token": "secret"},
	})

	require.Error(t, err)
	require.Nil(t, repo.created)
}

func TestAccountServiceUserAccountOwnership(t *testing.T) {
	ownerID := int64(42)
	account := &Account{
		ID:          7,
		Name:        "mine",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		OwnerUserID: &ownerID,
	}
	repo := &accountPoolRepoStub{accountsByID: map[int64]*Account{7: account}}
	svc := &AccountService{accountRepo: repo}

	item, err := svc.SetUserAccountSchedulable(context.Background(), ownerID, 7, false)
	require.NoError(t, err)
	require.NotNil(t, item)
	require.False(t, repo.updated.Schedulable)

	_, err = svc.SetUserAccountSchedulable(context.Background(), 99, 7, true)
	require.ErrorIs(t, err, ErrAccountOwnerRequired)
	require.Equal(t, 403, infraerrors.Code(err))
}

func TestAccountServiceUpdateUserAccountLimits(t *testing.T) {
	ownerID := int64(42)
	account := &Account{
		ID:          7,
		Name:        "mine",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		OwnerUserID: &ownerID,
		Extra: map[string]any{
			"quota_weekly_used":  70.0,
			"quota_weekly_start": time.Now().Add(-time.Hour).Format(time.RFC3339),
		},
	}
	repo := &accountPoolRepoStub{accountsByID: map[int64]*Account{7: account}}
	svc := &AccountService{accountRepo: repo}

	item, err := svc.UpdateUserAccountLimits(context.Background(), ownerID, 7, UpdateUserAccountLimitsRequest{
		WindowCostLimit:         accountPoolFloatPtr(50),
		QuotaWeeklyLimit:        accountPoolFloatPtr(100),
		QuotaWeeklyMinRemaining: accountPoolFloatPtr(20),
	})

	require.NoError(t, err)
	require.Equal(t, 50.0, repo.extraUpdates["window_cost_limit"])
	require.Equal(t, 20.0, repo.extraUpdates["quota_weekly_min_remaining"])
	require.Equal(t, 30.0, item.QuotaWeeklyRemaining)
	require.False(t, item.WeeklyRemainingBelowPolicy)
}

func TestAccountServiceUpdateUserAccountScope(t *testing.T) {
	ownerID := int64(42)
	account := &Account{
		ID:          7,
		Name:        "mine",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		OwnerUserID: &ownerID,
		Credentials: map[string]any{"refresh_token": "secret", "model_mapping": map[string]any{"old": "old"}},
		Extra: map[string]any{
			"codex_cli_only":                      true,
			"window_cost_limit":                   50.0,
			"contribution_5h_reserve_percent":     5.0,
			"contribution_weekly_reserve_percent": 10.0,
		},
	}
	repo := &accountPoolRepoStub{accountsByID: map[int64]*Account{7: account}}
	svc := &AccountService{accountRepo: repo}
	expiresAt := time.Now().Add(time.Hour).Unix()
	codexOnly := false
	modelMapping := map[string]string{"gpt-5.2": "gpt-5.2", " ": "ignored"}
	groupIDs := []int64{11, 12}

	item, err := svc.UpdateUserAccountScope(context.Background(), ownerID, 7, UpdateUserAccountScopeRequest{
		GroupIDs:                           &groupIDs,
		ExpiresAt:                          &expiresAt,
		AutoPauseOnExpired:                 accountPoolBoolPtr(true),
		ModelMapping:                       &modelMapping,
		CodexCLIOnly:                       &codexOnly,
		ContributionFiveHourReservePercent: accountPoolFloatPtr(25),
		ContributionWeeklyReservePercent:   accountPoolFloatPtr(35),
		ContributionProbeFailurePolicy:     accountPoolStringPtr(ContributionProbeFailurePolicyLocal),
	})

	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, []int64{11, 12}, repo.boundGroups)
	require.Equal(t, map[string]any{"gpt-5.2": "gpt-5.2"}, repo.updated.Credentials["model_mapping"])
	require.NotContains(t, repo.updated.Extra, "codex_cli_only")
	require.Equal(t, 50.0, repo.updated.Extra["window_cost_limit"])
	require.Equal(t, 25.0, repo.updated.Extra["contribution_5h_reserve_percent"])
	require.Equal(t, 35.0, repo.updated.Extra["contribution_weekly_reserve_percent"])
	require.Equal(t, ContributionProbeFailurePolicyLocal, repo.updated.Extra["contribution_probe_failure_policy"])
	require.NotNil(t, repo.updated.ExpiresAt)
	require.Equal(t, expiresAt, repo.updated.ExpiresAt.Unix())
	require.Equal(t, []int64{11, 12}, item.GroupIDs)
	require.False(t, item.CodexCLIOnly)
}

func TestAccountServiceListUserAccountPoolSafeDTO(t *testing.T) {
	ownerID := int64(42)
	otherID := int64(99)
	repo := &accountPoolRepoStub{accounts: []Account{
		{
			ID:          1,
			Name:        "mine",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			OwnerUserID: &ownerID,
			Credentials: map[string]any{"refresh_token": "secret", "plan_type": "plus", "subscription_expires_at": "2026-06-01T00:00:00Z"},
			Extra:       map[string]any{"window_cost_limit": 10.0, "privacy_mode": "training_off", "private": "hidden"},
		},
		{
			ID:          2,
			Name:        "other",
			Platform:    PlatformGemini,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			OwnerUserID: &otherID,
			Credentials: map[string]any{"refresh_token": "other-secret"},
		},
	}}
	svc := &AccountService{accountRepo: repo}

	items, _, err := svc.ListUserAccountPool(context.Background(), ownerID, pagination.PaginationParams{Page: 1, PageSize: 20}, UserAccountPoolListFilters{})

	require.NoError(t, err)
	require.Len(t, items, 2)
	require.True(t, items[0].IsMine)
	require.False(t, items[1].IsMine)
	require.Equal(t, "plus", items[0].PlanType)
	require.Equal(t, "training_off", items[0].PrivacyMode)
	require.Equal(t, "2026-06-01T00:00:00Z", items[0].SubscriptionExpiresAt)
	payload, err := json.Marshal(items[0])
	require.NoError(t, err)
	require.NotContains(t, string(payload), "refresh_token")
	require.NotContains(t, string(payload), "secret")
	require.NotContains(t, string(payload), "private")
}

func TestAccountServiceListUserAccountPoolPlanTypeFilterFallback(t *testing.T) {
	repo := &accountPoolRepoStub{accounts: []Account{
		{ID: 1, Name: "plus", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Credentials: map[string]any{"plan_type": "plus"}},
		{ID: 2, Name: "pro", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Credentials: map[string]any{"plan_type": "chatgptpro"}},
		{ID: 3, Name: "free", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Credentials: map[string]any{"plan_type": "free"}},
	}}
	svc := &AccountService{accountRepo: repo}

	items, page, err := svc.ListUserAccountPool(context.Background(), 42, pagination.PaginationParams{Page: 1, PageSize: 20}, UserAccountPoolListFilters{PlanType: "pro"})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(2), items[0].ID)
	require.Equal(t, int64(1), page.Total)
}

func TestFilterSurplusAISchedulableAccountsWeeklyReserve(t *testing.T) {
	now := time.Now()
	accounts := []Account{
		{
			ID:          1,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Extra: map[string]any{
				"quota_weekly_limit":         100.0,
				"quota_weekly_used":          90.0,
				"quota_weekly_min_remaining": 20.0,
				"quota_weekly_start":         now.Add(-time.Hour).Format(time.RFC3339),
			},
		},
		{
			ID:          2,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Extra: map[string]any{
				"quota_weekly_limit":         100.0,
				"quota_weekly_used":          70.0,
				"quota_weekly_min_remaining": 20.0,
				"quota_weekly_start":         now.Add(-time.Hour).Format(time.RFC3339),
			},
		},
		{
			ID:          3,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
		},
	}

	filtered := filterSurplusAISchedulableAccounts(accounts)

	require.Len(t, filtered, 1)
	require.Equal(t, int64(2), filtered[0].ID)
}

func accountPoolFloatPtr(v float64) *float64 {
	return &v
}

func accountPoolBoolPtr(v bool) *bool {
	return &v
}

func accountPoolStringPtr(v string) *string {
	return &v
}

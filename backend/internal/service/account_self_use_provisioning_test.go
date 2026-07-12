//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// ---- fakes ---------------------------------------------------------------

type selfUseSubFake struct {
	active       []UserSubscription
	assignCalls  int
	lastAssign   *AssignSubscriptionInput
	getActiveSub *UserSubscription
	revokedIDs   []int64
}

func (f *selfUseSubFake) ListActiveUserSubscriptions(_ context.Context, _ int64) ([]UserSubscription, error) {
	return f.active, nil
}

func (f *selfUseSubFake) AssignOrExtendSubscription(_ context.Context, input *AssignSubscriptionInput) (*UserSubscription, bool, error) {
	f.assignCalls++
	f.lastAssign = input
	return &UserSubscription{ID: 555, UserID: input.UserID, GroupID: input.GroupID}, false, nil
}

func (f *selfUseSubFake) GetActiveSubscription(_ context.Context, _, _ int64) (*UserSubscription, error) {
	return f.getActiveSub, nil
}

func (f *selfUseSubFake) RevokeSubscription(_ context.Context, id int64) error {
	f.revokedIDs = append(f.revokedIDs, id)
	return nil
}

type selfUseKeyFake struct {
	existing    []APIKey
	createCalls int
	lastCreate  *CreateAPIKeyRequest
	deletedIDs  []int64
}

func (f *selfUseKeyFake) List(_ context.Context, _ int64, _ pagination.PaginationParams, _ APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	return f.existing, nil, nil
}

func (f *selfUseKeyFake) Create(_ context.Context, _ int64, req CreateAPIKeyRequest) (*APIKey, error) {
	f.createCalls++
	r := req
	f.lastCreate = &r
	return &APIKey{ID: 777}, nil
}

func (f *selfUseKeyFake) Delete(_ context.Context, id int64, _ int64) error {
	f.deletedIDs = append(f.deletedIDs, id)
	return nil
}

type selfUseGroupRepoFake struct {
	groupRepoStub
	created      *Group
	byID         map[int64]*Group
	bound        map[int64][]int64
	accountCount int64
	deletedIDs   []int64
}

func (f *selfUseGroupRepoFake) Create(_ context.Context, g *Group) error {
	g.ID = 900
	cp := *g
	f.created = &cp
	if f.byID == nil {
		f.byID = map[int64]*Group{}
	}
	f.byID[g.ID] = &cp
	return nil
}

func (f *selfUseGroupRepoFake) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	if g, ok := f.byID[id]; ok {
		cp := *g
		return &cp, nil
	}
	return nil, ErrGroupNotFound
}

func (f *selfUseGroupRepoFake) BindAccountsToGroup(_ context.Context, groupID int64, accountIDs []int64) error {
	if f.bound == nil {
		f.bound = map[int64][]int64{}
	}
	f.bound[groupID] = append(f.bound[groupID], accountIDs...)
	return nil
}

func (f *selfUseGroupRepoFake) GetAccountCount(_ context.Context, _ int64) (int64, int64, error) {
	return f.accountCount, f.accountCount, nil
}

func (f *selfUseGroupRepoFake) Delete(_ context.Context, id int64) error {
	f.deletedIDs = append(f.deletedIDs, id)
	return nil
}

// ---- tests ---------------------------------------------------------------

func TestEnsureSelfUseAccess_FirstContribution(t *testing.T) {
	gr := &selfUseGroupRepoFake{}
	sub := &selfUseSubFake{}
	key := &selfUseKeyFake{}
	svc := &AccountService{groupRepo: gr, subscriptionSvc: sub, apiKeyService: key}

	acc := &Account{ID: 42, Platform: PlatformOpenAI, OwnerUserID: int64Ptr(7)}
	require.NoError(t, svc.ensureSelfUseAccess(context.Background(), 7, acc))

	// A self-use group is created with the required config.
	require.NotNil(t, gr.created)
	require.True(t, gr.created.AutoSelfUse)
	require.Equal(t, SubscriptionTypeSubscription, gr.created.SubscriptionType)
	require.True(t, gr.created.AllowImageGeneration)
	require.Nil(t, gr.created.DailyLimitUSD)
	require.Nil(t, gr.created.WeeklyLimitUSD)
	require.Nil(t, gr.created.MonthlyLimitUSD)
	require.Equal(t, StatusActive, gr.created.Status)

	// The account is bound to the new group (additively).
	require.Equal(t, []int64{42}, gr.bound[900])
	// Subscription assigned before the key, key is unlimited.
	require.Equal(t, 1, sub.assignCalls)
	require.Equal(t, int64(900), sub.lastAssign.GroupID)
	require.Equal(t, 1, key.createCalls)
	require.NotNil(t, key.lastCreate.GroupID)
	require.Equal(t, int64(900), *key.lastCreate.GroupID)
	require.Equal(t, float64(0), key.lastCreate.Quota)
}

func TestEnsureSelfUseAccess_ReusesExistingGroup(t *testing.T) {
	gr := &selfUseGroupRepoFake{
		byID: map[int64]*Group{
			900: {ID: 900, AutoSelfUse: true, SubscriptionType: SubscriptionTypeSubscription},
		},
	}
	sub := &selfUseSubFake{active: []UserSubscription{{ID: 1, UserID: 7, GroupID: 900}}}
	key := &selfUseKeyFake{existing: []APIKey{{ID: 5}}} // user already has a self-use key
	svc := &AccountService{groupRepo: gr, subscriptionSvc: sub, apiKeyService: key}

	acc := &Account{ID: 43, Platform: PlatformOpenAI, OwnerUserID: int64Ptr(7)}
	require.NoError(t, svc.ensureSelfUseAccess(context.Background(), 7, acc))

	// No new group, no new key; only additive bind + idempotent subscription extend.
	require.Nil(t, gr.created)
	require.Equal(t, []int64{43}, gr.bound[900])
	require.Equal(t, 1, sub.assignCalls)
	require.Equal(t, 0, key.createCalls)
}

func TestTeardownSelfUseAccess_EmptyGroupRevokesEverything(t *testing.T) {
	gr := &selfUseGroupRepoFake{
		byID:         map[int64]*Group{900: {ID: 900, AutoSelfUse: true}},
		accountCount: 0, // no live accounts left
	}
	sub := &selfUseSubFake{
		active:       []UserSubscription{{ID: 1, UserID: 7, GroupID: 900}},
		getActiveSub: &UserSubscription{ID: 1, UserID: 7, GroupID: 900},
	}
	key := &selfUseKeyFake{existing: []APIKey{{ID: 5}}}
	svc := &AccountService{groupRepo: gr, subscriptionSvc: sub, apiKeyService: key}

	require.NoError(t, svc.teardownSelfUseAccessIfEmpty(context.Background(), 7))
	require.Equal(t, []int64{1}, sub.revokedIDs)
	require.Equal(t, []int64{5}, key.deletedIDs)
	require.Equal(t, []int64{900}, gr.deletedIDs)
}

func TestTeardownSelfUseAccess_KeepsWhenAccountsRemain(t *testing.T) {
	gr := &selfUseGroupRepoFake{
		byID:         map[int64]*Group{900: {ID: 900, AutoSelfUse: true}},
		accountCount: 2, // other contributed accounts remain
	}
	sub := &selfUseSubFake{active: []UserSubscription{{ID: 1, UserID: 7, GroupID: 900}}}
	key := &selfUseKeyFake{existing: []APIKey{{ID: 5}}}
	svc := &AccountService{groupRepo: gr, subscriptionSvc: sub, apiKeyService: key}

	require.NoError(t, svc.teardownSelfUseAccessIfEmpty(context.Background(), 7))
	require.Empty(t, sub.revokedIDs)
	require.Empty(t, key.deletedIDs)
	require.Empty(t, gr.deletedIDs)
}

func TestEnsureSelfUseAccess_InertWithoutDeps(t *testing.T) {
	gr := &selfUseGroupRepoFake{}
	// No subscription / api-key deps wired.
	svc := &AccountService{groupRepo: gr}

	acc := &Account{ID: 44, Platform: PlatformOpenAI, OwnerUserID: int64Ptr(7)}
	require.NoError(t, svc.ensureSelfUseAccess(context.Background(), 7, acc))
	require.Nil(t, gr.created)
	require.Empty(t, gr.bound)
}

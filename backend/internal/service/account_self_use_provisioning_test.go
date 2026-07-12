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
	createdAll   []*Group
	byID         map[int64]*Group
	byName       map[string]int64
	nextID       int64
	bound        map[int64][]int64
	accountCount int64
	deletedIDs   []int64
}

func (f *selfUseGroupRepoFake) Create(_ context.Context, g *Group) error {
	if f.byName == nil {
		f.byName = map[string]int64{}
	}
	// Mirrors the production partial-unique index groups_name_unique_active: a
	// duplicate group name must fail. Without this the fake silently hid the bug
	// where every contributor after the first collided on a fixed group name.
	if _, dup := f.byName[g.Name]; dup {
		return ErrGroupExists
	}
	if f.nextID == 0 {
		f.nextID = 900
	}
	g.ID = f.nextID
	f.nextID++
	cp := *g
	f.created = &cp
	f.createdAll = append(f.createdAll, &cp)
	if f.byID == nil {
		f.byID = map[int64]*Group{}
	}
	f.byID[g.ID] = &cp
	f.byName[g.Name] = g.ID
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

// Each contributor must get their OWN self-use group: groups.name carries a partial
// unique index, so a fixed name would make every contributor after the first fail
// with ErrGroupExists (silently, since provisioning is best-effort) — and their
// accounts must never land in someone else's self-use group.
func TestEnsureSelfUseAccess_SecondContributorGetsOwnGroup(t *testing.T) {
	gr := &selfUseGroupRepoFake{}
	key := &selfUseKeyFake{}

	// User 7 contributes first.
	svc7 := &AccountService{groupRepo: gr, subscriptionSvc: &selfUseSubFake{}, apiKeyService: key}
	require.NoError(t, svc7.ensureSelfUseAccess(context.Background(), 7, &Account{ID: 42, Platform: PlatformOpenAI}))

	// User 8 contributes next, holding no self-use subscription of their own.
	svc8 := &AccountService{groupRepo: gr, subscriptionSvc: &selfUseSubFake{}, apiKeyService: key}
	require.NoError(t, svc8.ensureSelfUseAccess(context.Background(), 8, &Account{ID: 43, Platform: PlatformOpenAI}))

	require.Len(t, gr.createdAll, 2, "each contributor needs their own self-use group")
	require.NotEqual(t, gr.createdAll[0].Name, gr.createdAll[1].Name, "self-use group names must be per-user")
	require.NotEqual(t, gr.createdAll[0].ID, gr.createdAll[1].ID)

	// Isolation: each group holds only its own owner's contributed account.
	require.Equal(t, []int64{42}, gr.bound[gr.createdAll[0].ID])
	require.Equal(t, []int64{43}, gr.bound[gr.createdAll[1].ID])
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

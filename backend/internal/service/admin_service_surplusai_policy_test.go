//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type surplusAIPolicyAccountRepoStub struct {
	mockAccountRepoForGemini
	account             *Account
	createCalls         int
	updateCalls         int
	setSchedulableCalls int
}

func (r *surplusAIPolicyAccountRepoStub) GetByID(context.Context, int64) (*Account, error) {
	if r.account == nil {
		return nil, ErrAccountNotFound
	}
	return r.account, nil
}

func (r *surplusAIPolicyAccountRepoStub) Create(context.Context, *Account) error {
	r.createCalls++
	return nil
}

func (r *surplusAIPolicyAccountRepoStub) Update(context.Context, *Account) error {
	r.updateCalls++
	return nil
}

func (r *surplusAIPolicyAccountRepoStub) SetSchedulable(context.Context, int64, bool) error {
	r.setSchedulableCalls++
	return nil
}

func TestAdminServiceCreateAccountAllowsNonOAuthUpstreamAccount(t *testing.T) {
	repo := &surplusAIPolicyAccountRepoStub{}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:     "static key",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		// Skip default-group auto-bind so the stub doesn't need a groupRepo.
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, AccountTypeAPIKey, account.Type)
	require.Equal(t, 1, repo.createCalls)
}

func TestAdminServiceUpdateAccountAllowsChangingOAuthToNonOAuth(t *testing.T) {
	repo := &surplusAIPolicyAccountRepoStub{
		account: &Account{
			ID:       10,
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 10, &UpdateAccountInput{
		Type: AccountTypeAPIKey,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, AccountTypeAPIKey, account.Type)
	require.Equal(t, 1, repo.updateCalls)
}

func TestAdminServiceUpdateAccountRejectsChangingContributedOAuthType(t *testing.T) {
	ownerID := int64(77)
	repo := &surplusAIPolicyAccountRepoStub{
		account: &Account{
			ID:          10,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			OwnerUserID: &ownerID,
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 10, &UpdateAccountInput{Type: AccountTypeAPIKey})

	require.Nil(t, account)
	require.ErrorContains(t, err, "must remain OAuth")
	require.Zero(t, repo.updateCalls)
}

func TestAdminServiceSetAccountSchedulableAllowsNonOAuthAccount(t *testing.T) {
	repo := &surplusAIPolicyAccountRepoStub{
		account: &Account{
			ID:       20,
			Platform: PlatformGemini,
			Type:     AccountTypeAPIKey,
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.SetAccountSchedulable(context.Background(), 20, true)

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, AccountTypeAPIKey, account.Type)
	require.Equal(t, 1, repo.setSchedulableCalls)
}

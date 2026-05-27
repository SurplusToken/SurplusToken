//go:build unit

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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

func TestAdminServiceCreateAccountRejectsNonOAuthUpstreamAccount(t *testing.T) {
	repo := &surplusAIPolicyAccountRepoStub{}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:     "static key",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
	})

	require.Nil(t, account)
	require.Equal(t, "SURPLUSAI_OAUTH_ONLY", infraerrors.Reason(err))
	require.Zero(t, repo.createCalls)
}

func TestAdminServiceUpdateAccountRejectsChangingOAuthToNonOAuth(t *testing.T) {
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

	require.Nil(t, account)
	require.Equal(t, "SURPLUSAI_OAUTH_ONLY", infraerrors.Reason(err))
	require.Zero(t, repo.updateCalls)
}

func TestAdminServiceSetAccountSchedulableRejectsNonOAuthAccount(t *testing.T) {
	repo := &surplusAIPolicyAccountRepoStub{
		account: &Account{
			ID:       20,
			Platform: PlatformGemini,
			Type:     AccountTypeAPIKey,
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.SetAccountSchedulable(context.Background(), 20, true)

	require.Nil(t, account)
	require.Equal(t, "SURPLUSAI_OAUTH_ONLY", infraerrors.Reason(err))
	require.Zero(t, repo.setSchedulableCalls)
}

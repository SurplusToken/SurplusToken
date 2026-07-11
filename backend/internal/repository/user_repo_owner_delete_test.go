package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryDeleteBlocksLivePrimaryAccountOwner(t *testing.T) {
	t.Parallel()

	repo, client := newUserEntRepo(t)
	ctx := context.Background()
	user := &service.User{
		Email:        "account-owner@example.com",
		Username:     "account-owner",
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, user))

	_, err := client.Account.Create().
		SetName("owned-account").
		SetPlatform(service.PlatformOpenAI).
		SetType(service.AccountTypeOAuth).
		SetStatus(service.StatusActive).
		SetCredentials(map[string]any{"access_token": "test"}).
		SetOwnerUserID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	err = repo.Delete(ctx, user.ID)
	require.ErrorIs(t, err, service.ErrUserDeleteBlockedAccountOwnership)
	_, err = repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
}

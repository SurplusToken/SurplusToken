package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountRepositoryCreateRejectsSoftDeletedOwner(t *testing.T) {
	t.Parallel()

	userRepo, client := newUserEntRepo(t)
	ctx := context.Background()
	user := &service.User{
		Email:        "deleted-owner@example.com",
		Username:     "deleted-owner",
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	}
	require.NoError(t, userRepo.Create(ctx, user))
	_, err := client.User.UpdateOneID(user.ID).SetDeletedAt(time.Now()).Save(ctx)
	require.NoError(t, err)

	accountRepo := newAccountRepositoryWithSQL(client, userRepo.sql, nil)
	err = accountRepo.Create(ctx, &service.Account{
		Name:        "orphan-attempt",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeOAuth,
		Status:      service.StatusActive,
		Credentials: map[string]any{"access_token": "test"},
		OwnerUserID: &user.ID,
	})
	require.ErrorIs(t, err, service.ErrUserNotFound)
}

func TestAccountRepositorySetCoOwnersRejectsMissingUser(t *testing.T) {
	t.Parallel()

	userRepo, client := newUserEntRepo(t)
	ctx := context.Background()
	account, err := client.Account.Create().
		SetName("co-owner-target").
		SetPlatform(service.PlatformOpenAI).
		SetType(service.AccountTypeOAuth).
		SetStatus(service.StatusActive).
		SetCredentials(map[string]any{"access_token": "test"}).
		Save(ctx)
	require.NoError(t, err)

	accountRepo := newAccountRepositoryWithSQL(client, userRepo.sql, nil)
	err = accountRepo.SetAccountCoOwners(ctx, account.ID, []int64{999999}, nil)
	require.ErrorIs(t, err, service.ErrUserNotFound)
}

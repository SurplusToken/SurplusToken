package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newContributionPoolRepoMock(t *testing.T) (*accountContributionPoolRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &accountContributionPoolRepository{sql: db}, mock
}

func expectContributionOwnerSetLock(mock sqlmock.Sqlmock, accountID, primaryOwner int64, coOwners ...int64) {
	coOwnerRows := func() *sqlmock.Rows {
		rows := sqlmock.NewRows([]string{"user_id"})
		for _, userID := range coOwners {
			rows.AddRow(userID)
		}
		return rows
	}
	liveUserRows := sqlmock.NewRows([]string{"id"}).AddRow(primaryOwner)
	for _, userID := range coOwners {
		if userID != primaryOwner {
			liveUserRows.AddRow(userID)
		}
	}

	// Candidate read -> live user locks -> account lock -> authoritative re-read.
	mock.ExpectQuery("SELECT owner_user_id").WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id"}).AddRow(primaryOwner))
	mock.ExpectQuery("SELECT user_id").WithArgs(accountID).WillReturnRows(coOwnerRows())
	mock.ExpectQuery("SELECT id").WithArgs(sqlmock.AnyArg()).WillReturnRows(liveUserRows)
	mock.ExpectQuery("SELECT owner_user_id").WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id"}).AddRow(primaryOwner))
	mock.ExpectQuery("SELECT user_id").WithArgs(accountID).WillReturnRows(coOwnerRows())
}

func TestAccountContributionPoolDistributeReplaysSameFingerprint(t *testing.T) {
	t.Parallel()

	repo, mock := newContributionPoolRepoMock(t)
	mock.ExpectBegin()
	expectContributionOwnerSetLock(mock, 42, 7)
	mock.ExpectQuery("SELECT request_fingerprint, total_amount").
		WithArgs(int64(42), "idem-1").
		WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint", "total_amount"}).AddRow("fingerprint", 3.5))
	mock.ExpectCommit()

	result, err := repo.Distribute(context.Background(), service.PoolDistributionRequest{
		AccountID:          42,
		RequesterUserID:    7,
		IdempotencyKey:     "idem-1",
		RequestFingerprint: "fingerprint",
		Mode:               "even",
	})
	require.NoError(t, err)
	require.True(t, result.Replayed)
	require.Equal(t, 3.5, result.TotalAmount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountContributionPoolDistributeRejectsIdempotencyConflict(t *testing.T) {
	t.Parallel()

	repo, mock := newContributionPoolRepoMock(t)
	mock.ExpectBegin()
	expectContributionOwnerSetLock(mock, 42, 7)
	mock.ExpectQuery("SELECT request_fingerprint, total_amount").
		WithArgs(int64(42), "idem-1").
		WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint", "total_amount"}).AddRow("old", 3.5))
	mock.ExpectRollback()

	result, err := repo.Distribute(context.Background(), service.PoolDistributionRequest{
		AccountID:          42,
		RequesterUserID:    7,
		IdempotencyKey:     "idem-1",
		RequestFingerprint: "new",
		Mode:               "even",
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountContributionPoolDistributeCustomIsAtomic(t *testing.T) {
	t.Parallel()

	repo, mock := newContributionPoolRepoMock(t)
	mock.ExpectBegin()
	expectContributionOwnerSetLock(mock, 42, 7, 8)
	mock.ExpectQuery("SELECT request_fingerprint, total_amount").
		WithArgs(int64(42), "idem-1").
		WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint", "total_amount"}))
	mock.ExpectQuery("SELECT COALESCE\\(pool_amount, 0\\)::double precision").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"pool_amount"}).AddRow(1.0))
	mock.ExpectQuery("UPDATE account_contribution_pools").
		WithArgs(1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"pool_amount"}).AddRow(0.0))
	mock.ExpectQuery("INSERT INTO account_contribution_pool_distributions").
		WithArgs(int64(42), "idem-1", "fingerprint", "custom", 1.0, sqlmock.AnyArg(), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))

	for _, allocation := range []service.PoolAllocation{{UserID: 7, Amount: 0.6}, {UserID: 8, Amount: 0.4}} {
		mock.ExpectExec("INSERT INTO user_contributions").
			WithArgs(allocation.UserID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("UPDATE user_contributions").
			WithArgs(allocation.Amount, allocation.UserID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("INSERT INTO user_contribution_ledger").
			WithArgs(allocation.UserID, allocation.Amount, int64(42)).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}
	mock.ExpectExec("INSERT INTO account_contribution_pool_ledger").
		WithArgs(int64(42), 1.0, int64(99)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := repo.Distribute(context.Background(), service.PoolDistributionRequest{
		AccountID:          42,
		RequesterUserID:    7,
		IdempotencyKey:     "idem-1",
		RequestFingerprint: "fingerprint",
		Mode:               "custom",
		Allocations: []service.PoolAllocation{
			{UserID: 7, Amount: 0.6},
			{UserID: 8, Amount: 0.4},
		},
	})
	require.NoError(t, err)
	require.False(t, result.Replayed)
	require.Equal(t, 1.0, result.TotalAmount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountContributionPoolDistributeRejectsCurrentNonOwner(t *testing.T) {
	t.Parallel()

	repo, mock := newContributionPoolRepoMock(t)
	mock.ExpectBegin()
	expectContributionOwnerSetLock(mock, 42, 7)
	mock.ExpectQuery("SELECT request_fingerprint, total_amount").
		WithArgs(int64(42), "idem-1").
		WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint", "total_amount"}))
	mock.ExpectQuery("SELECT COALESCE\\(pool_amount, 0\\)::double precision").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"pool_amount"}).AddRow(2.0))
	mock.ExpectRollback()

	result, err := repo.Distribute(context.Background(), service.PoolDistributionRequest{
		AccountID:          42,
		RequesterUserID:    7,
		IdempotencyKey:     "idem-1",
		RequestFingerprint: "fingerprint",
		Mode:               "custom",
		Allocations:        []service.PoolAllocation{{UserID: 8, Amount: 1}},
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, service.ErrContributionBadRecipient)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountContributionPoolDistributeRejectsOwnerSetChangedWhileLocking(t *testing.T) {
	t.Parallel()

	repo, mock := newContributionPoolRepoMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT owner_user_id").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id"}).AddRow(int64(7)))
	mock.ExpectQuery("SELECT user_id").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(8)))
	mock.ExpectQuery("SELECT id").WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)).AddRow(int64(8)))
	mock.ExpectQuery("SELECT owner_user_id").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id"}).AddRow(int64(7)))
	mock.ExpectQuery("SELECT user_id").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(9)))
	mock.ExpectRollback()

	result, err := repo.Distribute(context.Background(), service.PoolDistributionRequest{
		AccountID:          42,
		RequesterUserID:    7,
		IdempotencyKey:     "idem-owner-change",
		RequestFingerprint: "fingerprint",
		Mode:               "even",
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, service.ErrContributionOwnerSetChanged)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountContributionPoolDistributeCreditFailureRollsBackDebit(t *testing.T) {
	t.Parallel()

	repo, mock := newContributionPoolRepoMock(t)
	mock.ExpectBegin()
	expectContributionOwnerSetLock(mock, 42, 7)
	mock.ExpectQuery("SELECT request_fingerprint, total_amount").
		WithArgs(int64(42), "idem-credit-failure").
		WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint", "total_amount"}))
	mock.ExpectQuery("SELECT COALESCE\\(pool_amount, 0\\)::double precision").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"pool_amount"}).AddRow(1.0))
	mock.ExpectQuery("UPDATE account_contribution_pools").
		WithArgs(1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"pool_amount"}).AddRow(0.0))
	mock.ExpectQuery("INSERT INTO account_contribution_pool_distributions").
		WithArgs(int64(42), "idem-credit-failure", "fingerprint", "custom", 1.0, sqlmock.AnyArg(), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))
	mock.ExpectExec("INSERT INTO user_contributions").WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	creditErr := errors.New("credit update failed")
	mock.ExpectExec("UPDATE user_contributions").WithArgs(1.0, int64(7)).WillReturnError(creditErr)
	mock.ExpectRollback()

	result, err := repo.Distribute(context.Background(), service.PoolDistributionRequest{
		AccountID:          42,
		RequesterUserID:    7,
		IdempotencyKey:     "idem-credit-failure",
		RequestFingerprint: "fingerprint",
		Mode:               "custom",
		Allocations:        []service.PoolAllocation{{UserID: 7, Amount: 1}},
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, creditErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSplitContributionPoolEvenlyDistributesSmallestUnit(t *testing.T) {
	t.Parallel()

	allocations := splitContributionPoolEvenly(0.00000001, []int64{7, 8})
	require.Equal(t, []service.PoolAllocation{{UserID: 7, Amount: 0.00000001}}, allocations)
}

func TestSplitContributionPoolEvenlyPreservesAllUnits(t *testing.T) {
	t.Parallel()

	allocations := splitContributionPoolEvenly(1, []int64{7, 8, 9})
	require.Equal(t, []service.PoolAllocation{
		{UserID: 7, Amount: 0.33333334},
		{UserID: 8, Amount: 0.33333333},
		{UserID: 9, Amount: 0.33333333},
	}, allocations)
}

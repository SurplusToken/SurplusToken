package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func newContributionWithdrawalRepoClient(t *testing.T) (*dbent.Client, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}

func TestRefundContributionWithdrawalReturnsReservedAmountAndAuditsCancellation(t *testing.T) {
	client, mock := newContributionWithdrawalRepoClient(t)

	mock.ExpectQuery("SELECT contribution_quota::double precision").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"contribution_quota",
			"contribution_frozen_quota",
			"contribution_history_quota",
		}).AddRow(2.5, 1.0, 20.0))
	mock.ExpectExec("UPDATE user_contributions").
		WithArgs(3.25, int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE contribution_withdrawals").
		WithArgs(int64(41)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO user_contribution_ledger").
		WithArgs(int64(9), 3.25, int64(41), 5.75, 1.0, 20.0).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := refundContributionWithdrawalTx(
		context.Background(),
		client,
		41,
		9,
		3.25,
		0,
		service.ContributionWithdrawalStatusCancelled,
		"",
		"",
	)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefundContributionWithdrawalRecordsAdminRejection(t *testing.T) {
	client, mock := newContributionWithdrawalRepoClient(t)

	mock.ExpectQuery("SELECT contribution_quota::double precision").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"contribution_quota",
			"contribution_frozen_quota",
			"contribution_history_quota",
		}).AddRow(0.0, 0.5, 10.0))
	mock.ExpectExec("UPDATE user_contributions").
		WithArgs(1.5, int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE contribution_withdrawals").
		WithArgs("invalid payout account", "", int64(2), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO user_contribution_ledger").
		WithArgs(int64(9), 1.5, int64(42), 1.5, 0.5, 10.0).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := refundContributionWithdrawalTx(
		context.Background(),
		client,
		42,
		9,
		1.5,
		2,
		service.ContributionWithdrawalStatusRejected,
		"invalid payout account",
		"",
	)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

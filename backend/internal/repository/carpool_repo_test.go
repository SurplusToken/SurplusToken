package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLaunchCarpoolCreatesUnlimitedOpenAIGroupAndSubscriptions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT name, description, owner_user_id FROM carpools WHERE id = $1")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"name", "description", "owner_user_id"}).AddRow("weekend-car", "test", int64(11)))
	mock.ExpectQuery("INSERT INTO groups").
		WithArgs("weekend-car", "Carpool subscription: test").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(91)))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventGroupChanged, nil, sqlmock.AnyArg(), nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id, user_id FROM carpool_members").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(int64(21), int64(11)).AddRow(int64(22), int64(12)))
	for i, userID := range []int64{11, 12} {
		mock.ExpectQuery("INSERT INTO user_subscriptions").
			WithArgs(userID, int64(91), sqlmock.AnyArg(), sqlmock.AnyArg(), sql.NullInt64{Int64: 11, Valid: true}, "Automatically assigned when carpool became full").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(101 + i)))
		mock.ExpectExec("UPDATE carpool_members SET status = 'active'").
			WithArgs(int64(21+i), int64(101+i), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec("UPDATE carpools SET status = 'active'").
		WithArgs(int64(7), int64(91), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE carpool_invites SET revoked_at").
		WithArgs(int64(7), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), sql.NullInt64{Int64: 11, Valid: true}, "launched").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	groupID, userIDs, err := launchCarpool(context.Background(), tx, 7)
	require.NoError(t, err)
	require.Equal(t, int64(91), groupID)
	require.Equal(t, []int64{11, 12}, userIDs)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

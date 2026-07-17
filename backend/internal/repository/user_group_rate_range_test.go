package repository

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUserGroupRateRepository_SharingRateRangesAreScopedByGroup(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT group_id, sharing_rate_min, sharing_rate_max
		FROM user_group_rate_multipliers
		WHERE user_id = $1
	`)).WithArgs(int64(7)).WillReturnRows(
		sqlmock.NewRows([]string{"group_id", "sharing_rate_min", "sharing_rate_max"}).
			AddRow(int64(11), 0.8, 1.2).
			AddRow(int64(22), 1.4, nil),
	)

	repo := &userGroupRateRepository{sql: db}
	ranges, err := repo.GetSharingRateRangesByUser(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, 0.8, *ranges[11].Min)
	require.Equal(t, 1.2, *ranges[11].Max)
	require.Equal(t, 1.4, *ranges[22].Min)
	require.Nil(t, ranges[22].Max)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserGroupRateRepository_UpdateSharingRateRangeTargetsOneGroup(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	min, max := 0.75, 1.35
	mock.ExpectExec("INSERT INTO user_group_rate_multipliers").
		WithArgs(int64(7), int64(22), min, max).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &userGroupRateRepository{sql: db}
	require.NoError(t, repo.UpdateSharingRateRangeByUserAndGroup(context.Background(), 7, 22, &min, &max))
	require.NoError(t, mock.ExpectationsWereMet())
}

package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

var testLaunchParams = carpoolLaunchParams{
	weeklyLimitUSD: 2400,
	seatFeeCNY:     400,
	usagePoolCNY:   1000,
	reserveRatio:   0.8,
}

// 开车时：group 写周限额安全帽 2400，成员订阅写 0.8×申报 + C，预付按发车人数锁定。
func TestLaunchCarpoolCreatesLimitedGroupAndPerMemberSubscriptions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// 2 名成员各申报 $1200（Σ=2400）：公共池 C = 2400 − 0.8×2400 = 480，
	// 每人订阅周限额 = 0.8×1200 + 480 = 1440；预付 = 400/2 + 1000×1200/2400 = 700。
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT name, description, owner_user_id FROM carpools WHERE id = $1")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"name", "description", "owner_user_id"}).AddRow("weekend-car", "test", int64(11)))
	mock.ExpectQuery("INSERT INTO groups").
		WithArgs("weekend-car", "Carpool subscription: test", 2400.0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(91)))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventGroupChanged, nil, sqlmock.AnyArg(), nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id, user_id, declared_weekly_quota_usd FROM carpool_members").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "declared_weekly_quota_usd"}).
			AddRow(int64(21), int64(11), 1200.0).
			AddRow(int64(22), int64(12), 1200.0))
	for i, userID := range []int64{11, 12} {
		mock.ExpectQuery("INSERT INTO user_subscriptions").
			WithArgs(userID, int64(91), sqlmock.AnyArg(), sqlmock.AnyArg(), sql.NullInt64{Int64: 11, Valid: true},
				"Automatically assigned when carpool launched", 1440.0).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(101 + i)))
		mock.ExpectExec("UPDATE carpool_members SET status = 'active'").
			WithArgs(int64(21+i), int64(101+i), 700.0, sqlmock.AnyArg()).
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
	groupID, userIDs, err := launchCarpool(context.Background(), tx, 7, testLaunchParams)
	require.NoError(t, err)
	require.Equal(t, int64(91), groupID)
	require.Equal(t, []int64{11, 12}, userIDs)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func carpoolDetailRow() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "name", "description", "organizer", "owner_user_id", "platform", "plan_type",
		"car_type", "level", "capacity", "member_count", "base_fee_cny", "usage_pool_cny_per_account",
		"visibility", "status", "join_locked", "scheduled_start_at", "launched_at",
		"group_id", "group_name", "member_role", "created_at",
		"weekly_limit_usd", "seat_fee_cny", "usage_pool_cny", "reserve_ratio",
		"launch_min_ratio", "launch_max_ratio", "declared_total",
	}).AddRow(
		int64(7), "weekend-car", "test", "owner", int64(11), "openai", "openai_pro",
		"small", 1, nil, 9, 130.0, 750.0,
		"public", "recruiting", false, nil, nil,
		nil, nil, "member", time.Now(),
		2400.0, 400.0, 1000.0, 0.8,
		0.95, 1.05, 2250.0,
	)
}

// 上车成功：申报写入成员记录，预付按当前人数记账（400/9 + 1000×250/2400 ≈ 148.61）。
func TestJoinCarpoolRecordsDeclarationAndPrepaid(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	prepaid := service.CarpoolPrepaidCNY(400, 1000, 2400, 250, 9)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, visibility, join_locked_at IS NOT NULL")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "visibility", "locked", "weekly_limit_usd", "launch_max_ratio", "seat_fee_cny", "usage_pool_cny"}).
			AddRow("recruiting", "public", false, 2400.0, 1.05, 400.0, 1000.0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(2000.0, 8))
	mock.ExpectExec("INSERT INTO carpool_members").
		WithArgs(int64(7), int64(55), nil, 250.0, prepaid).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(55), "member_joined").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE c.id = $2")).
		WithArgs(int64(55), int64(7)).
		WillReturnRows(carpoolDetailRow())
	mock.ExpectCommit()

	result, err := repo.Join(context.Background(), 7, 55, 250, nil)
	require.NoError(t, err)
	require.Equal(t, 250.0, result.DeclaredWeeklyQuotaUSD)
	require.InDelta(t, 148.61, result.PrepaidAmountCNY, 0.01)
	require.NotNil(t, result.Carpool)
	require.InDelta(t, 2250, result.Carpool.DeclaredTotalUSD, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 上车硬上限（§4.1）：Σ申报 2480 + 新申报 100 > 2520（105%×2400）→ 拒绝上车。
func TestJoinCarpoolRejectsDeclarationOverHardCap(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, visibility, join_locked_at IS NOT NULL")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "visibility", "locked", "weekly_limit_usd", "launch_max_ratio", "seat_fee_cny", "usage_pool_cny"}).
			AddRow("recruiting", "public", false, 2400.0, 1.05, 400.0, 1000.0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(2480.0, 10))
	mock.ExpectRollback()

	_, err = repo.Join(context.Background(), 7, 55, 100, nil)
	require.ErrorIs(t, err, service.ErrCarpoolQuotaExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 手动发车：Σ申报 2000 < 2280（95% 下限）→ 不可发车；force 降档（≥80%）则可发车。
func TestLaunchCarpoolEnforcesDeclarationBand(t *testing.T) {
	newLaunchMock := func(t *testing.T, declaredTotal float64) (service.CarpoolRepository, sqlmock.Sqlmock, func()) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status, weekly_limit_usd, seat_fee_cny, usage_pool_cny,")).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "weekly_limit_usd", "seat_fee_cny", "usage_pool_cny", "reserve_ratio", "launch_min_ratio", "launch_max_ratio"}).
				AddRow(int64(11), "recruiting", 2400.0, 400.0, 1000.0, 0.8, 0.95, 1.05))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(declaredTotal, 8))
		return NewCarpoolRepository(db), mock, func() { _ = db.Close() }
	}

	// Σ=2000（83%）：正常发车被拒
	repo, mock, cleanup := newLaunchMock(t, 2000)
	defer cleanup()
	mock.ExpectRollback()
	_, err := repo.Launch(context.Background(), 7, 11, false, false)
	require.ErrorIs(t, err, service.ErrCarpoolLaunchNotReady)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 非 owner 非 admin 不能发车。
func TestLaunchCarpoolForbiddenForNonOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status, weekly_limit_usd, seat_fee_cny, usage_pool_cny,")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "weekly_limit_usd", "seat_fee_cny", "usage_pool_cny", "reserve_ratio", "launch_min_ratio", "launch_max_ratio"}).
			AddRow(int64(11), "recruiting", 2400.0, 400.0, 1000.0, 0.8, 0.95, 1.05))
	mock.ExpectRollback()

	_, err = repo.Launch(context.Background(), 7, 12, false, false)
	require.ErrorIs(t, err, service.ErrCarpoolForbidden)
	require.NoError(t, mock.ExpectationsWereMet())
}

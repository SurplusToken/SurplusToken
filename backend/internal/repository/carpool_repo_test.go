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

// 开车时：group 写周限额安全帽 2400，成员订阅写保底 r=0.8×申报（weekly_reserved_usd）
// 与个人上限 r+C（weekly_limit_usd），周窗口起点全车对齐，预付按发车人数锁定。
func TestLaunchCarpoolCreatesLimitedGroupAndPerMemberSubscriptions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// 2 名成员各申报 $1200（Σ=2400）：保底 r = 0.8×1200 = 960，公共池 C = 2400 − 0.8×2400 = 480，
	// 每人订阅周限额 = 960 + 480 = 1440；预付 = 400/2 + 1000×1200/2400 = 700。
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
				"Automatically assigned when carpool launched", 1440.0, 960.0, sqlmock.AnyArg()).
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
		"launch_notified_at", "confirmed_at", "has_group_qr_code",
		"settled_at", "settled_by_user_id",
		"pricing_model", "rule_note",
	}).AddRow(
		int64(7), "weekend-car", "test", "owner", int64(11), "openai", "openai_pro",
		"small", 1, nil, 9, 130.0, 750.0,
		"public", "recruiting", false, nil, nil,
		nil, nil, "member", time.Now(),
		2400.0, 400.0, 1000.0, 0.8,
		0.95, 1.05, 2250.0,
		nil, nil, true,
		nil, nil,
		"quota", "",
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
		WillReturnRows(sqlmock.NewRows([]string{"status", "visibility", "locked", "pricing_model", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio", "seat_fee_cny", "usage_pool_cny"}).
			AddRow("recruiting", "public", false, "quota", 2400.0, 0.95, 1.05, 400.0, 1000.0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(2000.0, 8))
	mock.ExpectExec("INSERT INTO carpool_members").
		WithArgs(int64(7), int64(55), nil, 250.0, prepaid, true).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(55), "member_joined").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Σ=2250 未进区间 [2280, 2520]：不置 launch_notified_at。
	mock.ExpectQuery(regexp.QuoteMeta("WHERE c.id = $2")).
		WithArgs(int64(55), int64(7)).
		WillReturnRows(carpoolDetailRow())
	mock.ExpectCommit()

	result, err := repo.Join(context.Background(), 7, 55, 250, true, nil)
	require.NoError(t, err)
	require.Equal(t, 250.0, result.DeclaredWeeklyQuotaUSD)
	require.InDelta(t, 148.61, result.PrepaidAmountCNY, 0.01)
	require.False(t, result.LaunchBandEntered)
	require.NotNil(t, result.Carpool)
	require.InDelta(t, 2250, result.Carpool.DeclaredTotalUSD, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 进区间通知：Σ申报 进入 [95%, 105%]×周限额 时同事务置 launch_notified_at。
func TestJoinCarpoolEnteringLaunchBandMarksNotified(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	prepaid := service.CarpoolPrepaidCNY(400, 1000, 2400, 300, 9)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, visibility, join_locked_at IS NOT NULL")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "visibility", "locked", "pricing_model", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio", "seat_fee_cny", "usage_pool_cny"}).
			AddRow("recruiting", "public", false, "quota", 2400.0, 0.95, 1.05, 400.0, 1000.0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(2050.0, 8))
	mock.ExpectExec("INSERT INTO carpool_members").
		WithArgs(int64(7), int64(55), nil, 300.0, prepaid, true).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(55), "member_joined").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Σ=2350 ∈ [2280, 2520]：置 launch_notified_at（首次进入，影响 1 行）。
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpools SET launch_notified_at = NOW(), updated_at = NOW()")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE c.id = $2")).
		WithArgs(int64(55), int64(7)).
		WillReturnRows(carpoolDetailRow())
	mock.ExpectCommit()

	result, err := repo.Join(context.Background(), 7, 55, 300, true, nil)
	require.NoError(t, err)
	require.True(t, result.LaunchBandEntered)
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
		WillReturnRows(sqlmock.NewRows([]string{"status", "visibility", "locked", "pricing_model", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio", "seat_fee_cny", "usage_pool_cny"}).
			AddRow("recruiting", "public", false, "quota", 2400.0, 0.95, 1.05, 400.0, 1000.0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(2480.0, 10))
	mock.ExpectRollback()

	_, err = repo.Join(context.Background(), 7, 55, 100, true, nil)
	require.ErrorIs(t, err, service.ErrCarpoolQuotaExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 发车区间校验：confirmed 车正常发车 Σ<95% 下限 → 不可发车；force 降档 Σ<80% → 不可发车。
func TestLaunchCarpoolEnforcesDeclarationBand(t *testing.T) {
	newLaunchMock := func(t *testing.T, status string, declaredTotal float64) (service.CarpoolRepository, sqlmock.Sqlmock, func()) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT status, weekly_limit_usd, seat_fee_cny, usage_pool_cny,")).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"status", "weekly_limit_usd", "seat_fee_cny", "usage_pool_cny", "reserve_ratio", "launch_min_ratio", "launch_max_ratio"}).
				AddRow(status, 2400.0, 400.0, 1000.0, 0.8, 0.95, 1.05))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(declaredTotal, 8))
		return NewCarpoolRepository(db), mock, func() { _ = db.Close() }
	}

	// 正常发车（confirmed）：Σ=2000（83%）< 2280（95% 下限）→ 不可发车
	repo, mock, cleanup := newLaunchMock(t, "confirmed", 2000)
	defer cleanup()
	mock.ExpectRollback()
	_, err := repo.Launch(context.Background(), 7, 99, true, false)
	require.ErrorIs(t, err, service.ErrCarpoolLaunchNotReady)
	require.NoError(t, mock.ExpectationsWereMet())

	// force 降档发车（recruiting）：Σ=1800（75%）< 1920（80% 下限）→ 不可发车
	repo2, mock2, cleanup2 := newLaunchMock(t, "recruiting", 1800)
	defer cleanup2()
	mock2.ExpectRollback()
	_, err = repo2.Launch(context.Background(), 7, 99, true, true)
	require.ErrorIs(t, err, service.ErrCarpoolLaunchNotReady)
	require.NoError(t, mock2.ExpectationsWereMet())
}

// 发车仅 admin：上一轮 owner 可直接 launch 的行为已被两段确认取代，owner 非 admin 也 403。
func TestLaunchCarpoolForbiddenForNonAdmin(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, weekly_limit_usd, seat_fee_cny, usage_pool_cny,")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "weekly_limit_usd", "seat_fee_cny", "usage_pool_cny", "reserve_ratio", "launch_min_ratio", "launch_max_ratio"}).
			AddRow("confirmed", 2400.0, 400.0, 1000.0, 0.8, 0.95, 1.05))
	mock.ExpectRollback()

	// actorUserID=11 即 owner，仍被拒绝（仅 admin 可发车）
	_, err = repo.Launch(context.Background(), 7, 11, false, false)
	require.ErrorIs(t, err, service.ErrCarpoolForbidden)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 正常发车要求车已 confirmed；recruiting 车正常发车 → 409 CARPOOL_NOT_CONFIRMED。
func TestLaunchCarpoolRequiresConfirmedStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, weekly_limit_usd, seat_fee_cny, usage_pool_cny,")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "weekly_limit_usd", "seat_fee_cny", "usage_pool_cny", "reserve_ratio", "launch_min_ratio", "launch_max_ratio"}).
			AddRow("recruiting", 2400.0, 400.0, 1000.0, 0.8, 0.95, 1.05))
	mock.ExpectRollback()

	_, err = repo.Launch(context.Background(), 7, 99, true, false)
	require.ErrorIs(t, err, service.ErrCarpoolNotConfirmed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// force 降档发车要求 recruiting；confirmed 车走 force → 409。
func TestLaunchCarpoolForceRequiresRecruitingStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, weekly_limit_usd, seat_fee_cny, usage_pool_cny,")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "weekly_limit_usd", "seat_fee_cny", "usage_pool_cny", "reserve_ratio", "launch_min_ratio", "launch_max_ratio"}).
			AddRow("confirmed", 2400.0, 400.0, 1000.0, 0.8, 0.95, 1.05))
	mock.ExpectRollback()

	_, err = repo.Launch(context.Background(), 7, 99, true, true)
	require.ErrorIs(t, err, service.ErrCarpoolUnavailable)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 下车成功：成员行置 left 并记事件；Σ 跌出区间时重置 launch_notified_at。
func TestLeaveCarpoolReleasesQuotaAndResetsLaunchNotification(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status, weekly_limit_usd, launch_min_ratio, launch_max_ratio, launch_notified_at")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio", "launch_notified_at"}).
			AddRow(int64(11), "recruiting", 2400.0, 0.95, 1.05, time.Now()))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("joined"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpool_members SET status = 'left', left_at = NOW(), updated_at = NOW()")).
		WithArgs(int64(7), int64(55)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(55), "member_left").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// 下车后 Σ=2050 < 2280（95% 下限）→ 重置 launch_notified_at。
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2050.0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpools SET launch_notified_at = NULL, updated_at = NOW()")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE c.id = $2")).
		WithArgs(int64(55), int64(7)).
		WillReturnRows(carpoolDetailRow())
	mock.ExpectCommit()

	result, err := repo.Leave(context.Background(), 7, 55)
	require.NoError(t, err)
	require.NotNil(t, result.Carpool)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 下车幂等：已 left 的成员重复下车返回同样的成功结果（无额外写入）。
func TestLeaveCarpoolIdempotentWhenAlreadyLeft(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status, weekly_limit_usd, launch_min_ratio, launch_max_ratio, launch_notified_at")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio", "launch_notified_at"}).
			AddRow(int64(11), "recruiting", 2400.0, 0.95, 1.05, nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("left"))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE c.id = $2")).
		WithArgs(int64(55), int64(7)).
		WillReturnRows(carpoolDetailRow())
	mock.ExpectCommit()

	result, err := repo.Leave(context.Background(), 7, 55)
	require.NoError(t, err)
	require.NotNil(t, result.Carpool)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 车主下车 → 409（只能取消整车）。
func TestLeaveCarpoolRejectsOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status, weekly_limit_usd, launch_min_ratio, launch_max_ratio, launch_notified_at")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio", "launch_notified_at"}).
			AddRow(int64(11), "recruiting", 2400.0, 0.95, 1.05, nil))
	mock.ExpectRollback()

	_, err = repo.Leave(context.Background(), 7, 11)
	require.ErrorIs(t, err, service.ErrCarpoolOwnerCannotLeave)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 非 recruiting（如 confirmed）下车 → 409。
func TestLeaveCarpoolRejectsConfirmedCarpool(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status, weekly_limit_usd, launch_min_ratio, launch_max_ratio, launch_notified_at")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio", "launch_notified_at"}).
			AddRow(int64(11), "confirmed", 2400.0, 0.95, 1.05, time.Now()))
	mock.ExpectRollback()

	_, err = repo.Leave(context.Background(), 7, 55)
	require.ErrorIs(t, err, service.ErrCarpoolUnavailable)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 非成员下车 → 404。
func TestLeaveCarpoolRejectsNonMember(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status, weekly_limit_usd, launch_min_ratio, launch_max_ratio, launch_notified_at")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio", "launch_notified_at"}).
			AddRow(int64(11), "recruiting", 2400.0, 0.95, 1.05, nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = repo.Leave(context.Background(), 7, 55)
	require.ErrorIs(t, err, service.ErrCarpoolNotMember)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 车主确认成功：recruiting + Σ 在区间内 → status=confirmed 并记 confirmed_by/事件。
func TestConfirmCarpoolSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status, weekly_limit_usd, launch_min_ratio, launch_max_ratio")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio"}).
			AddRow(int64(11), "recruiting", 2400.0, 0.95, 1.05))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(2350.0, 9))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpools SET status = 'confirmed', confirmed_at = NOW(), confirmed_by = $2,")).
		WithArgs(int64(7), int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(11), "confirmed").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE c.id = $2")).
		WithArgs(int64(11), int64(7)).
		WillReturnRows(carpoolDetailRow())
	mock.ExpectCommit()

	result, err := repo.Confirm(context.Background(), 7, 11)
	require.NoError(t, err)
	require.NotNil(t, result.Carpool)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 确认仅 owner；其他成员 → 403。
func TestConfirmCarpoolForbiddenForNonOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status, weekly_limit_usd, launch_min_ratio, launch_max_ratio")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio"}).
			AddRow(int64(11), "recruiting", 2400.0, 0.95, 1.05))
	mock.ExpectRollback()

	_, err = repo.Confirm(context.Background(), 7, 55)
	require.ErrorIs(t, err, service.ErrCarpoolForbidden)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 确认要求 Σ 在发车区间内；区间外 → 409 CARPOOL_LAUNCH_NOT_READY。
func TestConfirmCarpoolRequiresDeclarationBand(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status, weekly_limit_usd, launch_min_ratio, launch_max_ratio")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio"}).
			AddRow(int64(11), "recruiting", 2400.0, 0.95, 1.05))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(2000.0, 8))
	mock.ExpectRollback()

	_, err = repo.Confirm(context.Background(), 7, 11)
	require.ErrorIs(t, err, service.ErrCarpoolLaunchNotReady)
	require.NoError(t, mock.ExpectationsWereMet())
}

// confirmed 全锁：owner 取消 confirmed 车 → 409；admin 可强制取消。
func TestCancelConfirmedCarpoolRequiresAdmin(t *testing.T) {
	newCancelMock := func(t *testing.T) (service.CarpoolRepository, sqlmock.Sqlmock, func()) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status FROM carpools WHERE id = $1 FOR UPDATE")).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status"}).AddRow(int64(11), "confirmed"))
		return NewCarpoolRepository(db), mock, func() { _ = db.Close() }
	}

	// owner 非 admin → 409
	repo, mock, cleanup := newCancelMock(t)
	defer cleanup()
	mock.ExpectRollback()
	err := repo.Cancel(context.Background(), 7, 11, false)
	require.ErrorIs(t, err, service.ErrCarpoolUnavailable)
	require.NoError(t, mock.ExpectationsWereMet())

	// admin → 成功取消
	repo2, mock2, cleanup2 := newCancelMock(t)
	defer cleanup2()
	mock2.ExpectExec(regexp.QuoteMeta("UPDATE carpools SET status = 'cancelled'")).
		WithArgs(int64(7), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock2.ExpectExec(regexp.QuoteMeta("UPDATE user_subscriptions SET deleted_at = NOW()")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock2.ExpectExec(regexp.QuoteMeta("UPDATE carpool_members SET status = 'cancelled'")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock2.ExpectExec(regexp.QuoteMeta("UPDATE carpool_invites SET revoked_at")).
		WithArgs(int64(7), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock2.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(99), "cancelled").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock2.ExpectCommit()
	err = repo2.Cancel(context.Background(), 7, 99, true)
	require.NoError(t, err)
	require.NoError(t, mock2.ExpectationsWereMet())
}

// 创建车辆：确认标记与群二维码随 INSERT 落库。
func TestCreateCarpoolStoresQRCodeAndConfirmation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	qrBytes := []byte{0x89, 0x50, 0x4E, 0x47}
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	input := service.CreateCarpoolInput{
		Name: "weekend-car", Visibility: "public", ScheduledStartAt: &start,
		CarType: "small", Level: 1,
		WeeklyLimitUSD: 2400, SeatFeeCNY: 400, UsagePoolCNY: 1000,
		ReserveRatio: 0.8, LaunchMinRatio: 0.95, LaunchMaxRatio: 1.05,
		AddedAdminWechat: true, GroupQRCodeBytes: qrBytes, GroupQRCodeContentType: "image/png",
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM groups WHERE name = $1 AND deleted_at IS NULL)")).
		WithArgs("weekend-car").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("INSERT INTO carpools").
		WithArgs("weekend-car", "", int64(11), "small", 1, "public", &start,
			2400.0, 400.0, 1000.0, 0.8, 0.95, 1.05, true, qrBytes, "image/png").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectExec("INSERT INTO carpool_members").
		WithArgs(int64(7), int64(11), 0.0, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO carpool_invites").
		WithArgs(int64(7), int64(11), "hash", "hint").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(11), "created").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE c.id = $2")).
		WithArgs(int64(11), int64(7)).
		WillReturnRows(carpoolDetailRow())
	mock.ExpectCommit()

	result, err := repo.Create(context.Background(), 11, input, "hash", "hint")
	require.NoError(t, err)
	require.NotNil(t, result.Carpool)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 群二维码读取：命中返回字节与 content-type；车不存在 404；无二维码 404。
func TestGetGroupQRCode(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		repo := NewCarpoolRepository(db)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT group_qr_code, group_qr_code_content_type FROM carpools WHERE id = $1")).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"group_qr_code", "group_qr_code_content_type"}).
				AddRow([]byte{1, 2, 3}, "image/png"))
		data, contentType, err := repo.GetGroupQRCode(context.Background(), 7)
		require.NoError(t, err)
		require.Equal(t, []byte{1, 2, 3}, data)
		require.Equal(t, "image/png", contentType)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("carpool missing", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		repo := NewCarpoolRepository(db)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT group_qr_code, group_qr_code_content_type FROM carpools WHERE id = $1")).
			WithArgs(int64(7)).
			WillReturnError(sql.ErrNoRows)
		_, _, err = repo.GetGroupQRCode(context.Background(), 7)
		require.ErrorIs(t, err, service.ErrCarpoolNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("qr code missing", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		repo := NewCarpoolRepository(db)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT group_qr_code, group_qr_code_content_type FROM carpools WHERE id = $1")).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"group_qr_code", "group_qr_code_content_type"}).AddRow(nil, nil))
		_, _, err = repo.GetGroupQRCode(context.Background(), 7)
		require.ErrorIs(t, err, service.ErrCarpoolQRCodeNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

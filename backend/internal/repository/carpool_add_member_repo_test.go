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

// expectAddMemberLoad 是代加成员（quota 分支）的锁车加载期望。
func expectAddMemberLoad(mock sqlmock.Sqlmock, status, pricingModel string, carType int) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, COALESCE(pricing_model, 'quota'), weekly_limit_usd,")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "pricing_model", "weekly_limit_usd",
			"launch_min_ratio", "launch_max_ratio", "seat_fee_cny", "usage_pool_cny", "car_type"}).
			AddRow(status, pricingModel, 2400.0, 0.95, 1.05, 50.0, 1200.0, carType))
}

// 代加成员（type 3）：申报写入成员行，quoted 按新计价口径落库
// （50 席位每人固定 + 0.8×1200×申报/整车周限额 = 50 + 100 = 150），
// Σ 首次进入发车区间时同事务置 launch_notified_at。
func TestCarpoolAddMemberRecordsDeclarationAndQuotedPrepaid(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	prepaid := service.CarpoolPrepaidCNY(service.CarpoolCarTypeQuotaV2, 50, 1200, 2400, 2500, 250, 10)
	require.InDelta(t, 150.0, prepaid, 1e-9)

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectAddMemberLoad(mock, "recruiting", "quota", service.CarpoolCarTypeQuotaV2)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(2250.0, 9))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO carpool_members (carpool_id, user_id, role, status,")).
		WithArgs(int64(7), int64(55), 250.0, prepaid, true).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(99), "member_added").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Σ=2500 ∈ [2280, 2520] → 进区间通知。
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpools SET launch_notified_at = NOW(), updated_at = NOW()")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// reconcile 复核：Σ=2500 仍在区间内 → 不写任何行。
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2500.0))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE c.id = $2")).
		WithArgs(int64(99), int64(7)).
		WillReturnRows(carpoolDetailRowWithCarType(service.CarpoolCarTypeQuotaV2))
	mock.ExpectCommit()

	result, err := repo.AddMember(context.Background(), 7, 99, service.AddCarpoolMemberInput{
		UserID: 55, DeclaredWeeklyQuotaUSD: 250, AcknowledgedRisk: true,
	})
	require.NoError(t, err)
	require.InDelta(t, 250.0, result.DeclaredWeeklyQuotaUSD, 1e-9)
	require.InDelta(t, prepaid, result.PrepaidAmountCNY, 1e-9)
	require.True(t, result.LaunchBandEntered)
	require.False(t, result.AutoUnconfirmed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 代加的全车硬上限与上车一致：Σ + 新申报 > 105%×周限额 时拒绝。
func TestCarpoolAddMemberEnforcesHardCap(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectAddMemberLoad(mock, "recruiting", "quota", service.CarpoolCarTypeQuotaV2)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(2450.0, 9))
	mock.ExpectRollback()

	_, err = repo.AddMember(context.Background(), 7, 99, service.AddCarpoolMemberInput{UserID: 55, DeclaredWeeklyQuotaUSD: 100})
	require.ErrorIs(t, err, service.ErrCarpoolQuotaExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 重复成员（status joined/active）拒绝。
func TestCarpoolAddMemberRejectsDuplicateMember(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectAddMemberLoad(mock, "recruiting", "quota", service.CarpoolCarTypeQuotaV2)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("joined"))
	mock.ExpectRollback()

	_, err = repo.AddMember(context.Background(), 7, 99, service.AddCarpoolMemberInput{UserID: 55, DeclaredWeeklyQuotaUSD: 100})
	require.ErrorIs(t, err, service.ErrCarpoolAlreadyJoined)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 自定义规则车不走申报制：代加走 AddMemberDirect 分支，这里必须拒绝。
func TestCarpoolAddMemberRejectsCustomRuleCar(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectAddMemberLoad(mock, "recruiting", "custom", service.CarpoolCarTypeCustom)
	mock.ExpectRollback()

	_, err = repo.AddMember(context.Background(), 7, 99, service.AddCarpoolMemberInput{UserID: 55, DeclaredWeeklyQuotaUSD: 100})
	require.ErrorIs(t, err, service.ErrCarpoolCustomRuleClosed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// confirmed 车存量 Σ 已在区间外（脏数据），代加后复核仍出界 → 自动退回招募中，
// 不留「确认过却发不出去」的僵尸车。
func TestCarpoolAddMemberAutoUnconfirmsConfirmedCarpool(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	prepaid := service.CarpoolPrepaidCNY(service.CarpoolCarTypeQuotaV2, 50, 1200, 2400, 2100, 100, 6)

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectAddMemberLoad(mock, "confirmed", "quota", service.CarpoolCarTypeQuotaV2)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(2000.0, 5))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO carpool_members (carpool_id, user_id, role, status,")).
		WithArgs(int64(7), int64(55), 100.0, prepaid, true).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(99), "member_added").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Σ=2100 < 2280（95% 下限）→ 无进区间通知；reconcile 先清提醒再退回招募中。
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2100.0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpools SET launch_notified_at = NULL, updated_at = NOW()")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpools SET status = 'recruiting', confirmed_at = NULL, confirmed_by = NULL,")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(99), "unconfirmed").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE c.id = $2")).
		WithArgs(int64(99), int64(7)).
		WillReturnRows(carpoolDetailRowWithCarType(service.CarpoolCarTypeQuotaV2))
	mock.ExpectCommit()

	result, err := repo.AddMember(context.Background(), 7, 99, service.AddCarpoolMemberInput{
		UserID: 55, DeclaredWeeklyQuotaUSD: 100, AcknowledgedRisk: true,
	})
	require.NoError(t, err)
	require.True(t, result.AutoUnconfirmed)
	require.False(t, result.LaunchBandEntered)
	require.NoError(t, mock.ExpectationsWereMet())
}

// expectAddDirectMemberLoad 是手动车代加成员的锁车加载期望。
func expectAddDirectMemberLoad(mock sqlmock.Sqlmock, status string, carType int, groupID any) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, car_type, weekly_limit_usd, group_id")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "car_type", "weekly_limit_usd", "group_id"}).
			AddRow(status, carType, 2400.0, groupID))
}

// 手动车（type 1）代加成员第一段：成员行直接 active、0 申报，返回建订阅所需的
// group_id 与车周限额；随后 BindMemberSubscription 回填 subscription_id。
func TestCarpoolAddMemberDirectCreatesActiveMemberAndBindsSubscription(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectAddDirectMemberLoad(mock, "active", service.CarpoolCarTypeQuotaLegacy, int64(91))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO carpool_members (carpool_id, user_id, role, status, declared_weekly_quota_usd,")).
		WithArgs(int64(7), int64(55), true).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(99), "member_added").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE c.id = $2")).
		WithArgs(int64(99), int64(7)).
		WillReturnRows(carpoolDetailRowWithCarType(service.CarpoolCarTypeQuotaLegacy))
	mock.ExpectCommit()

	result, groupID, weeklyLimitUSD, err := repo.AddMemberDirect(context.Background(), 7, 99, service.AddCarpoolMemberInput{
		UserID: 55, AcknowledgedRisk: true,
	})
	require.NoError(t, err)
	require.NotNil(t, result.Carpool)
	require.Equal(t, int64(91), groupID)
	require.InDelta(t, 2400.0, weeklyLimitUSD, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())

	// 回填 subscription_id：仅对仍 active 且未绑定订阅的成员行生效。
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpool_members SET subscription_id = $3, updated_at = NOW()")).
		WithArgs(int64(7), int64(55), int64(1001)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.BindMemberSubscription(context.Background(), 7, 55, 1001))
	require.NoError(t, mock.ExpectationsWereMet())
}

// 回填时成员行已不在「active 且未绑定」状态（补偿/并发动过）→ 报错，不覆盖。
func TestCarpoolBindMemberSubscriptionRejectsBoundRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpool_members SET subscription_id = $3, updated_at = NOW()")).
		WithArgs(int64(7), int64(55), int64(1001)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	require.ErrorIs(t, repo.BindMemberSubscription(context.Background(), 7, 55, 1001), service.ErrCarpoolNotMember)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 手动车重复成员（status active）拒绝。
func TestCarpoolAddMemberDirectRejectsDuplicateMember(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectAddDirectMemberLoad(mock, "active", service.CarpoolCarTypeCustom, int64(91))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), int64(55)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("active"))
	mock.ExpectRollback()

	_, _, _, err = repo.AddMemberDirect(context.Background(), 7, 99, service.AddCarpoolMemberInput{UserID: 55})
	require.ErrorIs(t, err, service.ErrCarpoolAlreadyJoined)
	require.NoError(t, mock.ExpectationsWereMet())
}

// quota 车（type 2/3）不走直接生效分支。
func TestCarpoolAddMemberDirectRejectsQuotaCar(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectAddDirectMemberLoad(mock, "recruiting", service.CarpoolCarTypeQuotaV2, nil)
	mock.ExpectRollback()

	_, _, _, err = repo.AddMemberDirect(context.Background(), 7, 99, service.AddCarpoolMemberInput{UserID: 55})
	require.ErrorIs(t, err, service.ErrCarpoolInvalidRequest)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 手动车必须在 active 状态：招募中的手动老车（升级前遗留）不能走直接生效。
func TestCarpoolAddMemberDirectRequiresActiveStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectAddDirectMemberLoad(mock, "recruiting", service.CarpoolCarTypeQuotaLegacy, nil)
	mock.ExpectRollback()

	_, _, _, err = repo.AddMemberDirect(context.Background(), 7, 99, service.AddCarpoolMemberInput{UserID: 55})
	require.ErrorIs(t, err, service.ErrCarpoolUnavailable)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 失败补偿：把尚未绑定订阅的成员行退回 left 并记原因。
func TestCarpoolRemoveDirectMemberMarksUnboundRowLeft(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpool_members SET status = 'left', left_at = NOW(),")).
		WithArgs(int64(7), int64(55), int64(99), "subscription assignment failed").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(99), "member_removed").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.RemoveDirectMember(context.Background(), 7, 55, 99, "subscription assignment failed"))
	require.NoError(t, mock.ExpectationsWereMet())
}

package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 管理端「发车前改成员」两个操作共用的前置查询：锁车 + 确认目标在册。
func expectMemberOpLoad(mock sqlmock.Sqlmock, status string, ownerID int64, pricingModel string, memberID int64) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, owner_user_id, weekly_limit_usd, launch_min_ratio, launch_max_ratio,")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "owner_user_id", "weekly_limit_usd", "launch_min_ratio", "launch_max_ratio", "pricing_model"}).
			AddRow(status, ownerID, 2400.0, 0.95, 1.05, pricingModel))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2")).
		WithArgs(int64(7), memberID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("joined"))
}

// 管理员把 confirmed 车的成员移走、Σ申报 跌破发车线：车必须自动退回招募中，
// 否则会留下一辆「确认过却永远发不出去」的僵尸车。
func TestRemoveMemberAutoUnconfirmsConfirmedCarpool(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectMemberOpLoad(mock, "confirmed", 11, "quota", 55)
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE carpool_members SET status = 'left', left_at = NOW(), updated_at = NOW()")).
		WithArgs(int64(7), int64(55)).
		WillReturnRows(sqlmock.NewRows([]string{"declared_weekly_quota_usd"}).AddRow(250.0))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(99), "member_removed").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Σ=2050 < 2280（95% 下限）→ 先清发车提醒，再退回招募中。
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2050.0))
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
		WillReturnRows(carpoolDetailRow())
	mock.ExpectCommit()

	result, err := repo.RemoveMember(context.Background(), 7, 55, 99)
	require.NoError(t, err)
	require.True(t, result.AutoUnconfirmed)
	require.InDelta(t, 250.0, result.DeclaredWeeklyQuotaUSD, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 车主不能移：移走车主这辆车就再没人能确认发车或取消。要散车走 Cancel。
func TestRemoveMemberRejectsOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectMemberOpLoad(mock, "recruiting", 55, "quota", 55)
	mock.ExpectRollback()

	_, err = repo.RemoveMember(context.Background(), 7, 55, 99)
	require.ErrorIs(t, err, service.ErrCarpoolOwnerCannotLeave)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 自定义规则车不走申报制（Join 已对它关闭，Σ申报 恒为 0）。给它的成员写申报会把
// Σ抬进发车区间，让一辆本该永远确认不了的车一路走到发车——Launch 会按额度预约制
// 给全车建订阅，与自定义规则的人工结算冲突。
func TestUpdateMemberQuotaRejectsCustomRuleCars(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectMemberOpLoad(mock, "recruiting", 11, "custom", 55)
	mock.ExpectRollback()

	_, err = repo.UpdateMemberQuota(context.Background(), 7, 55, 99, 250)
	require.ErrorIs(t, err, service.ErrCarpoolCustomRuleClosed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 代改申报的全车硬上限与上车一致：Σ其他人 + 新申报 > 105%×周限额 时拒绝，
// 免得改出一辆永远确认不了的超额车。
func TestUpdateMemberQuotaEnforcesHardCap(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectMemberOpLoad(mock, "confirmed", 11, "quota", 55)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0) FROM carpool_members")).
		WithArgs(int64(7), int64(55)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2450.0))
	mock.ExpectRollback()

	_, err = repo.UpdateMemberQuota(context.Background(), 7, 55, 99, 100)
	require.ErrorIs(t, err, service.ErrCarpoolQuotaExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 改完额度 Σ申报 仍在区间内：confirmed 保持不变，不动 confirmed_at，也不打扰车主。
func TestUpdateMemberQuotaKeepsConfirmedWhenStillInBand(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	expectMemberOpLoad(mock, "confirmed", 11, "quota", 55)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0) FROM carpool_members")).
		WithArgs(int64(7), int64(55)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2100.0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpool_members SET declared_weekly_quota_usd = $3, updated_at = NOW()")).
		WithArgs(int64(7), int64(55), 250.0).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(99), "member_quota_adjusted").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Σ=2350 ∈ [2280, 2520] → reconcile 不写任何行。
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(declared_weekly_quota_usd), 0) FROM carpool_members")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2350.0))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE c.id = $2")).
		WithArgs(int64(99), int64(7)).
		WillReturnRows(carpoolDetailRow())
	mock.ExpectCommit()

	result, err := repo.UpdateMemberQuota(context.Background(), 7, 55, 99, 250)
	require.NoError(t, err)
	require.False(t, result.AutoUnconfirmed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 管理员取消已发车的车：成员订阅必须一并软删——车没了人还能继续用额度是最贵的那种bug。
func TestCancelActiveCarpoolAsAdminTearsDownSubscriptions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status FROM carpools WHERE id = $1 FOR UPDATE")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status"}).AddRow(int64(11), "active"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpools SET status = 'cancelled', join_locked_at = NOW(), cancelled_at = NOW(),")).
		WithArgs(int64(7), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE user_subscriptions SET deleted_at = NOW()")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpool_members SET status = 'cancelled', updated_at = NOW() WHERE carpool_id = $1 AND status IN ('joined', 'active')")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpool_invites SET revoked_at")).
		WithArgs(int64(7), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(99), "cancelled").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.Cancel(context.Background(), 7, 99, true))
	require.NoError(t, mock.ExpectationsWereMet())
}

// 已发车的车对车主也是全锁：散车等于停掉别人的真订阅，只能管理员来做。
func TestCancelActiveCarpoolForbiddenForOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status FROM carpools WHERE id = $1 FOR UPDATE")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status"}).AddRow(int64(11), "active"))
	mock.ExpectRollback()

	err = repo.Cancel(context.Background(), 7, 11, false)
	require.ErrorIs(t, err, service.ErrCarpoolUnavailable)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 车主可以换自己车的群二维码（群过期/换群不必整车重建）。
func TestSetGroupQRCodeAsOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status FROM carpools WHERE id = $1 FOR UPDATE")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status"}).AddRow(int64(11), "active"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE carpools SET group_qr_code = $2, group_qr_code_content_type = $3,")).
		WithArgs(int64(7), []byte{0x89, 0x50, 0x4E, 0x47}, "image/png").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO carpool_events").
		WithArgs(int64(7), int64(11), "qr_code_replaced").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.SetGroupQRCode(context.Background(), 7, 11, false, []byte{0x89, 0x50, 0x4E, 0x47}, "image/png"))
	require.NoError(t, mock.ExpectationsWereMet())
}

// 既不是车主也不是 admin → 403。
func TestSetGroupQRCodeForbiddenForNonOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status FROM carpools WHERE id = $1 FOR UPDATE")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status"}).AddRow(int64(11), "recruiting"))
	mock.ExpectRollback()

	err = repo.SetGroupQRCode(context.Background(), 7, 55, false, []byte{0x89}, "image/png")
	require.ErrorIs(t, err, service.ErrCarpoolForbidden)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 已取消/已结束的车群已散，换码没有意义。
func TestSetGroupQRCodeRejectsEndedCarpool(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCarpoolRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT owner_user_id, status FROM carpools WHERE id = $1 FOR UPDATE")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status"}).AddRow(int64(11), "ended"))
	mock.ExpectRollback()

	err = repo.SetGroupQRCode(context.Background(), 7, 99, true, []byte{0x89}, "image/png")
	require.ErrorIs(t, err, service.ErrCarpoolUnavailable)
	require.NoError(t, mock.ExpectationsWereMet())
}

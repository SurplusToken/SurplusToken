package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// applyUsageBillingEffects 的订阅计费分支：UPDATE...RETURNING 在同一语句内取回
// 新周用量/窗口/保底，精确算出应计入组级公共池的增量（越界部分）。
func TestApplyUsageBillingEffects_ComputesCarpoolCommonsDelta(t *testing.T) {
	windowStart := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)

	newMockTx := func(t *testing.T) (sqlmock.Sqlmock, *sql.Tx) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mock.ExpectBegin()
		tx, err := db.BeginTx(context.Background(), nil)
		require.NoError(t, err)
		return mock, tx
	}

	incrementRows := func() *sqlmock.Rows {
		return sqlmock.NewRows(
			[]string{"weekly_usage_usd", "weekly_window_start", "weekly_reserved_usd", "group_id"})
	}

	t.Run("跨界：只有越过保底的部分计入公共池（950→1000，r=960，delta=40）", func(t *testing.T) {
		mock, tx := newMockTx(t)
		subID := int64(101)
		mock.ExpectQuery("UPDATE user_subscriptions").
			WithArgs(50.0, subID).
			WillReturnRows(incrementRows().AddRow(1000.0, windowStart, 960.0, int64(91)))
		mock.ExpectCommit()

		result := &service.UsageBillingApplyResult{Applied: true}
		err := (&usageBillingRepository{}).applyUsageBillingEffects(context.Background(), tx, &service.UsageBillingCommand{
			SubscriptionID: &subID, SubscriptionCost: 50,
		}, result)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		require.NotNil(t, result.CarpoolCommonsDelta)
		require.Equal(t, int64(91), result.CarpoolCommonsDelta.GroupID)
		require.Equal(t, windowStart, result.CarpoolCommonsDelta.WindowStart)
		require.InDelta(t, 40.0, result.CarpoolCommonsDelta.DeltaUSD, 1e-9)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("保底内：不产生公共池增量（900→950，r=960）", func(t *testing.T) {
		mock, tx := newMockTx(t)
		subID := int64(101)
		mock.ExpectQuery("UPDATE user_subscriptions").
			WithArgs(50.0, subID).
			WillReturnRows(incrementRows().AddRow(950.0, windowStart, 960.0, int64(91)))
		mock.ExpectCommit()

		result := &service.UsageBillingApplyResult{Applied: true}
		err := (&usageBillingRepository{}).applyUsageBillingEffects(context.Background(), tx, &service.UsageBillingCommand{
			SubscriptionID: &subID, SubscriptionCost: 50,
		}, result)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		require.Nil(t, result.CarpoolCommonsDelta)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("非拼车订阅（reserved 为 NULL）：不产生增量，行为不变", func(t *testing.T) {
		mock, tx := newMockTx(t)
		subID := int64(101)
		mock.ExpectQuery("UPDATE user_subscriptions").
			WithArgs(50.0, subID).
			WillReturnRows(incrementRows().AddRow(5000.0, windowStart, nil, int64(91)))
		mock.ExpectCommit()

		result := &service.UsageBillingApplyResult{Applied: true}
		err := (&usageBillingRepository{}).applyUsageBillingEffects(context.Background(), tx, &service.UsageBillingCommand{
			SubscriptionID: &subID, SubscriptionCost: 50,
		}, result)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		require.Nil(t, result.CarpoolCommonsDelta)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("订阅不存在：保持 ErrSubscriptionNotFound 语义", func(t *testing.T) {
		mock, tx := newMockTx(t)
		subID := int64(999)
		mock.ExpectQuery("UPDATE user_subscriptions").
			WithArgs(50.0, subID).
			WillReturnRows(incrementRows())

		result := &service.UsageBillingApplyResult{Applied: true}
		err := (&usageBillingRepository{}).applyUsageBillingEffects(context.Background(), tx, &service.UsageBillingCommand{
			SubscriptionID: &subID, SubscriptionCost: 50,
		}, result)
		require.ErrorIs(t, err, service.ErrSubscriptionNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

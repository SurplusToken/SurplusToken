package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAccountRepositorySumOthersWeeklySpendUsesOriginalModelCost(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	since := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)SELECT COALESCE\(SUM\(total_cost\), 0\) FROM usage_logs`).
		WithArgs(int64(170), since, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"total_cost"}).AddRow(12.5))

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	spend, err := repo.SumOthersWeeklySpend(context.Background(), 170, []int64{223, 224}, since)

	require.NoError(t, err)
	require.Equal(t, 12.5, spend)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOthersWeeklySpendCacheKeyVersionsOriginalCostSemantics(t *testing.T) {
	since := time.Date(2026, time.August, 9, 4, 31, 0, 0, time.FixedZone("CST", 8*60*60))
	require.Equal(t, "others_weekly_spend:v3:account:170:window:59540702", othersWeeklySpendKey(170, since))
}

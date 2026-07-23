package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetModelPerformance_AggregatesPassiveTraffic(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	start := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"model",
		"group_id",
		"success_count",
		"failure_count",
		"latency_sample_count",
		"ttft_sample_count",
		"avg_latency_ms",
		"avg_ttft_ms",
	}).AddRow("gpt-5", int64(7), int64(18), int64(2), int64(18), int64(16), int64(740), int64(180))

	mock.ExpectQuery(`WITH final_failures AS`).
		WithArgs(start, end, `{7,9}`).
		WillReturnRows(rows)

	stats, err := repo.GetModelPerformance(context.Background(), &service.ModelPerformanceQuery{
		StartTime: start,
		EndTime:   end,
		GroupIDs:  []int64{7, 9, 7, 0},
	})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, "gpt-5", stats[0].Model)
	require.Equal(t, int64(7), stats[0].GroupID)
	require.Equal(t, int64(18), stats[0].SuccessCount)
	require.Equal(t, int64(2), stats[0].FailureCount)
	require.Equal(t, 740, *stats[0].AvgLatencyMs)
	require.Equal(t, 180, *stats[0].AvgTTFTMs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetModelPerformance_EmptyVisibleGroupsSkipsQuery(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	start := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

	stats, err := repo.GetModelPerformance(context.Background(), &service.ModelPerformanceQuery{
		StartTime: start,
		EndTime:   start.Add(time.Hour),
		GroupIDs:  []int64{0, -1},
	})
	require.NoError(t, err)
	require.Empty(t, stats)
	require.NoError(t, mock.ExpectationsWereMet())
}

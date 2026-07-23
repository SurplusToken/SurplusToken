package service

import (
	"context"
	"time"
)

const ModelPerformanceWindowHours = 24

// ModelPerformanceQuery limits passive performance aggregation to groups the
// current user can access. It never triggers an upstream request.
type ModelPerformanceQuery struct {
	StartTime time.Time
	EndTime   time.Time
	GroupIDs  []int64
}

// ModelPerformanceStat is aggregated from real user traffic. Model is the
// normalized client-requested model name used only for joining catalog rows.
type ModelPerformanceStat struct {
	Model              string
	GroupID            int64
	SuccessCount       int64
	FailureCount       int64
	LatencySampleCount int64
	TTFTSampleCount    int64
	AvgLatencyMs       *int
	AvgTTFTMs          *int
}

func (s *OpsService) GetModelPerformance(
	ctx context.Context,
	groupIDs []int64,
) ([]ModelPerformanceStat, error) {
	if s == nil || s.opsRepo == nil || len(groupIDs) == 0 {
		return []ModelPerformanceStat{}, nil
	}
	end := time.Now().UTC()
	return s.opsRepo.GetModelPerformance(ctx, &ModelPerformanceQuery{
		StartTime: end.Add(-ModelPerformanceWindowHours * time.Hour),
		EndTime:   end,
		GroupIDs:  groupIDs,
	})
}

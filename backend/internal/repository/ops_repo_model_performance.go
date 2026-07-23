package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const modelPerformanceQuery = `
WITH final_failures AS (
  SELECT DISTINCT ON (e.request_id)
    e.request_id,
    LOWER(TRIM(COALESCE(NULLIF(e.requested_model, ''), NULLIF(e.model, '')))) AS model,
    e.group_id,
    e.created_at
  FROM ops_error_logs e
  WHERE e.created_at >= $1
    AND e.created_at < $2
    AND e.group_id = ANY($3)
    AND COALESCE(e.request_id, '') <> ''
    AND COALESCE(e.status_code, 0) >= 400
    AND e.is_count_tokens = FALSE
    AND e.is_business_limited = FALSE
    AND COALESCE(e.error_owner, '') IN ('provider', 'platform')
    AND TRIM(COALESCE(NULLIF(e.requested_model, ''), NULLIF(e.model, ''))) <> ''
  ORDER BY e.request_id, e.created_at DESC, e.id DESC
),
successful_requests AS (
  SELECT DISTINCT ON (ul.request_id)
    ul.request_id,
    LOWER(TRIM(COALESCE(NULLIF(ul.requested_model, ''), ul.model))) AS model,
    ul.group_id,
    ul.duration_ms,
    ul.first_token_ms
  FROM usage_logs ul
  WHERE ul.created_at >= $1
    AND ul.created_at < $2
    AND ul.group_id = ANY($3)
    AND TRIM(COALESCE(NULLIF(ul.requested_model, ''), ul.model)) <> ''
    AND NOT EXISTS (
      SELECT 1
      FROM final_failures failure
      WHERE failure.request_id = ul.request_id
    )
  ORDER BY ul.request_id, ul.created_at DESC, ul.id DESC
),
samples AS (
  SELECT
    model,
    group_id,
    TRUE AS success,
    duration_ms,
    first_token_ms
  FROM successful_requests
  UNION ALL
  SELECT
    model,
    group_id,
    FALSE AS success,
    NULL::INT AS duration_ms,
    NULL::INT AS first_token_ms
  FROM final_failures
)
SELECT
  model,
  group_id,
  COUNT(*) FILTER (WHERE success) AS success_count,
  COUNT(*) FILTER (WHERE NOT success) AS failure_count,
  COUNT(duration_ms) FILTER (WHERE success) AS latency_sample_count,
  COUNT(first_token_ms) FILTER (WHERE success) AS ttft_sample_count,
  ROUND(AVG(duration_ms) FILTER (WHERE success AND duration_ms IS NOT NULL))::BIGINT AS avg_latency_ms,
  ROUND(AVG(first_token_ms) FILTER (WHERE success AND first_token_ms IS NOT NULL))::BIGINT AS avg_ttft_ms
FROM samples
WHERE group_id IS NOT NULL
GROUP BY model, group_id
ORDER BY model, group_id`

func (r *opsRepository) GetModelPerformance(
	ctx context.Context,
	query *service.ModelPerformanceQuery,
) ([]service.ModelPerformanceStat, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if query == nil || query.StartTime.IsZero() || query.EndTime.IsZero() {
		return nil, fmt.Errorf("model performance time window required")
	}
	if !query.StartTime.Before(query.EndTime) {
		return nil, fmt.Errorf("model performance start_time must be before end_time")
	}
	groupIDs := positiveUniqueInt64s(query.GroupIDs)
	if len(groupIDs) == 0 {
		return []service.ModelPerformanceStat{}, nil
	}

	rows, err := r.db.QueryContext(
		ctx,
		modelPerformanceQuery,
		query.StartTime.UTC(),
		query.EndTime.UTC(),
		pq.Array(groupIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("query model performance: %w", err)
	}
	defer rows.Close()

	stats := make([]service.ModelPerformanceStat, 0)
	for rows.Next() {
		var stat service.ModelPerformanceStat
		var avgLatency sql.NullInt64
		var avgTTFT sql.NullInt64
		if err := rows.Scan(
			&stat.Model,
			&stat.GroupID,
			&stat.SuccessCount,
			&stat.FailureCount,
			&stat.LatencySampleCount,
			&stat.TTFTSampleCount,
			&avgLatency,
			&avgTTFT,
		); err != nil {
			return nil, fmt.Errorf("scan model performance: %w", err)
		}
		stat.Model = strings.ToLower(strings.TrimSpace(stat.Model))
		if stat.Model == "" || stat.GroupID <= 0 {
			continue
		}
		if avgLatency.Valid {
			value := int(avgLatency.Int64)
			stat.AvgLatencyMs = &value
		}
		if avgTTFT.Valid {
			value := int(avgTTFT.Int64)
			stat.AvgTTFTMs = &value
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model performance: %w", err)
	}
	return stats, nil
}

func positiveUniqueInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

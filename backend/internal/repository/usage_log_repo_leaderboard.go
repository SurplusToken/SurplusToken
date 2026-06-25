package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// UsageLeaderboard returns users ranked by consumed credits (actual_cost) for usage
// recorded at/after since, limited to the top `limit` rows.
//
// 复用 usage_logs 聚合查询的写法（r.sql / LEFT JOIN users / actual_cost > 0 过滤）：
//   - total_tokens = SUM(input + output + cache_creation + cache_read + image_output)
//   - total_cost   = SUM(actual_cost)（实际扣除的额度）
//
// 仅统计成功落账（actual_cost > 0）的请求，避免失败占位记录污染排行榜；
// 排除已删除用户（users.deleted_at IS NULL）。
func (r *usageLogRepository) UsageLeaderboard(ctx context.Context, since time.Time, limit int) (entries []service.LeaderboardEntry, err error) {
	query := `
		SELECT
			ul.user_id,
			COALESCE(u.username, '') AS username,
			COALESCE(u.email, '') AS email,
			COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens + ul.image_output_tokens), 0) AS total_tokens,
			COALESCE(SUM(ul.actual_cost), 0) AS total_cost
		FROM usage_logs ul
		LEFT JOIN users u ON u.id = ul.user_id
		WHERE ul.created_at >= $1
			AND ` + usageLogSuccessFilterUL + `
			AND (u.deleted_at IS NULL)
		GROUP BY ul.user_id, u.username, u.email
		ORDER BY total_cost DESC
		LIMIT $2
	`

	rows, err := r.sql.QueryContext(ctx, query, since, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			entries = nil
		}
	}()

	entries = make([]service.LeaderboardEntry, 0, limit)
	for rows.Next() {
		var row service.LeaderboardEntry
		if err := rows.Scan(
			&row.UserID,
			&row.Username,
			&row.Email,
			&row.TotalTokens,
			&row.TotalCost,
		); err != nil {
			return nil, err
		}
		entries = append(entries, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// UserUsageTotals returns one user's total tokens and consumed credits for usage recorded
// at/after since, along with their leaderboard rank.
//
// rank 通过统计「在同一时间窗内 actual_cost 总和严格大于该用户的不同用户数 + 1」得到，
// 因此即使该用户不在 top-N 内，rank 仍然正确。found 为 false 表示该用户在该时间窗内无用量。
func (r *usageLogRepository) UserUsageTotals(ctx context.Context, userID int64, since time.Time) (totalTokens int64, totalCost float64, rank int64, found bool, err error) {
	totalsQuery := `
		SELECT
			COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens + ul.image_output_tokens), 0) AS total_tokens,
			COALESCE(SUM(ul.actual_cost), 0) AS total_cost,
			COUNT(*) AS rows_count
		FROM usage_logs ul
		WHERE ul.created_at >= $1
			AND ul.user_id = $2
			AND ` + usageLogSuccessFilterUL + `
	`
	var rowsCount int64
	if err := scanSingleRow(ctx, r.sql, totalsQuery, []any{since, userID}, &totalTokens, &totalCost, &rowsCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, 0, false, nil
		}
		return 0, 0, 0, false, err
	}
	if rowsCount == 0 {
		return 0, 0, 0, false, nil
	}

	// rank = 1 + 在同一时间窗内 actual_cost 合计严格大于本用户的用户数。
	rankQuery := `
		SELECT COUNT(*) + 1
		FROM (
			SELECT ul.user_id
			FROM usage_logs ul
			LEFT JOIN users u ON u.id = ul.user_id
			WHERE ul.created_at >= $1
				AND ` + usageLogSuccessFilterUL + `
				AND (u.deleted_at IS NULL)
			GROUP BY ul.user_id
			HAVING COALESCE(SUM(ul.actual_cost), 0) > $2
		) higher
	`
	if err := scanSingleRow(ctx, r.sql, rankQuery, []any{since, totalCost}, &rank); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return totalTokens, totalCost, 1, true, nil
		}
		return 0, 0, 0, false, err
	}
	return totalTokens, totalCost, rank, true, nil
}

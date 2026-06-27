package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// LeaderboardEntry 是排行榜仓储层返回的单行原始数据（含用户名/邮箱用于生成展示名）。
type LeaderboardEntry struct {
	UserID      int64
	Username    string
	Email       string
	TotalTokens int64
	TotalCost   float64
}

// UsageLeaderboardItem 是排行榜对外返回的单条记录。
type UsageLeaderboardItem struct {
	Rank        int64   `json:"rank"`
	UserID      int64   `json:"user_id"`
	DisplayName string  `json:"display_name"`
	TotalTokens int64   `json:"total_tokens"`
	TotalCost   float64 `json:"total_cost"`
}

// UsageLeaderboardMe 是当前用户自身在排行榜中的行（无用量时为 nil）。
type UsageLeaderboardMe struct {
	Rank        int64   `json:"rank"`
	TotalTokens int64   `json:"total_tokens"`
	TotalCost   float64 `json:"total_cost"`
}

// UsageLeaderboardResponse 是排行榜接口的响应体。
type UsageLeaderboardResponse struct {
	Period  string                 `json:"period"`
	Entries []UsageLeaderboardItem `json:"entries"`
	Me      *UsageLeaderboardMe    `json:"me"`
}

const usageLeaderboardLimit = 50

// leaderboardSince 将 period 参数映射为统计起始时间（服务器配置时区）：
//   - today => 当天本地 00:00
//   - week  => 最近一个周一 00:00
//   - month => 本月 1 号 00:00
//
// 返回归一化后的 period（无效/缺省回退到 "today"）。
func leaderboardSince(period string) (string, time.Time) {
	now := timezone.Now()
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "week":
		return "week", timezone.StartOfWeek(now)
	case "month":
		return "month", timezone.StartOfMonth(now)
	case "today":
		return "today", timezone.StartOfDay(now)
	default:
		return "today", timezone.StartOfDay(now)
	}
}

// leaderboardDisplayName 优先使用用户名；为空时退化为掩码邮箱（保留本地部分前 2 个字符 + 域名），
// 例如 "ya***@pku.edu.cn"。两者都为空时退化为 "user#<id>"。
func leaderboardDisplayName(username, email string, userID int64) string {
	if name := strings.TrimSpace(username); name != "" {
		return name
	}
	if masked := maskLeaderboardEmail(email); masked != "" {
		return masked
	}
	return fmt.Sprintf("user#%d", userID)
}

// maskLeaderboardEmail 将邮箱本地部分掩码为「前 2 字符 + ***」，保留域名。非法/空邮箱返回空串。
// 例如 "yangpd@pku.edu.cn" => "ya***@pku.edu.cn"。
func maskLeaderboardEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return ""
	}
	local := email[:at]
	domain := email[at:] // 含 "@"
	if len(local) <= 2 {
		return local + "***" + domain
	}
	return local[:2] + "***" + domain
}

// LeaderboardTrendPoint 是排行榜趋势接口对外返回的单条记录（每个用户一条/每个时间点）。
type LeaderboardTrendPoint struct {
	UserID      int64  `json:"user_id"`
	DisplayName string `json:"display_name"`
	Date        string `json:"date"`
	Tokens      int64  `json:"tokens"`
}

// LeaderboardTrendResponse 是排行榜趋势接口的响应体。
type LeaderboardTrendResponse struct {
	Period      string                  `json:"period"`
	Granularity string                  `json:"granularity"`
	UsersTrend  []LeaderboardTrendPoint `json:"users_trend"`
}

const usageLeaderboardTrendLimit = 12

// leaderboardTrendGranularity 将归一化后的 period 映射为聚合粒度：
//   - today => "hour"
//   - week  => "day"
//   - month => "day"
func leaderboardTrendGranularity(period string) string {
	if period == "today" {
		return "hour"
	}
	return "day"
}

// GetUsageLeaderboardTrend 返回指定周期内 TOP 用户的逐时段 token 序列（每个用户一条线）。
// 任何已认证的普通用户均可调用；展示名按排行榜口径掩码，不泄露原始邮箱。
func (s *UsageService) GetUsageLeaderboardTrend(ctx context.Context, period string) (*LeaderboardTrendResponse, error) {
	normalizedPeriod, since := leaderboardSince(period)
	granularity := leaderboardTrendGranularity(normalizedPeriod)
	end := timezone.Now()

	points, err := s.usageRepo.GetUserUsageTrend(ctx, since, end, granularity, usageLeaderboardTrendLimit)
	if err != nil {
		return nil, fmt.Errorf("get usage leaderboard trend: %w", err)
	}

	items := make([]LeaderboardTrendPoint, 0, len(points))
	for _, p := range points {
		items = append(items, LeaderboardTrendPoint{
			UserID:      p.UserID,
			DisplayName: leaderboardDisplayName(p.Username, p.Email, p.UserID),
			Date:        p.Date,
			Tokens:      p.Tokens,
		})
	}

	return &LeaderboardTrendResponse{
		Period:      normalizedPeriod,
		Granularity: granularity,
		UsersTrend:  items,
	}, nil
}

// GetUsageLeaderboard 返回指定周期的用量排行榜（按消耗额度降序）以及当前用户自身的排名。
// 任何已认证的普通用户均可调用。
func (s *UsageService) GetUsageLeaderboard(ctx context.Context, period string, currentUserID int64) (*UsageLeaderboardResponse, error) {
	normalizedPeriod, since := leaderboardSince(period)

	entries, err := s.usageRepo.UsageLeaderboard(ctx, since, usageLeaderboardLimit)
	if err != nil {
		return nil, fmt.Errorf("get usage leaderboard: %w", err)
	}

	items := make([]UsageLeaderboardItem, 0, len(entries))
	for i, e := range entries {
		items = append(items, UsageLeaderboardItem{
			Rank:        int64(i + 1),
			UserID:      e.UserID,
			DisplayName: leaderboardDisplayName(e.Username, e.Email, e.UserID),
			TotalTokens: e.TotalTokens,
			TotalCost:   e.TotalCost,
		})
	}

	resp := &UsageLeaderboardResponse{
		Period:  normalizedPeriod,
		Entries: items,
		Me:      nil,
	}

	totalTokens, totalCost, rank, found, err := s.usageRepo.UserUsageTotals(ctx, currentUserID, since)
	if err != nil {
		return nil, fmt.Errorf("get usage leaderboard self totals: %w", err)
	}
	if found {
		resp.Me = &UsageLeaderboardMe{
			Rank:        rank,
			TotalTokens: totalTokens,
			TotalCost:   totalCost,
		}
	}

	return resp, nil
}

// GetGroupUsageLeaderboard ranks users by spend within one group (订阅) for the period.
// today/week are calendar-accurate; month is each user's subscription window ("订阅月").
// Any authenticated user may call it (consistent with the open platform leaderboard).
func (s *UsageService) GetGroupUsageLeaderboard(ctx context.Context, groupID int64, period string) (*UsageLeaderboardResponse, error) {
	normalizedPeriod, since := leaderboardSince(period)

	entries, err := s.usageRepo.GroupUsageLeaderboard(ctx, groupID, normalizedPeriod, since, usageLeaderboardLimit)
	if err != nil {
		return nil, fmt.Errorf("get group usage leaderboard: %w", err)
	}

	items := make([]UsageLeaderboardItem, 0, len(entries))
	for i, e := range entries {
		items = append(items, UsageLeaderboardItem{
			Rank:        int64(i + 1),
			UserID:      e.UserID,
			DisplayName: leaderboardDisplayName(e.Username, e.Email, e.UserID),
			TotalTokens: e.TotalTokens,
			TotalCost:   e.TotalCost,
		})
	}

	return &UsageLeaderboardResponse{Period: normalizedPeriod, Entries: items, Me: nil}, nil
}

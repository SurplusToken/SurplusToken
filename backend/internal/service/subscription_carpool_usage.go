package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

// CarpoolUsageMember is an anonymous member view for a carpool usage snapshot.
type CarpoolUsageMember struct {
	MemberNumber       int     `json:"member_number"`
	IsCurrentUser      bool    `json:"is_current_user"`
	DeclaredQuotaUSD   float64 `json:"declared_quota_usd"`
	ReservedQuotaUSD   float64 `json:"reserved_quota_usd"`
	UsageUSD           float64 `json:"usage_usd"`
	SharedPoolUsageUSD float64 `json:"shared_pool_usage_usd"`
}

// CarpoolSharedPoolUsage summarizes the shared portion of the carpool quota.
type CarpoolSharedPoolUsage struct {
	UsageUSD     float64 `json:"usage_usd"`
	CapacityUSD  float64 `json:"capacity_usd"`
	RemainingUSD float64 `json:"remaining_usd"`
}

// CarpoolUsageSnapshot summarizes one current-week carpool subscription.
type CarpoolUsageSnapshot struct {
	SubscriptionID   int64                  `json:"subscription_id"`
	WindowStart      time.Time              `json:"window_start"`
	WindowEnd        time.Time              `json:"window_end"`
	TotalUsageUSD    float64                `json:"total_usage_usd"`
	TotalCapacityUSD float64                `json:"total_capacity_usd"`
	SharedPool       CarpoolSharedPoolUsage `json:"shared_pool"`
	Members          []CarpoolUsageMember   `json:"members"`
}

type carpoolUsageRow struct {
	viewerSubscriptionID    int64
	groupID                 int64
	windowStart             time.Time
	viewerWeeklyLimitUSD    sql.NullFloat64
	viewerWeeklyReservedUSD float64
	memberSubscriptionID    int64
	declaredQuotaUSD        float64
	reservedQuotaUSD        float64
	usageUSD                float64
}

const eligibleCarpoolViewerSubscriptionsSQL = `SELECT DISTINCT s.id AS viewer_subscription_id
FROM user_subscriptions s
JOIN carpools c
  ON c.group_id = s.group_id
 AND c.status = 'active'
JOIN carpool_members viewer_cm
  ON viewer_cm.carpool_id = c.id
 AND viewer_cm.subscription_id = s.id
 AND viewer_cm.user_id = s.user_id
 AND viewer_cm.status IN ('joined', 'active')
WHERE s.user_id = $1
  AND s.status = 'active'
  AND s.expires_at > NOW()
  AND s.deleted_at IS NULL
  AND s.weekly_reserved_usd IS NOT NULL
  AND s.weekly_window_start IS NOT NULL
ORDER BY s.id`

const listCarpoolUsageSnapshotsSQL = `WITH mine AS (
    SELECT s.id AS viewer_subscription_id,
           s.group_id,
           c.id AS carpool_id,
           s.weekly_window_start AS window_start,
           s.weekly_limit_usd AS viewer_weekly_limit_usd,
           s.weekly_reserved_usd AS viewer_weekly_reserved_usd
    FROM user_subscriptions s
    JOIN carpools c
      ON c.group_id = s.group_id
     AND c.status = 'active'
    JOIN carpool_members viewer_cm
      ON viewer_cm.carpool_id = c.id
     AND viewer_cm.subscription_id = s.id
     AND viewer_cm.user_id = s.user_id
     AND viewer_cm.status IN ('joined', 'active')
    WHERE s.user_id = $1
      AND s.status = 'active'
      AND s.expires_at > NOW()
      AND s.deleted_at IS NULL
      AND s.weekly_reserved_usd IS NOT NULL
      AND s.weekly_window_start IS NOT NULL
)
SELECT mine.viewer_subscription_id,
       mine.group_id,
       mine.window_start,
       mine.viewer_weekly_limit_usd,
       mine.viewer_weekly_reserved_usd,
       member.id AS member_subscription_id,
       cm.declared_weekly_quota_usd AS declared_quota_usd,
       member.weekly_reserved_usd AS reserved_quota_usd,
       member.weekly_usage_usd AS usage_usd
FROM mine
JOIN carpool_members cm
  ON cm.carpool_id = mine.carpool_id
 AND cm.status IN ('joined', 'active')
JOIN user_subscriptions member
 ON cm.subscription_id = member.id
 AND cm.user_id = member.user_id
 AND member.group_id = mine.group_id
 AND member.weekly_window_start = mine.window_start
 AND member.deleted_at IS NULL
 AND member.weekly_reserved_usd IS NOT NULL
ORDER BY mine.viewer_subscription_id, member.id`

// ListCarpoolUsageSnapshots returns current-week anonymous usage snapshots for a user's carpools.
func (s *SubscriptionService) ListCarpoolUsageSnapshots(ctx context.Context, userID int64) ([]CarpoolUsageSnapshot, error) {
	if s == nil || s.entClient == nil {
		return nil, fmt.Errorf("subscription service client is not configured")
	}
	if s.userSubRepo == nil {
		return nil, fmt.Errorf("subscription service repository is not configured")
	}

	viewerSubscriptionIDs, err := s.listEligibleCarpoolViewerSubscriptionIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(viewerSubscriptionIDs) == 0 {
		return make([]CarpoolUsageSnapshot, 0), nil
	}
	for _, subscriptionID := range viewerSubscriptionIDs {
		sub, err := s.userSubRepo.GetByID(ctx, subscriptionID)
		if err != nil {
			return nil, err
		}
		if _, err := s.EnsureWindowMaintenance(ctx, sub); err != nil {
			return nil, err
		}
	}

	rows, err := s.entClient.QueryContext(ctx, listCarpoolUsageSnapshotsSQL, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	snapshots := make([]CarpoolUsageSnapshot, 0)
	bySubscriptionID := make(map[int64]int)
	reservedTotals := make(map[int64]float64)
	viewerRows := make(map[int64]carpoolUsageRow)

	for rows.Next() {
		var row carpoolUsageRow
		if err := rows.Scan(
			&row.viewerSubscriptionID,
			&row.groupID,
			&row.windowStart,
			&row.viewerWeeklyLimitUSD,
			&row.viewerWeeklyReservedUSD,
			&row.memberSubscriptionID,
			&row.declaredQuotaUSD,
			&row.reservedQuotaUSD,
			&row.usageUSD,
		); err != nil {
			return nil, err
		}

		index, ok := bySubscriptionID[row.viewerSubscriptionID]
		if !ok {
			index = len(snapshots)
			bySubscriptionID[row.viewerSubscriptionID] = index
			viewerRows[row.viewerSubscriptionID] = row
			snapshots = append(snapshots, CarpoolUsageSnapshot{
				SubscriptionID: row.viewerSubscriptionID,
				WindowStart:    row.windowStart,
				WindowEnd:      row.windowStart.Add(7 * 24 * time.Hour),
				Members:        make([]CarpoolUsageMember, 0),
			})
		}

		isCurrentUser := row.memberSubscriptionID == row.viewerSubscriptionID
		sharedPoolUsage := math.Max(0, row.usageUSD-row.reservedQuotaUSD)
		snapshots[index].Members = append(snapshots[index].Members, CarpoolUsageMember{
			IsCurrentUser:      isCurrentUser,
			DeclaredQuotaUSD:   row.declaredQuotaUSD,
			ReservedQuotaUSD:   row.reservedQuotaUSD,
			UsageUSD:           row.usageUSD,
			SharedPoolUsageUSD: sharedPoolUsage,
		})
		snapshots[index].TotalUsageUSD += row.usageUSD
		snapshots[index].SharedPool.UsageUSD += sharedPoolUsage
		reservedTotals[row.viewerSubscriptionID] += row.reservedQuotaUSD
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for index := range snapshots {
		snapshot := &snapshots[index]
		members := make([]CarpoolUsageMember, 0, len(snapshot.Members))
		for _, member := range snapshot.Members {
			if member.IsCurrentUser {
				member.MemberNumber = 0
				members = append(members, member)
			}
		}
		for _, member := range snapshot.Members {
			if !member.IsCurrentUser {
				member.MemberNumber = len(members)
				members = append(members, member)
			}
		}
		snapshot.Members = members

		viewer := viewerRows[snapshot.SubscriptionID]
		weeklyReservedUSD := viewer.viewerWeeklyReservedUSD
		windowStart := viewer.windowStart
		sub := &UserSubscription{
			GroupID:           viewer.groupID,
			WeeklyWindowStart: &windowStart,
			WeeklyReservedUSD: &weeklyReservedUSD,
		}
		if viewer.viewerWeeklyLimitUSD.Valid {
			weeklyLimitUSD := viewer.viewerWeeklyLimitUSD.Float64
			sub.WeeklyLimitUSD = &weeklyLimitUSD
		}
		capacityUSD := sub.CarpoolSharedPoolCapacityUSD()
		if s.billingCacheService != nil {
			capacityUSD = s.billingCacheService.carpoolCommonsCapacity(ctx, sub)
		}
		snapshot.SharedPool.CapacityUSD = math.Max(0, capacityUSD)
		snapshot.SharedPool.RemainingUSD = math.Max(0, snapshot.SharedPool.CapacityUSD-snapshot.SharedPool.UsageUSD)
		snapshot.TotalCapacityUSD = reservedTotals[snapshot.SubscriptionID] + snapshot.SharedPool.CapacityUSD
	}

	return snapshots, nil
}

func (s *SubscriptionService) listEligibleCarpoolViewerSubscriptionIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.entClient.QueryContext(ctx, eligibleCarpoolViewerSubscriptionsSQL, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	subscriptionIDs := make([]int64, 0)
	for rows.Next() {
		var subscriptionID int64
		if err := rows.Scan(&subscriptionID); err != nil {
			return nil, err
		}
		subscriptionIDs = append(subscriptionIDs, subscriptionID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return subscriptionIDs, nil
}

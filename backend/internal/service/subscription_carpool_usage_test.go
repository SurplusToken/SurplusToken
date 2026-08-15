package service

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func newCarpoolUsageTestService(t *testing.T, billingCacheService *BillingCacheService) (*SubscriptionService, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	weeklyWindowStart := startOfDay(time.Now())
	weeklyLimitUSD := 960.0
	weeklyReservedUSD := 480.0
	repo := &carpoolUsageUserSubRepoStub{
		subscriptions: map[int64]*UserSubscription{
			101: {
				ID:                101,
				UserID:            7,
				GroupID:           9,
				Status:            SubscriptionStatusActive,
				ExpiresAt:         time.Now().Add(24 * time.Hour),
				WeeklyWindowStart: &weeklyWindowStart,
				WeeklyLimitUSD:    &weeklyLimitUSD,
				WeeklyReservedUSD: &weeklyReservedUSD,
			},
		},
	}

	svc := NewSubscriptionService(nil, repo, billingCacheService, client, nil)
	t.Cleanup(svc.Stop)
	return svc, mock
}

type carpoolUsageUserSubRepoStub struct {
	userSubRepoNoop

	subscriptions    map[int64]*UserSubscription
	getByIDErr       error
	resetWeeklyErr   error
	getByIDCalls     []int64
	resetWeeklyCalls int
	onResetWeekly    func(time.Time)
}

func (r *carpoolUsageUserSubRepoStub) GetByID(ctx context.Context, id int64) (*UserSubscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.getByIDCalls = append(r.getByIDCalls, id)
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	sub := r.subscriptions[id]
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	copy := *sub
	return &copy, nil
}

func (r *carpoolUsageUserSubRepoStub) ResetWeeklyUsage(ctx context.Context, id int64, _ *time.Time, newStart time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.resetWeeklyCalls++
	if r.resetWeeklyErr != nil {
		return r.resetWeeklyErr
	}
	sub := r.subscriptions[id]
	copy := *sub
	copy.WeeklyWindowStart = &newStart
	copy.WeeklyUsageUSD = 0
	r.subscriptions[id] = &copy
	if r.onResetWeekly != nil {
		r.onResetWeekly(newStart)
	}
	return nil
}

func carpoolUsageRepo(t *testing.T, svc *SubscriptionService) *carpoolUsageUserSubRepoStub {
	t.Helper()
	repo, ok := svc.userSubRepo.(*carpoolUsageUserSubRepoStub)
	require.True(t, ok)
	return repo
}

const expectedEligibleCarpoolViewerSubscriptionsSQL = `SELECT DISTINCT s.id AS viewer_subscription_id
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

func carpoolUsageRows(window time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"viewer_subscription_id", "group_id", "window_start",
		"viewer_weekly_limit_usd", "viewer_weekly_reserved_usd",
		"member_subscription_id",
		"declared_quota_usd", "reserved_quota_usd", "usage_usd",
	}).
		// The viewer row is deliberately not first; anonymous members retain query order.
		AddRow(101, 9, window, 960.0, 480.0, 102, 700.0, 560.0, 588.0).
		AddRow(101, 9, window, 960.0, 480.0, 101, 600.0, 480.0, 426.4).
		AddRow(101, 9, window, 960.0, 480.0, 103, 600.0, 480.0, 352.7).
		AddRow(101, 9, window, 960.0, 480.0, 104, 500.0, 400.0, 330.1)
}

func carpoolUsageColumns() []string {
	return []string{
		"viewer_subscription_id", "group_id", "window_start",
		"viewer_weekly_limit_usd", "viewer_weekly_reserved_usd",
		"member_subscription_id",
		"declared_quota_usd", "reserved_quota_usd", "usage_usd",
	}
}

func expectEligibleCarpoolViewers(mock sqlmock.Sqlmock, viewerSubscriptionIDs ...int64) {
	rows := sqlmock.NewRows([]string{"viewer_subscription_id"})
	for _, id := range viewerSubscriptionIDs {
		rows.AddRow(id)
	}
	mock.ExpectQuery(regexp.QuoteMeta(expectedEligibleCarpoolViewerSubscriptionsSQL)).
		WithArgs(int64(7)).
		WillReturnRows(rows)
}

func expectCarpoolUsageQuery(mock sqlmock.Sqlmock, rows *sqlmock.Rows) {
	expectEligibleCarpoolViewers(mock, 101)
	mock.ExpectQuery(regexp.QuoteMeta("WITH mine")).
		WithArgs(int64(7)).
		WillReturnRows(rows)
}

func TestListCarpoolUsageAggregatesAnonymousSnapshot(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	window := *carpoolUsageRepo(t, svc).subscriptions[101].WeeklyWindowStart
	expectCarpoolUsageQuery(mock, carpoolUsageRows(window))

	snapshots, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)

	snapshot := snapshots[0]
	require.Equal(t, int64(101), snapshot.SubscriptionID)
	require.Equal(t, window, snapshot.WindowStart)
	require.Equal(t, window.Add(7*24*time.Hour), snapshot.WindowEnd)
	require.InDelta(t, 1697.2, snapshot.TotalUsageUSD, 1e-9)
	require.InDelta(t, 2400.0, snapshot.TotalCapacityUSD, 1e-9)
	require.InDelta(t, 28.0, snapshot.SharedPool.UsageUSD, 1e-9)
	require.InDelta(t, 480.0, snapshot.SharedPool.CapacityUSD, 1e-9)
	require.InDelta(t, 452.0, snapshot.SharedPool.RemainingUSD, 1e-9)
	require.Len(t, snapshot.Members, 4)
	require.Equal(t, 0, snapshot.Members[0].MemberNumber)
	require.True(t, snapshot.Members[0].IsCurrentUser)
	require.InDelta(t, 426.4, snapshot.Members[0].UsageUSD, 1e-9)
	require.Equal(t, 1, snapshot.Members[1].MemberNumber)
	require.False(t, snapshot.Members[1].IsCurrentUser)
	require.InDelta(t, 28.0, snapshot.Members[1].SharedPoolUsageUSD, 1e-9)
	require.Equal(t, []int{0, 1, 2, 3}, []int{
		snapshot.Members[0].MemberNumber,
		snapshot.Members[1].MemberNumber,
		snapshot.Members[2].MemberNumber,
		snapshot.Members[3].MemberNumber,
	})
	require.Equal(t, []float64{426.4, 588.0, 352.7, 330.1}, []float64{
		snapshot.Members[0].UsageUSD,
		snapshot.Members[1].UsageUSD,
		snapshot.Members[2].UsageUSD,
		snapshot.Members[3].UsageUSD,
	})

	payload, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "user_id")
	require.NotContains(t, string(payload), "username")
	require.NotContains(t, string(payload), "email")
	require.NoError(t, mock.ExpectationsWereMet())
}

// 展示给用户的共享池容量必须与执行侧同源：车周限额 − Σ保底。
//
// 早先这里会用"上游实测反推"的值覆盖静态值，于是界面显示的余量与真正用于
// 拦截的余量可能对不上；多账号的车上反推值还会被推高（分子是所有账号的用量、
// 分母只是其中一个账号的百分比），界面显示还有余量、请求却已经被拒。
func TestListCarpoolUsageShowsStaticSharedPoolCapacity(t *testing.T) {
	billingCacheService := &BillingCacheService{cfg: &config.Config{}}
	svc, mock := newCarpoolUsageTestService(t, billingCacheService)
	window := *carpoolUsageRepo(t, svc).subscriptions[101].WeeklyWindowStart
	expectCarpoolUsageQuery(mock, carpoolUsageRows(window))

	snapshots, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.InDelta(t, 480.0, snapshots[0].SharedPool.CapacityUSD, 1e-9)
	require.InDelta(t, 2400.0, snapshots[0].TotalCapacityUSD, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsageReturnsEmptyForNonCarpoolUser(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	expectEligibleCarpoolViewers(mock)

	snapshots, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, snapshots)
	require.Empty(t, snapshots)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsageReturnsEmptyWithoutEligibleViewerMembership(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	repo := carpoolUsageRepo(t, svc)
	expectEligibleCarpoolViewers(mock)

	snapshots, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, snapshots)
	require.Empty(t, snapshots)
	require.Empty(t, repo.getByIDCalls)
	require.Zero(t, repo.resetWeeklyCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsageHandlesNullViewerWeeklyLimit(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	window := *carpoolUsageRepo(t, svc).subscriptions[101].WeeklyWindowStart
	rows := sqlmock.NewRows(carpoolUsageColumns()).
		AddRow(101, 9, window, nil, 480.0, 101, 600.0, 480.0, 426.4).
		AddRow(101, 9, window, nil, 480.0, 102, 700.0, 560.0, 588.0)
	expectCarpoolUsageQuery(mock, rows)

	snapshots, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.Zero(t, snapshots[0].SharedPool.CapacityUSD)
	require.InDelta(t, 1040.0, snapshots[0].TotalCapacityUSD, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsageRejectsNilServiceOrClient(t *testing.T) {
	var nilService *SubscriptionService
	_, err := nilService.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.EqualError(t, err, "subscription service client is not configured")

	_, err = (&SubscriptionService{}).ListCarpoolUsageSnapshots(context.Background(), 7)
	require.EqualError(t, err, "subscription service client is not configured")

	svc, mock := newCarpoolUsageTestService(t, nil)
	svc.userSubRepo = nil
	_, err = svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.EqualError(t, err, "subscription service repository is not configured")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsagePropagatesRowsIterationFailure(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	window := *carpoolUsageRepo(t, svc).subscriptions[101].WeeklyWindowStart
	rows := sqlmock.NewRows(carpoolUsageColumns()).
		AddRow(101, 9, window, 960.0, 480.0, 101, 600.0, 480.0, 426.4).
		AddRow(101, 9, window, 960.0, 480.0, 102, 700.0, 560.0, 588.0).
		RowError(1, errors.New("rows failed"))
	expectCarpoolUsageQuery(mock, rows)

	_, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.EqualError(t, err, "rows failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsageQueryRequiresExactMembershipAuthorization(t *testing.T) {
	require.Equal(t, expectedEligibleCarpoolViewerSubscriptionsSQL, eligibleCarpoolViewerSubscriptionsSQL)
	require.Contains(t, eligibleCarpoolViewerSubscriptionsSQL, "JOIN carpool_members viewer_cm")
	require.Contains(t, eligibleCarpoolViewerSubscriptionsSQL, "viewer_cm.carpool_id = c.id")
	require.Contains(t, eligibleCarpoolViewerSubscriptionsSQL, "viewer_cm.subscription_id = s.id")
	require.Contains(t, eligibleCarpoolViewerSubscriptionsSQL, "viewer_cm.user_id = s.user_id")
	require.Contains(t, eligibleCarpoolViewerSubscriptionsSQL, "viewer_cm.status IN ('joined', 'active')")
	require.Contains(t, listCarpoolUsageSnapshotsSQL, "JOIN carpool_members viewer_cm")
	require.Contains(t, listCarpoolUsageSnapshotsSQL, "viewer_cm.carpool_id = c.id")
	require.Contains(t, listCarpoolUsageSnapshotsSQL, "viewer_cm.subscription_id = s.id")
	require.Contains(t, listCarpoolUsageSnapshotsSQL, "viewer_cm.user_id = s.user_id")
	require.Contains(t, listCarpoolUsageSnapshotsSQL, "viewer_cm.status IN ('joined', 'active')")
	require.Contains(t, listCarpoolUsageSnapshotsSQL, "cm.carpool_id = mine.carpool_id")
	require.Contains(t, listCarpoolUsageSnapshotsSQL, "cm.subscription_id = member.id")
	require.Contains(t, listCarpoolUsageSnapshotsSQL, "cm.user_id = member.user_id")
	require.Contains(t, listCarpoolUsageSnapshotsSQL, "s.status = 'active'")
	require.Contains(t, listCarpoolUsageSnapshotsSQL, "s.expires_at > NOW()")
	require.NotContains(t, listCarpoolUsageSnapshotsSQL, "member.status = 'active'")
	require.NotContains(t, listCarpoolUsageSnapshotsSQL, "member.expires_at > NOW()")
	require.NotContains(t, listCarpoolUsageSnapshotsSQL, "member_user_id")
}

func TestListCarpoolUsageIncludesInactiveMembersCurrentWindowUsage(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	window := *carpoolUsageRepo(t, svc).subscriptions[101].WeeklyWindowStart
	rows := sqlmock.NewRows(carpoolUsageColumns()).
		AddRow(101, 9, window, 960.0, 480.0, 101, 600.0, 480.0, 100.0).
		// These rows represent suspended and expired subscriptions whose carpool
		// memberships remain joined/active in the same billing window.
		AddRow(101, 9, window, 960.0, 480.0, 102, 700.0, 560.0, 700.0).
		AddRow(101, 9, window, 960.0, 480.0, 103, 600.0, 480.0, 600.0)
	expectCarpoolUsageQuery(mock, rows)

	snapshots, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.InDelta(t, 1400.0, snapshots[0].TotalUsageUSD, 1e-9)
	require.InDelta(t, 260.0, snapshots[0].SharedPool.UsageUSD, 1e-9)
	require.Len(t, snapshots[0].Members, 3)
	require.InDelta(t, 700.0, snapshots[0].Members[1].UsageUSD, 1e-9)
	require.InDelta(t, 600.0, snapshots[0].Members[2].UsageUSD, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsagePropagatesQueryFailure(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	expectEligibleCarpoolViewers(mock, 101)
	mock.ExpectQuery(regexp.QuoteMeta("WITH mine")).
		WithArgs(int64(7)).
		WillReturnError(errors.New("query failed"))

	_, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.EqualError(t, err, "query failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsagePropagatesEligibilityQueryFailure(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	mock.ExpectQuery(regexp.QuoteMeta(expectedEligibleCarpoolViewerSubscriptionsSQL)).
		WithArgs(int64(7)).
		WillReturnError(errors.New("eligibility query failed"))

	_, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.EqualError(t, err, "eligibility query failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsagePropagatesEligibilityRowsFailure(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	rows := sqlmock.NewRows([]string{"viewer_subscription_id"}).
		AddRow(101).
		RowError(0, errors.New("eligibility rows failed"))
	mock.ExpectQuery(regexp.QuoteMeta(expectedEligibleCarpoolViewerSubscriptionsSQL)).
		WithArgs(int64(7)).
		WillReturnRows(rows)

	_, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.EqualError(t, err, "eligibility rows failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsageRespectsCanceledContext(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.ListCarpoolUsageSnapshots(ctx, 7)
	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsagePropagatesRepositoryFailure(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	repo := carpoolUsageRepo(t, svc)
	repo.getByIDErr = errors.New("load subscription failed")
	expectEligibleCarpoolViewers(mock, 101)

	_, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.EqualError(t, err, "load subscription failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsagePropagatesWindowMaintenanceFailure(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	repo := carpoolUsageRepo(t, svc)
	staleWindow := startOfDay(time.Now()).Add(-8 * 24 * time.Hour)
	repo.subscriptions[101].WeeklyWindowStart = &staleWindow
	repo.resetWeeklyErr = errors.New("reset weekly usage failed")
	expectEligibleCarpoolViewers(mock, 101)

	_, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.EqualError(t, err, "reset weekly usage failed")
	require.Equal(t, 1, repo.resetWeeklyCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCarpoolUsageMaintainsStaleWindowBeforeAggregate(t *testing.T) {
	svc, mock := newCarpoolUsageTestService(t, nil)
	repo := carpoolUsageRepo(t, svc)
	staleWindow := startOfDay(time.Now()).Add(-8 * 24 * time.Hour)
	repo.subscriptions[101].WeeklyWindowStart = &staleWindow
	repo.subscriptions[101].WeeklyUsageUSD = 777
	expectEligibleCarpoolViewers(mock, 101)
	repo.onResetWeekly = func(refreshedWindow time.Time) {
		rows := sqlmock.NewRows(carpoolUsageColumns()).
			AddRow(101, 9, refreshedWindow, 960.0, 480.0, 101, 600.0, 480.0, 0.0)
		mock.ExpectQuery(regexp.QuoteMeta("WITH mine")).
			WithArgs(int64(7)).
			WillReturnRows(rows)
	}

	snapshots, err := svc.ListCarpoolUsageSnapshots(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, 1, repo.resetWeeklyCalls)
	require.Equal(t, []int64{101, 101}, repo.getByIDCalls)
	require.Len(t, snapshots, 1)
	require.NotEqual(t, staleWindow, snapshots[0].WindowStart)
	require.Zero(t, snapshots[0].TotalUsageUSD)
	require.Zero(t, snapshots[0].Members[0].UsageUSD)
	require.NotEqual(t, 777.0, snapshots[0].TotalUsageUSD)
	require.NoError(t, mock.ExpectationsWereMet())
}

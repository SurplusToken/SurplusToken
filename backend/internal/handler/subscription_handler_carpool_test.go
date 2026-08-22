package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

type handlerCarpoolUsageUserSubRepo struct {
	service.UserSubscriptionRepository

	sub          *service.UserSubscription
	getByIDCalls int
}

func (r *handlerCarpoolUsageUserSubRepo) GetByID(ctx context.Context, id int64) (*service.UserSubscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.getByIDCalls++
	requireID := r.sub != nil && r.sub.ID == id
	if !requireID {
		return nil, service.ErrSubscriptionNotFound
	}
	copy := *r.sub
	return &copy, nil
}

func TestSubscriptionHandlerListCarpoolUsageRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/carpool-usage", nil)

	(&SubscriptionHandler{}).ListCarpoolUsage(c)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSubscriptionHandlerListCarpoolUsageReturnsCurrentUsersSnapshots(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	windowStart := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	weeklyLimitUSD := 960.0
	weeklyReservedUSD := 480.0
	repo := &handlerCarpoolUsageUserSubRepo{sub: &service.UserSubscription{
		ID:                101,
		UserID:            7,
		GroupID:           9,
		Status:            service.SubscriptionStatusActive,
		ExpiresAt:         time.Now().Add(24 * time.Hour),
		WeeklyWindowStart: &windowStart,
		WeeklyLimitUSD:    &weeklyLimitUSD,
		WeeklyReservedUSD: &weeklyReservedUSD,
	}}
	subscriptionService := service.NewSubscriptionService(nil, repo, nil, client, nil)
	t.Cleanup(subscriptionService.Stop)

	eligibleRows := sqlmock.NewRows([]string{"viewer_subscription_id"}).AddRow(101)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT DISTINCT s.id AS viewer_subscription_id")).
		WithArgs(int64(7)).
		WillReturnRows(eligibleRows)
	rows := sqlmock.NewRows([]string{
		"viewer_subscription_id", "group_id", "window_start",
		"viewer_weekly_limit_usd", "viewer_weekly_reserved_usd",
		"member_subscription_id", "declared_quota_usd", "reserved_quota_usd", "usage_usd",
	}).AddRow(101, 9, windowStart, 960.0, 480.0, 101, 600.0, 480.0, 426.4)
	mock.ExpectQuery(regexp.QuoteMeta("WITH mine")).WithArgs(int64(7)).WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/carpool-usage", nil)
	c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 7})

	NewSubscriptionHandler(subscriptionService).ListCarpoolUsage(c)

	require.Equal(t, http.StatusOK, w.Code)
	var got response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 0, got.Code)
	data, ok := got.Data.([]any)
	require.True(t, ok)
	require.Len(t, data, 1)
	snapshot, ok := data[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(101), snapshot["subscription_id"])
	require.Equal(t, 2, repo.getByIDCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}

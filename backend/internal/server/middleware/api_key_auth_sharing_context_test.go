//go:build unit

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// sharingContextTestSettingRepo is a minimal SettingRepository stub whose only
// meaningful behavior is GetMultiple, which is what SettingService.
// IsSharingRangeFilterEnabled reads through loadSharingRateSettings.
type sharingContextTestSettingRepo struct {
	values map[string]string
}

func (r *sharingContextTestSettingRepo) Get(_ context.Context, _ string) (*service.Setting, error) {
	return nil, errors.New("not implemented")
}

func (r *sharingContextTestSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", service.ErrSettingNotFound
}

func (r *sharingContextTestSettingRepo) Set(_ context.Context, _, _ string) error {
	return errors.New("not implemented")
}

func (r *sharingContextTestSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := r.values[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func (r *sharingContextTestSettingRepo) SetMultiple(_ context.Context, _ map[string]string) error {
	return errors.New("not implemented")
}

func (r *sharingContextTestSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	return nil, errors.New("not implemented")
}

func (r *sharingContextTestSettingRepo) Delete(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

var _ service.SettingRepository = (*sharingContextTestSettingRepo)(nil)

func sharingRangeFilterEnabledSettingService() *service.SettingService {
	repo := &sharingContextTestSettingRepo{values: map[string]string{
		service.SettingKeySharingRangeFilterEnabled: "true",
	}}
	return service.NewSettingService(repo, &config.Config{})
}

func sharingContextTestUser(id int64) *service.User {
	min, max := 1.0, 2.0
	return &service.User{
		ID:             id,
		Role:           service.RoleUser,
		Status:         service.StatusActive,
		Balance:        10,
		Concurrency:    3,
		SharingRateMin: &min,
		SharingRateMax: &max,
	}
}

// TestAPIKeyAuthSetsSharingRateContext_StandardNormalMode exercises the main
// (non-Google) API key auth middleware end-to-end in RunModeStandard — i.e.
// past the SimpleMode early return, through the full billing-enforcement path
// down to the final setAuthRequestContext call — and asserts the requesting
// user ID, accepted sharing-rate range, and range-filter-enabled flag it
// injects into the request context are all correct.
func TestAPIKeyAuthSetsSharingRateContext_StandardNormalMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := sharingContextTestUser(7)
	apiKey := &service.APIKey{
		ID:     100,
		UserID: user.ID,
		Key:    "test-key",
		Status: service.StatusActive,
		User:   user,
		Group:  &service.Group{ID: 1, Platform: service.PlatformAnthropic, Status: service.StatusActive, Hydrated: true, SubscriptionType: service.SubscriptionTypeStandard, DynamicSharingPool: true},
	}
	apiKeyRepo := &stubApiKeyRepo{
		getByKey: func(ctx context.Context, key string) (*service.APIKey, error) {
			if key != apiKey.Key {
				return nil, service.ErrAPIKeyNotFound
			}
			clone := *apiKey
			return &clone, nil
		},
	}

	cfg := &config.Config{RunMode: config.RunModeStandard}
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, nil, nil, nil, nil, nil, cfg)
	settingService := sharingRangeFilterEnabledSettingService()

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg, settingService)))

	var gotUserID int64
	var gotMin, gotMax *float64
	var gotFilterEnabled bool
	router.GET("/t", func(c *gin.Context) {
		ctx := c.Request.Context()
		gotUserID = service.RequestingUserIDFromContext(ctx)
		gotMin, gotMax = service.SharingRateAcceptedRangeFromContext(ctx)
		gotFilterEnabled = service.SharingRangeFilterEnabledFromContext(ctx)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set("x-api-key", apiKey.Key)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, user.ID, gotUserID)
	require.NotNil(t, gotMin)
	require.NotNil(t, gotMax)
	require.Equal(t, 1.0, *gotMin)
	require.Equal(t, 2.0, *gotMax)
	require.True(t, gotFilterEnabled)
}

// TestAPIKeyAuthGoogleSetsSharingRateContext_SimpleMode exercises the Google
// API key auth middleware's SimpleMode early-return branch and asserts it
// injects the same requesting-user / sharing-rate-range / filter-enabled
// context as the standard middleware.
func TestAPIKeyAuthGoogleSetsSharingRateContext_SimpleMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := sharingContextTestUser(9)
	apiKey := &service.APIKey{
		ID:     200,
		UserID: user.ID,
		Key:    "g-key",
		Status: service.StatusActive,
		User:   user,
		Group:  &service.Group{ID: 2, Platform: service.PlatformGemini, Status: service.StatusActive, Hydrated: true, SubscriptionType: service.SubscriptionTypeStandard, DynamicSharingPool: true},
	}
	apiKeyRepo := &stubApiKeyRepo{
		getByKey: func(ctx context.Context, key string) (*service.APIKey, error) {
			if key != apiKey.Key {
				return nil, service.ErrAPIKeyNotFound
			}
			clone := *apiKey
			return &clone, nil
		},
	}

	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, nil, nil, nil, nil, nil, cfg)
	settingService := sharingRangeFilterEnabledSettingService()

	router := gin.New()
	router.Use(gin.HandlerFunc(APIKeyAuthWithSubscriptionGoogle(apiKeyService, nil, cfg, settingService)))

	var gotUserID int64
	var gotMin, gotMax *float64
	var gotFilterEnabled bool
	router.GET("/t", func(c *gin.Context) {
		ctx := c.Request.Context()
		gotUserID = service.RequestingUserIDFromContext(ctx)
		gotMin, gotMax = service.SharingRateAcceptedRangeFromContext(ctx)
		gotFilterEnabled = service.SharingRangeFilterEnabledFromContext(ctx)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set("x-goog-api-key", apiKey.Key)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, user.ID, gotUserID)
	require.NotNil(t, gotMin)
	require.NotNil(t, gotMax)
	require.Equal(t, 1.0, *gotMin)
	require.Equal(t, 2.0, *gotMax)
	require.True(t, gotFilterEnabled)
}

// TestNewAPIKeyAuthMiddleware_ThreeArgCallCompiles pins the backward-compatible
// 3-arg call site (no SettingService): it must keep compiling and behaving as
// before (sharing-range-filter defaults to false when settingService is nil).
func TestNewAPIKeyAuthMiddleware_ThreeArgCallCompiles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := sharingContextTestUser(11)
	apiKey := &service.APIKey{
		ID:     300,
		UserID: user.ID,
		Key:    "three-arg-key",
		Status: service.StatusActive,
		User:   user,
	}
	apiKeyRepo := &stubApiKeyRepo{
		getByKey: func(ctx context.Context, key string) (*service.APIKey, error) {
			if key != apiKey.Key {
				return nil, service.ErrAPIKeyNotFound
			}
			clone := *apiKey
			return &clone, nil
		},
	}

	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, nil, nil, nil, nil, nil, cfg)

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg)))

	var gotFilterEnabled bool
	router.GET("/t", func(c *gin.Context) {
		gotFilterEnabled = service.SharingRangeFilterEnabledFromContext(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set("x-api-key", apiKey.Key)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.False(t, gotFilterEnabled)
}

func TestSetAuthRequestContext_FixedGroupKeepsMarketplaceFilterOff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/t", nil)
	candidate := &service.APIKey{
		User: sharingContextTestUser(12),
		Group: &service.Group{
			ID: 3, Platform: service.PlatformOpenAI, Status: service.StatusActive,
			Hydrated: true, SubscriptionType: service.SubscriptionTypeStandard,
		},
	}
	setAuthRequestContext(c, candidate, sharingRangeFilterEnabledSettingService())
	require.False(t, service.DynamicSharingPoolEnabledFromContext(c.Request.Context()))
	require.False(t, service.SharingRangeFilterEnabledFromContext(c.Request.Context()))
}

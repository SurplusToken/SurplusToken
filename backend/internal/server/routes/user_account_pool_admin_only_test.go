package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newAccountPoolTestRouter 注册全部用户路由：JWT 桩按 Authorization 头注入角色，
// 其余依赖一律 nil（限流/守卫对 nil 均 fail-open，账号池 Handler 零值即可注册）。
func newAccountPoolTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	jwtAuth := servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
			return
		}
		role := service.RoleUser
		if authHeader == "Bearer admin-token" {
			role = service.RoleAdmin
		}
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 1})
		c.Set(string(servermiddleware.ContextKeyUserRole), role)
		c.Next()
	})
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	handlers := &handler.Handlers{AccountPool: &handler.AccountPoolHandler{}}
	RegisterUserRoutes(router.Group("/api/v1"), handlers, jwtAuth, auditLog, nil, nil, nil, nil, nil)
	return router
}

func TestUserAccountPoolRoutesRequireAdmin(t *testing.T) {
	router := newAccountPoolTestRouter()

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/accounts/pool"},
		{http.MethodGet, "/api/v1/accounts/pool/dynamic-groups"},
		{http.MethodPut, "/api/v1/accounts/pool/dynamic-groups/1/sharing-rate-range"},
		{http.MethodGet, "/api/v1/accounts/proxies"},
		{http.MethodPost, "/api/v1/accounts/proxies/1/test"},
		{http.MethodPost, "/api/v1/accounts/oauth/auth-url"},
		{http.MethodPost, "/api/v1/accounts/oauth"},
		{http.MethodGet, "/api/v1/accounts/1/models"},
		{http.MethodPost, "/api/v1/accounts/1/test"},
		{http.MethodPost, "/api/v1/accounts/1/refresh"},
		{http.MethodDelete, "/api/v1/accounts/1"},
		{http.MethodPatch, "/api/v1/accounts/1/schedulable"},
		{http.MethodPatch, "/api/v1/accounts/1/scope"},
		{http.MethodGet, "/api/v1/accounts/1/stats"},
		{http.MethodPost, "/api/v1/accounts/pool/1/remote-session/setup"},
		{http.MethodGet, "/api/v1/accounts/pool/1/remote-session/status"},
		{http.MethodGet, "/api/v1/accounts/pool/1/contribution-pool"},
		{http.MethodGet, "/api/v1/accounts/pool/1/usage"},
		{http.MethodPost, "/api/v1/accounts/pool/1/openai-quota/reset"},
		{http.MethodPost, "/api/v1/scheduled-test-plans"},
		{http.MethodPut, "/api/v1/scheduled-test-plans/1"},
		{http.MethodDelete, "/api/v1/scheduled-test-plans/1"},
		{http.MethodGet, "/api/v1/scheduled-test-plans/1/results"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, nil)
			request.Header.Set("Authorization", "Bearer user-token")
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusForbidden, recorder.Code)

			var body struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.Equal(t, "FORBIDDEN", body.Code)
		})
	}
}

func TestUserAccountPoolRoutesRejectUnauthenticated(t *testing.T) {
	router := newAccountPoolTestRouter()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/pool", nil)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

// 管理员请求必须穿过 AdminOnly 到达 Handler。测试里没有真实 service 图，
// 用 remote-session status 的「未配置」分支（503 REMOTE_SESSION_NOT_CONFIGURED）
// 作为「已到达 Handler」的确定性判据；池列表则兜底断言不再是 401/403。
func TestUserAccountPoolRoutesAllowAdminThroughGate(t *testing.T) {
	router := newAccountPoolTestRouter()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/pool/1/remote-session/status", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "REMOTE_SESSION_NOT_CONFIGURED")

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/accounts/pool", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	router.ServeHTTP(recorder, request)
	require.NotEqual(t, http.StatusForbidden, recorder.Code)
	require.NotEqual(t, http.StatusUnauthorized, recorder.Code)
}

// 对照组：AdminOnly 只应覆盖账号池路由组，普通用户路由不受影响（不应返回 403）。
func TestAdminOnlyDoesNotLeakToOtherUserRoutes(t *testing.T) {
	router := newAccountPoolTestRouter()

	for _, path := range []string{"/api/v1/user/profile", "/api/v1/keys", "/api/v1/usage"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer user-token")
		router.ServeHTTP(recorder, request)
		require.NotEqual(t, http.StatusForbidden, recorder.Code, "user route %s must stay reachable for normal users", path)
	}
}

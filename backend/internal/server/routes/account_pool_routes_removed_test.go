package routes

import (
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
// 其余依赖一律 nil（限流/守卫对 nil 均 fail-open）。
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
	handlers := &handler.Handlers{}
	RegisterUserRoutes(router.Group("/api/v1"), handlers, jwtAuth, auditLog, nil, nil, nil, nil, nil)
	return router
}

// 用户侧 OAuth 账号池已整体下线（产品决定，2026-10）：/accounts 与
// /scheduled-test-plans 路由组不再注册，管理员与普通用户调用一律 404。
// 管理端 /admin/accounts 与 /admin/scheduled-test-plans 不受影响。
func TestUserAccountPoolRoutesAreNotRegistered(t *testing.T) {
	router := newAccountPoolTestRouter()

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "list pool", method: http.MethodGet, path: "/api/v1/accounts/pool"},
		{name: "list dynamic pools", method: http.MethodGet, path: "/api/v1/accounts/pool/dynamic-groups"},
		{name: "update dynamic pool rate range", method: http.MethodPut, path: "/api/v1/accounts/pool/dynamic-groups/1/sharing-rate-range"},
		{name: "list proxies", method: http.MethodGet, path: "/api/v1/accounts/proxies"},
		{name: "test proxy", method: http.MethodPost, path: "/api/v1/accounts/proxies/1/test"},
		{name: "oauth auth url", method: http.MethodPost, path: "/api/v1/accounts/oauth/auth-url"},
		{name: "oauth exchange code", method: http.MethodPost, path: "/api/v1/accounts/oauth/exchange-code"},
		{name: "oauth refresh token", method: http.MethodPost, path: "/api/v1/accounts/oauth/refresh-token"},
		{name: "import codex session", method: http.MethodPost, path: "/api/v1/accounts/oauth/import/codex-session"},
		{name: "create oauth account", method: http.MethodPost, path: "/api/v1/accounts/oauth"},
		{name: "account models", method: http.MethodGet, path: "/api/v1/accounts/1/models"},
		{name: "test account", method: http.MethodPost, path: "/api/v1/accounts/1/test"},
		{name: "refresh account", method: http.MethodPost, path: "/api/v1/accounts/1/refresh"},
		{name: "recover account state", method: http.MethodPost, path: "/api/v1/accounts/1/recover-state"},
		{name: "set privacy", method: http.MethodPost, path: "/api/v1/accounts/1/set-privacy"},
		{name: "apply oauth credentials", method: http.MethodPost, path: "/api/v1/accounts/1/apply-oauth-credentials"},
		{name: "list scheduled test plans", method: http.MethodGet, path: "/api/v1/accounts/1/scheduled-test-plans"},
		{name: "account stats", method: http.MethodGet, path: "/api/v1/accounts/1/stats"},
		{name: "set schedulable", method: http.MethodPatch, path: "/api/v1/accounts/1/schedulable"},
		{name: "update scope", method: http.MethodPatch, path: "/api/v1/accounts/1/scope"},
		{name: "delete account", method: http.MethodDelete, path: "/api/v1/accounts/1"},
		{name: "owner account usage", method: http.MethodGet, path: "/api/v1/accounts/pool/1/usage"},
		{name: "owner openai quota", method: http.MethodGet, path: "/api/v1/accounts/pool/1/openai-quota"},
		{name: "reset owner openai quota", method: http.MethodPost, path: "/api/v1/accounts/pool/1/openai-quota/reset"},
		{name: "create scheduled test plan", method: http.MethodPost, path: "/api/v1/scheduled-test-plans"},
		{name: "update scheduled test plan", method: http.MethodPut, path: "/api/v1/scheduled-test-plans/1"},
		{name: "delete scheduled test plan", method: http.MethodDelete, path: "/api/v1/scheduled-test-plans/1"},
		{name: "list scheduled test results", method: http.MethodGet, path: "/api/v1/scheduled-test-plans/1/results"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, token := range []string{"Bearer admin-token", "Bearer user-token"} {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(tc.method, tc.path, nil)
				request.Header.Set("Authorization", token)
				router.ServeHTTP(recorder, request)
				require.Equal(t, http.StatusNotFound, recorder.Code)
			}
		})
	}
}

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

// 用户贡献账号体系已下线（产品决定，2026-10）：贡献余额/转账/提现与账号级
// 贡献奖励池端点不再注册，任何调用都应得到 404。对应 Handler/Service/Repository
// 源文件已删除，历史对账数据保留在 user_contributions 等表中。
func TestUserContributionRoutesAreNotRegistered(t *testing.T) {
	router := newAccountPoolTestRouter()

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "summary", method: http.MethodGet, path: "/api/v1/user/contribution"},
		{name: "transfer", method: http.MethodPost, path: "/api/v1/user/contribution/transfer"},
		{name: "list withdrawals", method: http.MethodGet, path: "/api/v1/user/contribution/withdrawals"},
		{name: "create withdrawal", method: http.MethodPost, path: "/api/v1/user/contribution/withdrawals"},
		{name: "withdrawal qr code", method: http.MethodGet, path: "/api/v1/user/contribution/withdrawals/1/qr-code"},
		{name: "cancel withdrawal", method: http.MethodPost, path: "/api/v1/user/contribution/withdrawals/1/cancel"},
		{name: "account contribution pool", method: http.MethodGet, path: "/api/v1/accounts/pool/1/contribution-pool"},
		{name: "distribute contribution pool", method: http.MethodPost, path: "/api/v1/accounts/pool/1/contribution-pool/distribute"},
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

// newAdminContributionTestRouter 注册全部管理端路由：AdminAuth 桩直接注入管理员
// 角色，其余依赖一律 nil（限流/合规守卫对 nil 均 fail-open），零值 Handlers 即可注册。
func newAdminContributionTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 1})
		c.Set(string(servermiddleware.ContextKeyUserRole), service.RoleAdmin)
		c.Next()
	})
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{}}
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, adminAuth, auditLog, nil, nil, nil)
	return router
}

// 管理端贡献提现审核端点随贡献体系一并下线：不再注册，一律 404。
func TestAdminContributionWithdrawalRoutesAreNotRegistered(t *testing.T) {
	router := newAdminContributionTestRouter()

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/v1/admin/contribution-withdrawals"},
		{name: "qr code", method: http.MethodGet, path: "/api/v1/admin/contribution-withdrawals/1/qr-code"},
		{name: "review", method: http.MethodPut, path: "/api/v1/admin/contribution-withdrawals/1/status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, nil)
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// webSessionLaunchHandler 构造未启用桌面网页版（signer=nil）的 handler：
// PublicOrigin 为空，launch 页的 form-action 应回落到 'none'。
func webSessionLaunchHandler() *CarpoolWebSessionHandler {
	return NewCarpoolWebSessionHandler(service.NewCarpoolWebSessionService(nil, nil, nil, nil))
}

func TestWebSessionLaunchRendersAutoSubmitPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/carpools/:id/web-session/launch", webSessionLaunchHandler().Launch)

	req := httptest.NewRequest(http.MethodGet, "/carpools/19/web-session/launch", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	// 未配置网关时 form-action 收紧为 'none'；CSP 允许 inline script 供自动兑换。
	csp := rec.Header().Get("Content-Security-Policy")
	require.Contains(t, csp, "script-src 'unsafe-inline'")
	require.Contains(t, csp, "connect-src 'self'")
	require.Contains(t, csp, "form-action 'none'")
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	body := rec.Body.String()
	// 兑换页按注入的车 ID 请求会话票据，并原地 POST 到网关 redeem。
	require.Contains(t, body, "fetch('/api/v1/carpools/19/web-session'")
	require.Contains(t, body, "session.gateway_origin + '/auth/sso/redeem'")
	require.Contains(t, body, "localStorage.getItem('auth_token')")
}

func TestWebSessionLaunchRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/carpools/:id/web-session/launch", webSessionLaunchHandler().Launch)

	req := httptest.NewRequest(http.MethodGet, "/carpools/abc/web-session/launch", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.False(t, strings.Contains(rec.Header().Get("Content-Type"), "text/html"))
}

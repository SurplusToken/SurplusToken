package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

// JWTAuthMiddleware JWT 认证中间件类型
type JWTAuthMiddleware gin.HandlerFunc

// AdminAuthMiddleware 管理员认证中间件类型
type AdminAuthMiddleware gin.HandlerFunc

// APIKeyAuthMiddleware API Key 认证中间件类型
type APIKeyAuthMiddleware gin.HandlerFunc

// ProvideAPIKeyAuthMiddleware 以固定 4 参数包装 NewAPIKeyAuthMiddleware，供 wire 注入。
// wire 不支持向变长参数 provider 注入依赖（会尝试为 []*service.SettingService 找
// provider 而失败），因此 NewAPIKeyAuthMiddleware 本身保留变长参数只为兼容现有
// 3 参数调用点（主要是测试），生产装配统一走这个固定参数的 wrapper。
func ProvideAPIKeyAuthMiddleware(apiKeyService *service.APIKeyService, subscriptionService *service.SubscriptionService, cfg *config.Config, settingService *service.SettingService) APIKeyAuthMiddleware {
	return NewAPIKeyAuthMiddleware(apiKeyService, subscriptionService, cfg, settingService)
}

// ProviderSet 中间件层的依赖注入
var ProviderSet = wire.NewSet(
	NewJWTAuthMiddleware,
	NewAdminAuthMiddleware,
	ProvideAPIKeyAuthMiddleware,
)

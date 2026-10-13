package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	// carpoolCreateMaxBodyBytes 创建车辆的请求体上限：群二维码解码后 ≤2MB，
	// base64 后约 2.7MB，留到 8MB 覆盖 data URL 前缀与其余字段。
	carpoolCreateMaxBodyBytes = 8 << 20
	// carpoolSmallMaxBodyBytes 是拼车里只带短文本的请求体上限。
	carpoolSmallMaxBodyBytes = 64 << 10
)

// RegisterUserRoutes 注册用户相关路由（需要认证）
func RegisterUserRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	apiKeyAuth middleware.APIKeyAuthMiddleware,
	settingService *service.SettingService,
	opsService *service.OpsService,
	cfg *config.Config,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
	// 面板全局按用户限流：防止单个账号高频刷接口打爆数据库
	authenticated.Use(panelRateLimiter.Global())
	// 用户管理面变更类操作入审计（含 TOTP 启用/禁用、step-up 验证、密码修改等安全事件）
	authenticated.Use(gin.HandlerFunc(auditLog))
	{
		// 用户接口
		user := authenticated.Group("/user")
		{
			user.GET("/profile", h.User.GetProfile)
			user.PUT("/password", h.User.ChangePassword)
			user.PUT("", h.User.UpdateProfile)
			user.GET("/aff", h.User.GetAffiliate)
			user.POST("/aff/transfer", h.User.TransferAffiliateQuota)
			// 用户贡献账号体系已下线（产品决定，2026-10）：贡献余额/提现端点不再注册，
			// ContributionService/Handler 源文件已删除。历史对账数据保留在 user_contributions 等表中。
			user.POST("/account-bindings/email/send-code", h.User.SendEmailBindingCode)
			user.POST("/account-bindings/email", h.User.BindEmailIdentity)
			user.DELETE("/account-bindings/:provider", h.User.UnbindIdentity)
			user.POST("/auth-identities/bind/start", h.User.StartIdentityBinding)
			user.GET("/api-keys/:id/usage/daily", panelRateLimiter.Heavy(), h.Usage.GetMyAPIKeyDailyUsage)
			user.GET("/platform-quotas", h.User.GetMyPlatformQuotas)
			user.GET("/sharing-rate-range", h.User.GetSharingRateRange)
			user.PUT("/sharing-rate-range", h.User.UpdateSharingRateRange)

			// 通知邮箱管理
			notifyEmail := user.Group("/notify-email")
			{
				notifyEmail.POST("/send-code", h.User.SendNotifyEmailCode)
				notifyEmail.POST("/verify", h.User.VerifyNotifyEmail)
				notifyEmail.PUT("/toggle", h.User.ToggleNotifyEmail)
				notifyEmail.DELETE("", h.User.RemoveNotifyEmail)
			}

			// TOTP 双因素认证
			totp := user.Group("/totp")
			{
				totp.GET("/status", h.Totp.GetStatus)
				totp.GET("/verification-method", h.Totp.GetVerificationMethod)
				totp.POST("/send-code", h.Totp.SendVerifyCode)
				totp.POST("/setup", h.Totp.InitiateSetup)
				totp.POST("/enable", h.Totp.Enable)
				totp.POST("/disable", h.Totp.Disable)
				// 敏感操作二次验证：授予当前会话一段时间的 step-up 权限
				totp.POST("/step-up", h.Totp.StepUp)
			}

			passkeys := user.Group("/passkeys")
			{
				passkeys.GET("", h.Passkey.List)
				passkeys.POST("/register/begin", h.Passkey.BeginRegistration)
				passkeys.POST("/register/finish", h.Passkey.FinishRegistration)
				passkeys.PATCH("/:id", h.Passkey.Rename)
				passkeys.DELETE("/:id", h.Passkey.Delete)
			}
		}

		// API Key管理
		keys := authenticated.Group("/keys")
		{
			keys.GET("", h.APIKey.List)
			keys.GET("/:id", h.APIKey.GetByID)
			keys.POST("", h.APIKey.Create)
			keys.PUT("/:id", h.APIKey.Update)
			keys.DELETE("/:id", h.APIKey.Delete)
		}

		// 聊天功能已全站下线（产品决定，2026-10）：/chat 路由组不再注册，
		// ChatHandler/ChatService/ChatRepository 源文件保留以便回滚。

		// 用户可用分组（非管理员接口）
		groups := authenticated.Group("/groups")
		{
			groups.GET("/available", h.APIKey.GetAvailableGroups)
			groups.GET("/rates", h.APIKey.GetUserGroupRates)
		}

		// 用户可用渠道（非管理员接口）
		channels := authenticated.Group("/channels")
		{
			channels.GET("/available", h.AvailableChannel.List)
		}

		// 用户侧 OAuth 账号池已整体下线（产品决定，2026-10）：/accounts 与
		// /scheduled-test-plans 路由组不再注册，任何调用一律 404。
		// AccountPoolHandler/AccountPoolService 源文件已删除；管理端 /admin/accounts
		// 与 /admin/scheduled-test-plans 不受影响。

		// 使用记录（聚合统计属重查询，叠加更严格的按用户限流）
		usage := authenticated.Group("/usage")
		usage.Use(panelRateLimiter.Heavy())
		{
			usage.GET("", h.Usage.List)
			usage.GET("/errors", h.Usage.ListErrors)
			usage.GET("/errors/:id", h.Usage.GetErrorDetail)
			usage.GET("/:id", h.Usage.GetByID)
			usage.GET("/stats", h.Usage.Stats)
			usage.GET("/leaderboard", h.Usage.Leaderboard)
			usage.GET("/leaderboard/trend", h.Usage.LeaderboardTrend)
			// User dashboard endpoints
			usage.GET("/dashboard/stats", h.Usage.DashboardStats)
			usage.GET("/dashboard/trend", h.Usage.DashboardTrend)
			usage.GET("/dashboard/models", h.Usage.DashboardModels)
			usage.GET("/dashboard/snapshot-v2", h.Usage.DashboardSnapshotV2)
			usage.POST("/dashboard/api-keys-usage", h.Usage.DashboardAPIKeysUsage)
		}

		// 公告（用户可见）
		announcements := authenticated.Group("/announcements")
		{
			announcements.GET("", h.Announcement.List)
			announcements.POST("/:id/read", h.Announcement.MarkRead)
		}

		// 卡密兑换
		redeem := authenticated.Group("/redeem")
		{
			redeem.POST("", h.Redeem.Redeem)
			redeem.GET("/history", h.Redeem.GetHistory)
		}

		// 用户订阅
		subscriptions := authenticated.Group("/subscriptions")
		{
			subscriptions.GET("", h.Subscription.List)
			subscriptions.GET("/carpool-usage", h.Subscription.ListCarpoolUsage)
			subscriptions.GET("/active", h.Subscription.GetActive)
			subscriptions.GET("/progress", h.Subscription.GetProgress)
			subscriptions.GET("/summary", h.Subscription.GetSummary)
		}

		carpools := authenticated.Group("/carpools")
		{
			carpools.GET("", h.Carpool.List)
			// 创建请求带 base64 群二维码（解码后上限 2MB ⇒ 编码后约 2.7MB）。
			// 没有路由级上限时会落到全局 256MB，且 base64 在解码前要全量拷进内存。
			carpools.POST("", middleware.RequestBodyLimit(carpoolCreateMaxBodyBytes), h.Carpool.Create)
			carpools.GET("/declaration-recommendation", h.Carpool.DeclarationRecommendation)
			carpools.GET("/pending-launch", h.Carpool.PendingLaunch)
			carpools.GET("/invites/:token", h.Carpool.ResolveInvite)
			carpools.POST("/join-by-invite", h.Carpool.JoinByInvite)
			carpools.POST("/custom-rule-interest", middleware.RequestBodyLimit(carpoolSmallMaxBodyBytes), h.Carpool.CustomRuleInterest)
			carpools.POST("/:id/invites", h.Carpool.CreateInvite)
			carpools.POST("/:id/join", h.Carpool.Join)
			carpools.POST("/:id/leave", h.Carpool.Leave)
			carpools.POST("/:id/confirm", h.Carpool.Confirm)
			carpools.POST("/:id/unconfirm", h.Carpool.Unconfirm)
			carpools.GET("/:id/qr-code", h.Carpool.GroupQRCode)
			carpools.PUT("/:id/qr-code", middleware.RequestBodyLimit(carpoolCreateMaxBodyBytes), h.Carpool.ReplaceGroupQRCode)
			carpools.GET("/:id/roster", h.Carpool.Roster)
			// 上游 cpr 账号周额度（服务端代理 + Redis 缓存；refresh 有服务端冷却）。
			carpools.GET("/:id/upstream-quota", h.UpstreamQuota.Get)
			carpools.POST("/:id/upstream-quota/refresh", h.UpstreamQuota.Refresh)
			// 本周用量摘要（订阅页同口径 + 全车合计，读自家库直出）。可见性与
			// upstream-quota 同一道闸（成员/admin，未上车者仅 public 且招募中可读）。
			carpools.GET("/:id/usage-summary", h.UsageSummary.Get)
			carpools.POST("/:id/launch", h.Carpool.Launch)
			carpools.GET("/:id/settlement", h.Carpool.Settlement)
			carpools.POST("/:id/settlement/settle", h.Carpool.Settle)
			carpools.POST("/:id/settlement/unsettle", h.Carpool.Unsettle)
			carpools.POST("/:id/cancel", h.Carpool.Cancel)
			carpools.PATCH("/:id/join-lock", h.Carpool.SetJoinLocked)
			// 管理端：总览与发车前管理，权限在 service 层按 isAdmin 判定。
			carpools.GET("/admin/overview", h.Carpool.AdminOverview)
			carpools.PATCH("/:id", h.Carpool.UpdateCarpool)
			carpools.POST("/:id/transfer-owner", h.Carpool.TransferOwner)
			carpools.POST("/:id/members", middleware.RequestBodyLimit(carpoolSmallMaxBodyBytes), h.Carpool.AddMember)
			carpools.POST("/:id/members/:userId/remove", h.Carpool.RemoveMember)
			carpools.PATCH("/:id/members/:userId/quota", h.Carpool.UpdateMemberQuota)
		}

		// 渠道监控（用户只读）
		monitors := authenticated.Group("/channel-monitors")
		{
			monitors.GET("", h.ChannelMonitor.List)
			monitors.GET("/:id/status", h.ChannelMonitor.GetStatus)
		}

		// V2 passive views require feature on + mode=v2.
		monitorV2 := authenticated.Group("/channel-monitor-v2")
		monitorV2.Use(panelRateLimiter.Heavy())
		monitorV2.Use(channelMonitorModeV2Guard(settingService))
		{
			monitorV2.GET("/dimensions", h.ChannelMonitorV2.Dimensions)
			monitorV2.GET("/snapshot", h.ChannelMonitorV2.Snapshot)
			monitorV2.GET("/models", h.ChannelMonitorV2.Models)
			monitorV2.GET("/matrix", h.ChannelMonitorV2.Matrix)
			monitorV2.GET("/errors", h.ChannelMonitorV2.Errors)
			monitorV2.GET("/users", h.ChannelMonitorV2.Users)
		}
	}
}

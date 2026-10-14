package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// d2wAccessGrantTTL 是网页版 Access 的授权时长：先定 30 天（desktop2web
// grant_expires_at_ms）。临近到期（剩余 < d2wAccessGrantRefreshThreshold）
// 时 ensureChatUIAccess 会按新 30 天窗口顺延续期。
const d2wAccessGrantTTL = 30 * 24 * time.Hour

var (
	// desktop2web 未配置（admin_base_url / admin_password / sso_private_key_file
	// 任一缺失）：功能禁用，惯例与 cpr 一致答 503 而不是 404。
	ErrCarpoolWebSessionNotConfigured = infraerrors.New(503, "DESKTOP2WEB_NOT_CONFIGURED", "desktop2web web session is not configured")
	// 车无账号 / 账号无邮箱 / slot 未配置：前端据此隐藏「打开网页版 ChatGPT」按钮。
	ErrCarpoolWebSessionNotProvisioned = infraerrors.Conflict("WEB_SESSION_NOT_CONFIGURED", "carpool web session is not provisioned for this account")
)

// CarpoolWebSessionAccount 是本车网页版会话解析出的上游账号信息（GetWebSessionAccount
// 的返回形态）。Email 为 credentials->>'email'；SlotID 为 extra 覆盖值，空串表示
// 走默认推导（邮箱 @ → -at-）。
type CarpoolWebSessionAccount struct {
	AccountID int64
	Email     string
	SlotID    string
}

// CarpoolWebSession 是 POST /carpools/:id/web-session 的载荷
// （handler 包一层 {"web_session": ...}）。GatewayOrigin 是兑换表单的 action 前缀。
type CarpoolWebSession struct {
	GatewayOrigin string `json:"gateway_origin"`
	Ticket        string `json:"ticket"`
}

// CarpoolWebSessionRepository 是 CarpoolWebSessionService 的最小依赖面，
// 由 *carpoolRepository 实现；窄接口让 service 测试免于 stub 整个 CarpoolRepository。
type CarpoolWebSessionRepository interface {
	// GetByID 加载车辆并填充查看者视角的 MemberRole（可见性判定与 GetRoster 同源）。
	GetByID(ctx context.Context, carpoolID, userID int64) (*Carpool, error)
	// GetWebSessionAccount 解析本车绑定的上游账号（主账号 + 组内回退，与
	// GetCprAccountID 同口径）。无可用账号返回 (nil, nil)。
	GetWebSessionAccount(ctx context.Context, carpoolID int64) (*CarpoolWebSessionAccount, error)
}

// CarpoolWebSessionUserDirectory 是查看者资料的最小依赖面（userRepository 实现），
// 用于 ticket displayName（用户昵称，缺省归一 "SurplusTokenLab"）。
type CarpoolWebSessionUserDirectory interface {
	GetByID(ctx context.Context, id int64) (*User, error)
}

// CarpoolWebSessionService 给拼车成员签发 desktop2web 网页版 ChatGPT 会话：
// 查车 → 可见性闸门（与 upstream-quota 同闸）→ 解析上游账号 → 推导 slot id →
// 幂等供给 Access（先读比再写，避免吊销在用的浏览器会话）→ 签 ES256 ticket。
type CarpoolWebSessionService struct {
	repo     CarpoolWebSessionRepository
	users    CarpoolWebSessionUserDirectory
	admin    *D2wAdminClient
	signer   *D2wTicketSigner
	now      func() time.Time
	grantTTL time.Duration
}

func NewCarpoolWebSessionService(repo CarpoolWebSessionRepository, users CarpoolWebSessionUserDirectory, admin *D2wAdminClient, signer *D2wTicketSigner) *CarpoolWebSessionService {
	return &CarpoolWebSessionService{
		repo:     repo,
		users:    users,
		admin:    admin,
		signer:   signer,
		now:      time.Now,
		grantTTL: d2wAccessGrantTTL,
	}
}

// Create 为查看者生成本车的网页版会话票据。未配置 503、非成员 403、
// 车无账号/账号无邮箱/slot 未配置 409；gateway 侧故障原样冒泡（500）。
//
// 权限比 upstream-quota 更严：额度信息允许「public 且招募中」的未上车者
// 查看（上车决策参考），但网页版直接消耗车上账号，必须真是成员或 admin——
// member_role 子查询只排除 status='left'，即只要还在车上（joined/active）。
func (s *CarpoolWebSessionService) Create(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*CarpoolWebSession, error) {
	if s.admin == nil || s.signer == nil {
		return nil, ErrCarpoolWebSessionNotConfigured
	}
	item, err := s.repo.GetByID(ctx, carpoolID, actorUserID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrCarpoolForbidden
	}
	if !isAdmin && item.MemberRole == nil &&
		(item.OwnerUserID == nil || *item.OwnerUserID != actorUserID) {
		return nil, ErrCarpoolForbidden
	}
	account, err := s.repo.GetWebSessionAccount(ctx, carpoolID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrCarpoolWebSessionNotProvisioned
	}
	slotID := account.SlotID
	if slotID == "" {
		email := strings.TrimSpace(account.Email)
		if email == "" {
			return nil, ErrCarpoolWebSessionNotProvisioned
		}
		if !strings.Contains(email, "@") {
			return nil, ErrCarpoolWebSessionNotProvisioned
		}
		slotID = strings.ReplaceAll(email, "@", "-at-")
	}
	// AccessID 按 (用户, slot) 稳定命名：同一成员重复打开复用同一条 Access，
	// 配合 EnsureChatUIAccess 的先读比再写，日常点击零吊销。
	userID := strconv.FormatInt(actorUserID, 10)
	accessID := "st-" + userID + "-" + slotID
	grantExpiresAtMs := s.now().Add(s.grantTTL).UnixMilli()
	if _, err := s.admin.EnsureChatUIAccess(ctx, accessID, userID, slotID, grantExpiresAtMs); err != nil {
		return nil, fmt.Errorf("desktop2web ensure access: %w", err)
	}
	ticket, err := s.signer.SignTicket(s.now(), userID, accessID, grantExpiresAtMs, s.displayName(ctx, actorUserID))
	if err != nil {
		return nil, err
	}
	return &CarpoolWebSession{GatewayOrigin: s.signer.PublicOrigin(), Ticket: ticket}, nil
}

// displayName 取查看者昵称（Username）；用户查不到或用户名为空时归一
// "SurplusTokenLab"（网关缺省值）。失败静默——展示名不是拒绝路径。
func (s *CarpoolWebSessionService) displayName(ctx context.Context, userID int64) string {
	if s.users == nil {
		return d2wDefaultDisplayName
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil || user == nil {
		return d2wDefaultDisplayName
	}
	return user.Username
}

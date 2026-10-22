package service

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

// desktop2web SSO 契约常量（与网关 packages/contracts/src/sso-ticket.ts 对齐）。
const (
	d2wTicketType         = "desktop2web-sso+jwt"
	d2wTicketAudience     = "desktop2web-gateway"
	d2wTicketVersion      = 1
	d2wTicketLifetime     = 60 * time.Second // iat=now-10s / exp=now+50s，两端各留 30s 时钟容差
	d2wDefaultDisplayName = "SurplusTokenLab"
)

// d2wTicketClaims 是 SSO ticket 的严格 claims 集合：网关按 strictObject 校验，
// 多一个键即拒，因此不嵌 jwt.RegisteredClaims（aud 会被 ClaimStrings 序列化成
// 数组，而网关要的是字符串字面量）。iat/exp 是整数秒（浮点会被拒），grantExp
// 是毫秒 epoch int64。实现 jwt.Claims 接口只为过 jwt.NewWithClaims 的类型约束，
// 签发侧不读这些 getter。
type d2wTicketClaims struct {
	Version     int    `json:"version"`
	Iss         string `json:"iss"`
	Aud         string `json:"aud"`
	Sub         string `json:"sub"`
	Jti         string `json:"jti"`
	Iat         int64  `json:"iat"`
	Exp         int64  `json:"exp"`
	AccessID    string `json:"accessId"`
	GrantExp    int64  `json:"grantExp"`
	DisplayName string `json:"displayName"`
}

func (c d2wTicketClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.Exp, 0)), nil
}

func (c d2wTicketClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.Iat, 0)), nil
}

func (c d2wTicketClaims) GetNotBefore() (*jwt.NumericDate, error) { return nil, nil }

func (c d2wTicketClaims) GetIssuer() (string, error) { return c.Iss, nil }

func (c d2wTicketClaims) GetSubject() (string, error) { return c.Sub, nil }

func (c d2wTicketClaims) GetAudience() (jwt.ClaimStrings, error) { return jwt.ClaimStrings{c.Aud}, nil }

// D2wTicketSigner signs desktop2web SSO tickets (ES256 JWS). The private key
// lives only on the issuer side; the gateway holds the public key and rejects
// anything whose kid/iss/claims deviate from the contract.
type D2wTicketSigner struct {
	issuer    string
	keyID     string
	publicURL string // 展示/兑换用 origin，原样回给前端
	key       *ecdsa.PrivateKey
}

// NewD2wTicketSigner loads the ES256 private key from config. Returns nil
// (feature disabled) when sso_private_key_file is empty; a present but
// unreadable/unparseable key is a configuration mistake — log and disable so
// the endpoint answers 503 instead of crashing unrelated requests.
func NewD2wTicketSigner(cfg *config.Config) *D2wTicketSigner {
	if cfg == nil {
		return nil
	}
	keyFile := strings.TrimSpace(cfg.Desktop2Web.SsoPrivateKeyFile)
	if keyFile == "" {
		return nil
	}
	pemBytes, err := os.ReadFile(keyFile)
	if err != nil {
		slog.Warn("desktop2web sso private key unreadable; web-session feature disabled",
			"key_file", keyFile, "error", err)
		return nil
	}
	// ParseECPrivateKeyFromPEM 同时吃 SEC1（EC PRIVATE KEY）与 PKCS8（PRIVATE KEY）。
	key, err := jwt.ParseECPrivateKeyFromPEM(pemBytes)
	if err != nil {
		slog.Warn("desktop2web sso private key not parseable; web-session feature disabled",
			"key_file", keyFile, "error", err)
		return nil
	}
	if key.Curve.Params().BitSize != 256 {
		slog.Warn("desktop2web sso private key is not P-256; web-session feature disabled",
			"key_file", keyFile, "bits", key.Curve.Params().BitSize)
		return nil
	}
	return &D2wTicketSigner{
		issuer:    strings.TrimSpace(cfg.Desktop2Web.SsoIssuer),
		keyID:     strings.TrimSpace(cfg.Desktop2Web.SsoKeyID),
		publicURL: strings.TrimRight(strings.TrimSpace(cfg.Desktop2Web.PublicOrigin), "/"),
		key:       key,
	}
}

// PublicOrigin 返回网关对外 origin（前端兑换表单的 action 前缀）。
func (s *D2wTicketSigner) PublicOrigin() string {
	if s == nil {
		return ""
	}
	return s.publicURL
}

// SignTicket mints one SSO ticket. now 由调用方传入（测试可注入）；
// sub=Access.user_id，grantExp=Access.grant_expires_at_ms，jti 为 16 字节 hex
// （首字符恒为字母数字，满足网关 ^[A-Za-z0-9][A-Za-z0-9._:@-]*$ 约束——base64url
// 可能以 -/_ 开头被拒）。
func (s *D2wTicketSigner) SignTicket(now time.Time, sub, accessID string, grantExpiresAtMs int64, displayName string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("desktop2web ticket signer not configured")
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = d2wDefaultDisplayName
	}
	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", fmt.Errorf("desktop2web generate jti: %w", err)
	}
	claims := d2wTicketClaims{
		Version:     d2wTicketVersion,
		Iss:         s.issuer,
		Aud:         d2wTicketAudience,
		Sub:         sub,
		Jti:         hex.EncodeToString(jtiBytes),
		Iat:         now.Add(-10 * time.Second).Unix(),
		Exp:         now.Add(50 * time.Second).Unix(),
		AccessID:    accessID,
		GrantExp:    grantExpiresAtMs,
		DisplayName: displayName,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = s.keyID
	token.Header["typ"] = d2wTicketType
	signed, err := token.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("desktop2web sign ticket: %w", err)
	}
	return signed, nil
}

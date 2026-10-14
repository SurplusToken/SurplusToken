//go:build unit

package service_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// writeD2wTestKey 生成一把 P-256 PKCS8 测试私钥落盘，返回配置文件用的 *config.Config。
func writeD2wTestKey(t *testing.T) (*config.Config, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	keyFile := filepath.Join(t.TempDir(), "d2w-test.pem")
	require.NoError(t, os.WriteFile(keyFile, pemBytes, 0o600))
	return &config.Config{
		Desktop2Web: config.Desktop2WebConfig{
			PublicOrigin:      "http://d2w.test:4310",
			SsoIssuer:         "https://surplustoken.test",
			SsoKeyID:          "test-key-1",
			SsoPrivateKeyFile: keyFile,
			AdminBaseURL:      "http://d2w.test:4310",
			AdminPassword:     "admin-secret",
		},
	}, key
}

// decodeD2wTicket 解开 JWS（不验签），返回 header 与 claims 的原始 map，
// 用于断言「claims 严格 10 键、header alg/typ/kid」。
func decodeD2wTicket(t *testing.T, ticket string) (map[string]any, map[string]any) {
	t.Helper()
	parts := splitJWS(ticket)
	require.Len(t, parts, 3)
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	claimsRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var header map[string]any
	require.NoError(t, json.Unmarshal(headerRaw, &header))
	var claims map[string]any
	require.NoError(t, json.Unmarshal(claimsRaw, &claims))
	return header, claims
}

func splitJWS(ticket string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(ticket); i++ {
		if ticket[i] == '.' {
			parts = append(parts, ticket[start:i])
			start = i + 1
		}
	}
	parts = append(parts, ticket[start:])
	return parts
}

func TestD2wTicketSignerDisabledWithoutKeyFile(t *testing.T) {
	require.Nil(t, service.NewD2wTicketSigner(nil))
	require.Nil(t, service.NewD2wTicketSigner(&config.Config{}))
}

func TestD2wTicketSignerDisabledOnUnreadableKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.Desktop2Web.SsoPrivateKeyFile = filepath.Join(t.TempDir(), "missing.pem")
	require.Nil(t, service.NewD2wTicketSigner(cfg))
}

func TestD2wTicketClaimsStrictShape(t *testing.T) {
	cfg, _ := writeD2wTestKey(t)
	signer := service.NewD2wTicketSigner(cfg)
	require.NotNil(t, signer)

	now := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	ticket, err := signer.SignTicket(now, "42", "st-42-bob-at-example.com", now.Add(30*24*time.Hour).UnixMilli(), "  bob  ")
	require.NoError(t, err)

	header, claims := decodeD2wTicket(t, ticket)
	require.Equal(t, "ES256", header["alg"])
	require.Equal(t, "desktop2web-sso+jwt", header["typ"])
	require.Equal(t, "test-key-1", header["kid"])

	// 网关 strictObject：多一个键都会被拒——集合必须恰好是这 10 个。
	wantKeys := []string{"version", "iss", "aud", "sub", "jti", "iat", "exp", "accessId", "grantExp", "displayName"}
	require.Len(t, claims, len(wantKeys))
	for _, key := range wantKeys {
		require.Contains(t, claims, key)
	}
	require.Equal(t, float64(1), claims["version"])
	require.Equal(t, "https://surplustoken.test", claims["iss"])
	require.Equal(t, "desktop2web-gateway", claims["aud"])
	require.Equal(t, "42", claims["sub"])
	require.Equal(t, "st-42-bob-at-example.com", claims["accessId"])
	require.Equal(t, "bob", claims["displayName"])

	// 时间戳为整数秒：iat=now-10、exp=now+50，exp-iat=60 ≤ 120。
	iat, ok := claims["iat"].(float64)
	require.True(t, ok)
	exp, ok := claims["exp"].(float64)
	require.True(t, ok)
	require.Equal(t, now.Add(-10*time.Second).Unix(), int64(iat))
	require.Equal(t, now.Add(50*time.Second).Unix(), int64(exp))
	require.Equal(t, float64(60), exp-iat)

	// jti 是 16 字节 hex：首字符恒为字母数字（base64url 可能以 -/_ 开头被网关拒）。
	jti, ok := claims["jti"].(string)
	require.True(t, ok)
	require.Len(t, jti, 32)
	_, err = hex.DecodeString(jti)
	require.NoError(t, err)

	// grantExp 毫秒 epoch，整数。
	grantExp, ok := claims["grantExp"].(float64)
	require.True(t, ok)
	require.Equal(t, now.Add(30*24*time.Hour).UnixMilli(), int64(grantExp))
}

func TestD2wTicketSignatureVerifiesWithPublicKey(t *testing.T) {
	cfg, key := writeD2wTestKey(t)
	signer := service.NewD2wTicketSigner(cfg)
	// 用真实当前时间：jwt.Parse 会按墙上时钟校验 exp。
	now := time.Now()
	ticket, err := signer.SignTicket(now, "7", "st-7-slot", now.Add(24*time.Hour).UnixMilli(), "")
	require.NoError(t, err)

	parsed, err := jwt.ParseWithClaims(ticket, jwt.MapClaims{}, func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	}, jwt.WithAudience("desktop2web-gateway"), jwt.WithIssuer("https://surplustoken.test"))
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	// displayName 空白时归一 SurplusTokenLab。
	claims := parsed.Claims.(jwt.MapClaims)
	require.Equal(t, "SurplusTokenLab", claims["displayName"])
}

func TestD2wTicketJtiUniquePerCall(t *testing.T) {
	cfg, _ := writeD2wTestKey(t)
	signer := service.NewD2wTicketSigner(cfg)
	now := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	seen := map[string]struct{}{}
	for i := 0; i < 32; i++ {
		ticket, err := signer.SignTicket(now, "1", "st-1-s", now.UnixMilli(), "n")
		require.NoError(t, err)
		_, claims := decodeD2wTicket(t, ticket)
		jti := claims["jti"].(string)
		require.NotContains(t, seen, jti, "jti 单次消费，重复即事故")
		seen[jti] = struct{}{}
	}
}

// d2wGatewayFixture 是一个假 desktop2web 网关：登录发 cookie、accesses 列表、PUT upsert。
type d2wGatewayFixture struct {
	srv           *httptest.Server
	loginCalls    *atomic.Int32
	listCalls     *atomic.Int32
	putCalls      *atomic.Int32
	deleteCalls   *atomic.Int32
	lastPutBody   atomic.Value // string
	lastPutOrigin atomic.Value // string
	lastDeletePath atomic.Value // string
	accesses      atomic.Value // []service.D2wAccess
	putErr        atomic.Value // string：非空时 PUT 返回 500
	cookieChecks  *atomic.Int32 // 携带正确 cookie 的请求数
}

func newD2wGatewayFixture(t *testing.T) *d2wGatewayFixture {
	f := &d2wGatewayFixture{
		loginCalls:   &atomic.Int32{},
		deleteCalls:  &atomic.Int32{},
		listCalls:    &atomic.Int32{},
		putCalls:     &atomic.Int32{},
		cookieChecks: &atomic.Int32{},
	}
	f.accesses.Store([]service.D2wAccess{})
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 除登录外都要求 admin cookie；没带的一律 401 ADMIN_UNAUTHORIZED。
		if r.URL.Path != "/__backend/admin/auth/v1/login" {
			if cookie, err := r.Cookie("desktop2web_admin_session"); err != nil || cookie.Value != "session-token" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"ADMIN_UNAUTHORIZED"}`))
				return
			}
			f.cookieChecks.Add(1)
		}
		switch r.URL.Path {
		case "/__backend/admin/auth/v1/login":
			f.loginCalls.Add(1)
			var body struct {
				Password string `json:"password"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if r.Header.Get("Origin") != "http://d2w.test:4310" || body.Password != "admin-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"ADMIN_UNAUTHORIZED"}`))
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "desktop2web_admin_session", Value: "session-token", Path: "/"})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"schemaVersion":1,"authenticated":true}`))
		case "/__backend/admin/v1/accesses":
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			f.listCalls.Add(1)
			payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "accesses": f.accesses.Load()})
			_, _ = w.Write(payload)
		default:
			// DELETE /__backend/admin/v1/accesses/:accessId
			if r.Method == http.MethodDelete {
				f.deleteCalls.Add(1)
				f.lastDeletePath.Store(r.URL.Path)
				id := filepath.Base(r.URL.Path)
				list := f.accesses.Load().([]service.D2wAccess)
				kept := list[:0]
				for _, item := range list {
					if item.AccessID != id {
						kept = append(kept, item)
					}
				}
				f.accesses.Store(kept)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			// PUT /__backend/admin/v1/accesses/:accessId
			if r.Method == http.MethodPut {
				f.putCalls.Add(1)
				f.lastPutOrigin.Store(r.Header.Get("Origin"))
				buf, err := io.ReadAll(r.Body)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				f.lastPutBody.Store(string(buf))
				if msg, _ := f.putErr.Load().(string); msg != "" {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
					return
				}
				var access service.D2wAccess
				if err := json.Unmarshal(buf, &access); err == nil {
					list := f.accesses.Load().([]service.D2wAccess)
					replaced := false
					for i := range list {
						if list[i].AccessID == access.AccessID {
							list[i] = access
							replaced = true
						}
					}
					if !replaced {
						access.AccessID = filepath.Base(r.URL.Path)
						list = append(list, access)
					}
					f.accesses.Store(list)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *d2wGatewayFixture) cfg() *config.Config {
	return &config.Config{
		Desktop2Web: config.Desktop2WebConfig{
			PublicOrigin:  "http://d2w.test:4310",
			SsoIssuer:     "https://surplustoken.test",
			SsoKeyID:      "test-key-1",
			AdminBaseURL:  f.srv.URL,
			AdminPassword: "admin-secret",
		},
	}
}

func TestD2wAdminClientLoginAndEnsureAccess(t *testing.T) {
	fixture := newD2wGatewayFixture(t)
	client := service.NewD2wAdminClient(fixture.cfg())
	require.NotNil(t, client)

	ctx := context.Background()
	grant := time.Now().Add(30 * 24 * time.Hour).UnixMilli()

	// 第一次：记录不存在 → PUT 创建。
	wrote, err := client.EnsureChatUIAccess(ctx, "st-9-bob-at-example.com", "9", "bob-at-example.com", grant)
	require.NoError(t, err)
	require.True(t, wrote)
	require.Equal(t, int32(1), fixture.putCalls.Load())
	require.Equal(t, int32(1), fixture.loginCalls.Load(), "首次请求应触发一次登录拿 cookie")
	require.Equal(t, "http://d2w.test:4310", fixture.lastPutOrigin.Load(), "写操作必须带精确 Origin 头")
	require.NotContains(t, fixture.lastPutBody.Load().(string), "accessId", "网关 upsertAccessInputSchema 是 strictObject(omit accessId)，多键即 400")

	// 第二次：字段完全相同（grant 未临近到期）→ 跳过，绝不吊销在用的会话。
	wrote, err = client.EnsureChatUIAccess(ctx, "st-9-bob-at-example.com", "9", "bob-at-example.com", time.Now().Add(30*24*time.Hour).UnixMilli())
	require.NoError(t, err)
	require.False(t, wrote, "幂等比对通过时不允许 PUT（PUT 会吊销全部活跃会话）")
	require.Equal(t, int32(1), fixture.putCalls.Load())

	// grant 即将到期（< 24h）→ 视为有变化，PUT 续期。
	old := fixture.accesses.Load().([]service.D2wAccess)
	old[0].GrantExpiresAtMs = time.Now().Add(2 * time.Hour).UnixMilli()
	fixture.accesses.Store(old)
	wrote, err = client.EnsureChatUIAccess(ctx, "st-9-bob-at-example.com", "9", "bob-at-example.com", grant)
	require.NoError(t, err)
	require.True(t, wrote, "授权临近到期应顺延续期")
	require.Equal(t, int32(2), fixture.putCalls.Load())

	// slot 变了 → PUT（字段变化路径）。
	wrote, err = client.EnsureChatUIAccess(ctx, "st-9-bob-at-example.com", "9", "alice-at-example.com", grant)
	require.NoError(t, err)
	require.True(t, wrote)
	require.Equal(t, int32(3), fixture.putCalls.Load())
}

func TestD2wAdminClientReloginOn401(t *testing.T) {
	fixture := newD2wGatewayFixture(t)
	client := service.NewD2wAdminClient(fixture.cfg())
	require.NotNil(t, client)
	ctx := context.Background()

	//  cookie jar 是空的 → 第一次 ListAccesses 401 → 自动登录 → 重试成功。
	accesses, err := client.ListAccesses(ctx)
	require.NoError(t, err)
	require.NotNil(t, accesses)
	require.Equal(t, int32(1), fixture.loginCalls.Load())
	require.GreaterOrEqual(t, fixture.cookieChecks.Load(), int32(1))
}

func TestD2wAdminClientDisabledWithoutConfig(t *testing.T) {
	require.Nil(t, service.NewD2wAdminClient(nil))
	require.Nil(t, service.NewD2wAdminClient(&config.Config{}))
	cfg := &config.Config{}
	cfg.Desktop2Web.AdminBaseURL = "http://d2w.test:4310"
	require.Nil(t, service.NewD2wAdminClient(cfg), "缺密码必须禁用而不是空密码登录")
}

func TestD2wAdminClientSurfacesGatewayError(t *testing.T) {
	fixture := newD2wGatewayFixture(t)
	fixture.putErr.Store("ADMIN_RUNTIME_ACTION_FAILED")
	client := service.NewD2wAdminClient(fixture.cfg())
	ctx := context.Background()
	_, err := client.EnsureChatUIAccess(ctx, "st-1-s", "1", "s", time.Now().Add(time.Hour).UnixMilli())
	require.Error(t, err)
	require.Contains(t, err.Error(), "ADMIN_RUNTIME_ACTION_FAILED")
}

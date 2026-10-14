package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// D2wAdminClient talks to the desktop2web gateway admin API (prefix
// /__backend/admin/v1, auth = admin session cookie). Write operations (and the
// login that mints the cookie) require the Origin header to exactly equal the
// gateway's PUBLIC_ORIGIN — the gateway refuses otherwise (401
// ADMIN_UNAUTHORIZED), and the reverse proxy must not rewrite Origin.
//
// Session lifecycle: the cookie jar carries the admin cookie; any 401 from the
// gateway triggers one re-login and a single retry of the original request.
// The cookie lasts 12h server-side, so steady-state traffic rarely logs in.
type D2wAdminClient struct {
	adminBaseURL string
	password     string
	origin       string
	httpClient   *http.Client

	// loginMu serializes re-login so a burst of 401s logs in once, not per caller.
	loginMu sync.Mutex
}

// d2wAdminLoginPath is the admin login route (Origin-gated, mints the session cookie).
const d2wAdminLoginPath = "/__backend/admin/auth/v1/login"

// D2wAccess 是 desktop2web Access 记录（access_id → user/slot/role/grant）。
// 字段名与网关 accessSummarySchema 对齐（camelCase）。
type D2wAccess struct {
	AccessID         string `json:"accessId"`
	UserID           string `json:"userId"`
	AccountSlotID    string `json:"accountSlotId"`
	Role             string `json:"role"`
	GrantExpiresAtMs int64  `json:"grantExpiresAtMs"`
}

// d2wAccessUpsert 是 PUT /accesses/:id 的请求体：网关 upsertAccessInputSchema
// 由 strictObject omit(accessId) 得出，多带一个键（哪怕是空 accessId）都会被拒。
type d2wAccessUpsert struct {
	UserID           string `json:"userId"`
	AccountSlotID    string `json:"accountSlotId"`
	Role             string `json:"role"`
	GrantExpiresAtMs int64  `json:"grantExpiresAtMs"`
}

// d2wAccessGrantRefreshThreshold：存量 Access 的授权剩余不足该阈值时视为
// "即将过期"，ensureChatUIAccess 会顺手续到 now+d2wAccessGrantTTL。阈值设 24h：
// 远低于 30 天授权期，又不会因时钟抖动把刚签的记录反复重写（每次 PUT 都会
// 吊销该 Access 下全部活跃会话，见 production.md §4）。
const d2wAccessGrantRefreshThreshold = 24 * time.Hour

// NewD2wAdminClient builds the desktop2web admin client from config. Returns
// nil (feature disabled) when admin_base_url or admin_password is empty, so
// callers can answer 503 DESKTOP2WEB_NOT_CONFIGURED like the cpr feature does.
func NewD2wAdminClient(cfg *config.Config) *D2wAdminClient {
	if cfg == nil {
		return nil
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.Desktop2Web.AdminBaseURL), "/")
	password := strings.TrimSpace(cfg.Desktop2Web.AdminPassword)
	if base == "" || password == "" {
		return nil
	}
	jar, _ := cookiejar.New(nil)
	return &D2wAdminClient{
		adminBaseURL: base,
		password:     password,
		origin:       strings.TrimSpace(cfg.Desktop2Web.PublicOrigin),
		httpClient:   &http.Client{Timeout: 30 * time.Second, Jar: jar},
	}
}

// d2wErrorEnvelope is the gateway's {"error": "CODE"} error body (admin-api
// sendError sends the code as a plain string; the SSO redeem route uses the
// same shape). Tolerate an object form too so a contract drift degrades to
// the HTTP status instead of a confusing empty code.
type d2wErrorEnvelope struct {
	Error json.RawMessage `json:"error"`
}

func (e *d2wErrorEnvelope) code() string {
	if e == nil || len(e.Error) == 0 {
		return ""
	}
	var code string
	if err := json.Unmarshal(e.Error, &code); err == nil {
		return code
	}
	var obj struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(e.Error, &obj); err == nil {
		return obj.Code
	}
	return ""
}

// do sends one admin-API request. forceOrigin adds the exact PUBLIC_ORIGIN
// Origin header (login + write ops). A 401 ADMIN_UNAUTHORIZED re-logins once
// and retries; login failure (bad password / Origin mismatch) is returned as
// an error so the endpoint degrades instead of looping.
func (c *D2wAdminClient) do(ctx context.Context, method, path string, body any, forceOrigin bool, out any) error {
	if c == nil {
		return fmt.Errorf("desktop2web admin client not configured")
	}
	exec := func() (*http.Response, []byte, error) {
		var reader io.Reader
		if body != nil {
			encoded, err := json.Marshal(body)
			if err != nil {
				return nil, nil, fmt.Errorf("desktop2web marshal %s: %w", path, err)
			}
			reader = bytes.NewReader(encoded)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.adminBaseURL+path, reader)
		if err != nil {
			return nil, nil, fmt.Errorf("desktop2web new request %s: %w", path, err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if forceOrigin {
			req.Header.Set("Origin", c.origin)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("desktop2web do %s: %w", path, err)
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("desktop2web read %s: %w", path, err)
		}
		return resp, raw, nil
	}

	resp, raw, err := exec()
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized && path != d2wAdminLoginPath {
		if loginErr := c.login(ctx); loginErr != nil {
			return fmt.Errorf("desktop2web re-login after 401 %s: %w", path, loginErr)
		}
		resp, raw, err = exec()
		if err != nil {
			return err
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var envelope d2wErrorEnvelope
		_ = json.Unmarshal(raw, &envelope)
		code := envelope.code()
		if code == "" {
			code = fmt.Sprintf("HTTP_%d", resp.StatusCode)
		}
		return fmt.Errorf("desktop2web %s: %s", path, code)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("desktop2web decode %s: %w", path, err)
		}
	}
	return nil
}

// login POSTs the admin password (Origin-gated) and lets the jar keep the
// session cookie. loginMu serializes concurrent re-logins.
func (c *D2wAdminClient) login(ctx context.Context) error {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	return c.do(ctx, http.MethodPost, d2wAdminLoginPath,
		map[string]any{"password": c.password}, true, nil)
}

// ListAccesses returns the gateway's full access list (GET, no Origin check).
func (c *D2wAdminClient) ListAccesses(ctx context.Context) ([]D2wAccess, error) {
	if c == nil {
		return nil, fmt.Errorf("desktop2web admin client not configured")
	}
	var out struct {
		Accesses []D2wAccess `json:"accesses"`
	}
	if err := c.do(ctx, http.MethodGet, "/__backend/admin/v1/accesses", nil, false, &out); err != nil {
		return nil, err
	}
	return out.Accesses, nil
}

// putAccess upserts one access (create-or-update). ANY field differing from the
// stored record revokes every active session under that access, so callers must
// read-compare-write via EnsureChatUIAccess instead of calling this directly.
func (c *D2wAdminClient) putAccess(ctx context.Context, accessID string, access D2wAccess) error {
	if c == nil {
		return fmt.Errorf("desktop2web admin client not configured")
	}
	path := "/__backend/admin/v1/accesses/" + accessID
	upsert := d2wAccessUpsert{
		UserID:           access.UserID,
		AccountSlotID:    access.AccountSlotID,
		Role:             access.Role,
		GrantExpiresAtMs: access.GrantExpiresAtMs,
	}
	return c.do(ctx, http.MethodPut, path, upsert, true, nil)
}

// EnsureChatUIAccess 维护一条 chat-ui Access 的幂等供给：先 GET 列表比对，
// 完全相同（且授权远未到期）直接跳过——绝不做"每次都 PUT"的事，因为网关对
// 任一字段变化的 upsert 都会吊销该 Access 下全部活跃会话。返回 wrote=true
// 表示本次发生了 PUT（新建或续期）。
func (c *D2wAdminClient) EnsureChatUIAccess(ctx context.Context, accessID, userID, accountSlotID string, grantExpiresAtMs int64) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("desktop2web admin client not configured")
	}
	accesses, err := c.ListAccesses(ctx)
	if err != nil {
		return false, err
	}
	desired := D2wAccess{
		UserID:           userID,
		AccountSlotID:    accountSlotID,
		Role:             "chat-ui",
		GrantExpiresAtMs: grantExpiresAtMs,
	}
	nowMs := time.Now().UnixMilli()
	for i := range accesses {
		item := &accesses[i]
		if item.AccessID != accessID {
			continue
		}
		// 字段一致且授权剩余 > 阈值：跳过（含早退的"刚签过"路径）。
		if item.UserID == desired.UserID &&
			item.AccountSlotID == desired.AccountSlotID &&
			item.Role == desired.Role &&
			item.GrantExpiresAtMs > nowMs+int64(d2wAccessGrantRefreshThreshold/time.Millisecond) {
			return false, nil
		}
		break
	}
	if err := c.putAccess(ctx, accessID, desired); err != nil {
		return false, err
	}
	return true, nil
}

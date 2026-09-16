package service

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

// Desktop2Web SSO ticket contract. These mirror desktop2web
// packages/contracts/src/sso-ticket.ts and are validated for exact equality by the
// gateway on every redemption, so they are protocol constants rather than tunables.
const (
	desktop2WebTicketVersion  = 1
	desktop2WebTicketAudience = "desktop2web-gateway"
	desktop2WebTicketType     = "desktop2web-sso+jwt"

	// desktop2WebTicketMaxLifetime is the gateway's hard ceiling (exp - iat <= 120s).
	desktop2WebTicketMaxLifetime = 120 * time.Second
	// desktop2WebTicketLifetime is what we actually ask for. Deliberately below the
	// ceiling so ordinary clock skew between this service and the gateway cannot push
	// a ticket past maxTokenAge; the gateway also rejects an `iat` more than 30s ahead.
	desktop2WebTicketLifetime = 90 * time.Second
)

// Desktop2Web gateway endpoints.
const (
	desktop2WebRedeemPath      = "/auth/sso/redeem"
	desktop2WebAdminLoginPath  = "/__backend/admin/auth/v1/login"
	desktop2WebAdminAPIPrefix  = "/__backend/admin/v1"
	desktop2WebAdminCookieName = "desktop2web_admin_session"
	// desktop2WebAdminSessionTTL is how long a cached admin cookie is reused. The
	// gateway issues 12h sessions; re-login a little early rather than discovering
	// expiry mid-request.
	desktop2WebAdminSessionTTL = 11 * time.Hour
	desktop2WebRequestTimeout  = 15 * time.Second
	desktop2WebMaxResponseSize = 1 << 20
)

// Desktop2Web remote-browser defaults.
const (
	desktop2WebDefaultRole     = "chat-ui"
	desktop2WebDefaultGrantTTL = 12 * time.Hour

	// desktop2WebUserPrefix namespaces ticket subjects and Access ids so they cannot
	// collide with identities the gateway creates for its own admin console.
	desktop2WebUserPrefix = "surplus-u"

	// desktop2WebMaxStableIDLength is the gateway's limit for userId/accessId/jti.
	desktop2WebMaxStableIDLength = 128
)

// accountSlotIDPattern is the gateway's accountSlotIdSchema. A slot id that does not
// match it can never be addressed.
var accountSlotIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// stableIDPattern is the gateway's stableIdSchema, applied to userId/accessId/jti.
var stableIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@-]*$`)

// Desktop2WebClient provisions Accesses on a Desktop2Web gateway and mints the
// single-use ES256 SSO tickets that carry a user into a browser session.
//
// There is no service credential on the gateway side: every admin call authenticates
// with the admin password and, for writes, an Origin header equal to the gateway's
// configured public origin (which must therefore equal cfg.BaseURL).
type Desktop2WebClient struct {
	baseURL       string
	issuer        string
	keyID         string
	role          string
	grantTTL      time.Duration
	adminPassword string
	privateKey    *ecdsa.PrivateKey
	httpClient    *http.Client

	// configErr records a misconfiguration detected at construction (base_url set but
	// unusable key material). The feature stays "enabled" so the failure surfaces as an
	// explicit error instead of silently disabling the button.
	configErr error

	mu          sync.Mutex
	adminCookie string
	adminExpiry time.Time
}

// NewDesktop2WebClient builds the client from config. Returns nil (feature disabled)
// when desktop2web.base_url is empty. Key material is loaded eagerly so a broken
// deployment fails on the first connect with a specific error rather than a confusing
// gateway rejection.
func NewDesktop2WebClient(cfg *config.Config) *Desktop2WebClient {
	if cfg == nil {
		return nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.Desktop2Web.BaseURL), "/")
	if baseURL == "" {
		return nil
	}

	client := &Desktop2WebClient{
		baseURL:       baseURL,
		issuer:        strings.TrimSpace(cfg.Desktop2Web.Issuer),
		keyID:         strings.TrimSpace(cfg.Desktop2Web.KeyID),
		role:          strings.TrimSpace(cfg.Desktop2Web.Role),
		adminPassword: strings.TrimSpace(cfg.Desktop2Web.AdminPassword),
		httpClient:    &http.Client{Timeout: desktop2WebRequestTimeout},
	}
	if client.role == "" {
		client.role = desktop2WebDefaultRole
	}
	client.grantTTL = desktop2WebDefaultGrantTTL
	if cfg.Desktop2Web.GrantTTLSeconds > 0 {
		client.grantTTL = time.Duration(cfg.Desktop2Web.GrantTTLSeconds) * time.Second
	}

	switch {
	case client.issuer == "":
		client.configErr = fmt.Errorf("desktop2web.issuer is not configured")
	case client.keyID == "":
		client.configErr = fmt.Errorf("desktop2web.key_id is not configured")
	case client.adminPassword == "":
		client.configErr = fmt.Errorf("desktop2web.admin_password is not configured")
	case client.role != "chat-ui" && client.role != "chat-isolated":
		client.configErr = fmt.Errorf("desktop2web.role must be chat-ui or chat-isolated, got %q", client.role)
	default:
		key, err := loadECPrivateKey(strings.TrimSpace(cfg.Desktop2Web.PrivateKeyFile))
		if err != nil {
			client.configErr = err
		} else {
			client.privateKey = key
		}
	}
	return client
}

// Enabled reports whether the remote-browser hand-off is configured.
func (c *Desktop2WebClient) Enabled() bool { return c != nil }

// Configured reports whether the client has usable key material and credentials.
func (c *Desktop2WebClient) Configured() bool {
	return c != nil && c.configErr == nil && c.privateKey != nil
}

// RedeemURL is the gateway endpoint the browser form-POSTs a ticket to.
func (c *Desktop2WebClient) RedeemURL() string {
	if c == nil {
		return ""
	}
	return c.baseURL + desktop2WebRedeemPath
}

// loadECPrivateKey reads a PEM EC P-256 private key (PKCS#8 or SEC1).
func loadECPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	if path == "" {
		return nil, fmt.Errorf("desktop2web.private_key_file is not configured")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read desktop2web private key: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("desktop2web private key %s is not PEM encoded", path)
	}

	var key *ecdsa.PrivateKey
	switch block.Type {
	case "PRIVATE KEY":
		parsed, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if parseErr != nil {
			return nil, fmt.Errorf("parse desktop2web PKCS#8 key: %w", parseErr)
		}
		ecKey, ok := parsed.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("desktop2web private key must be an EC key, got %T", parsed)
		}
		key = ecKey
	case "EC PRIVATE KEY":
		parsed, parseErr := x509.ParseECPrivateKey(block.Bytes)
		if parseErr != nil {
			return nil, fmt.Errorf("parse desktop2web SEC1 key: %w", parseErr)
		}
		key = parsed
	default:
		return nil, fmt.Errorf("desktop2web private key has unsupported PEM type %q", block.Type)
	}

	// ES256 is fixed to P-256; a larger curve would produce a JWS the gateway rejects.
	if key.Curve.Params().BitSize != 256 {
		return nil, fmt.Errorf("desktop2web private key must use P-256, got %d bits", key.Curve.Params().BitSize)
	}
	return key, nil
}

// accountSlotID returns the gateway account slot for an account: the explicit
// accounts.extra.desktop2web_slot_id override when present, otherwise the deployment
// convention of the account's email with "@" rewritten to "-at-"
// ("lyucmu@gmail.com" -> "lyucmu-at-gmail.com").
//
// The convention is ours, not the gateway's — Desktop2Web has no slot-id derivation
// and accepts any operator-chosen id — so an id that does not address a registered
// slot is reported by EnsureAccountSlot rather than assumed.
func (c *Desktop2WebClient) accountSlotID(account *Account) (string, error) {
	if override := strings.TrimSpace(account.GetExtraString(desktop2WebSlotIDExtraKey)); override != "" {
		if !accountSlotIDPattern.MatchString(override) {
			return "", ErrRemoteSessionSlotInvalid
		}
		return override, nil
	}

	email := strings.ToLower(strings.TrimSpace(account.GetCredential("email")))
	if email == "" {
		return "", ErrRemoteSessionSlotUnmapped
	}
	candidate := strings.ReplaceAll(email, "@", "-at-")
	if !accountSlotIDPattern.MatchString(candidate) {
		return "", ErrRemoteSessionSlotUnmapped
	}
	return candidate, nil
}

// EnsureAccountSlot verifies the slot exists on the gateway and is enabled. Access
// provisioning does not check this, so without the pre-flight a user would receive a
// ticket that only fails at redemption with an opaque ACCESS_UNAVAILABLE.
func (c *Desktop2WebClient) EnsureAccountSlot(ctx context.Context, slotID string) error {
	if err := c.usable(); err != nil {
		return err
	}
	var list struct {
		Accounts []struct {
			AccountSlotID string `json:"accountSlotId"`
			Enabled       bool   `json:"enabled"`
		} `json:"accounts"`
	}
	if err := c.adminJSON(ctx, http.MethodGet, desktop2WebAdminAPIPrefix+"/accounts", nil, &list); err != nil {
		return err
	}
	for _, slot := range list.Accounts {
		if slot.AccountSlotID != slotID {
			continue
		}
		if !slot.Enabled {
			return ErrRemoteSessionSlotDisabled
		}
		return nil
	}
	return ErrRemoteSessionSlotUnmapped
}

// EnsureAccess creates or updates the Access that authorizes one user for one slot.
//
// The gateway treats this route as upsert, but any difference from the stored row
// revokes that Access's live sessions, so it is called only when a ticket is about to
// be minted and never as a background refresh.
func (c *Desktop2WebClient) EnsureAccess(ctx context.Context, accessID, userID, slotID string, grantExpiresAtMs int64) error {
	if err := c.usable(); err != nil {
		return err
	}
	body := map[string]any{
		"userId":           userID,
		"accountSlotId":    slotID,
		"role":             c.role,
		"grantExpiresAtMs": grantExpiresAtMs,
	}
	path := desktop2WebAdminAPIPrefix + "/accesses/" + accessID
	return c.adminJSON(ctx, http.MethodPut, path, body, nil)
}

// MintTicket returns a signed ES256 SSO ticket for an existing Access.
func (c *Desktop2WebClient) MintTicket(userID, accessID, slotID string, grantExpiresAtMs int64) (string, error) {
	if err := c.usable(); err != nil {
		return "", err
	}
	if !stableIDPattern.MatchString(userID) || len(userID) > desktop2WebMaxStableIDLength {
		return "", fmt.Errorf("desktop2web user id %q is not a valid stable id", userID)
	}
	if !stableIDPattern.MatchString(accessID) || len(accessID) > desktop2WebMaxStableIDLength {
		return "", fmt.Errorf("desktop2web access id %q is not a valid stable id", accessID)
	}
	// The gateway rejects any ticket whose lifetime exceeds its own ceiling, and that
	// rejection only shows up at redemption. Fail here instead, where the cause is named.
	if desktop2WebTicketLifetime <= 0 || desktop2WebTicketLifetime > desktop2WebTicketMaxLifetime {
		return "", fmt.Errorf(
			"desktop2web ticket lifetime %s is outside the gateway ceiling of %s",
			desktop2WebTicketLifetime, desktop2WebTicketMaxLifetime,
		)
	}

	now := time.Now()
	claims := desktop2WebTicketClaims{
		Version:   desktop2WebTicketVersion,
		Issuer:    c.issuer,
		Audience:  desktop2WebTicketAudience,
		Subject:   userID,
		JTI:       newTicketID(),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(desktop2WebTicketLifetime).Unix(),
		AccessID:  accessID,
		// The gateway reads grantExp in seconds and multiplies by 1000; the Access row
		// stores the same instant in milliseconds.
		GrantExp: grantExpiresAtMs / 1000,
	}
	_ = slotID // the slot is bound to the Access, not to the ticket
	return c.sign(claims)
}

// desktop2WebTicketClaims is the ticket payload. The gateway parses it with a strict
// schema, so the field set is contract: no extra key may appear, and every timestamp
// is an integer epoch in *seconds*. The JSON tags are the wire format; the jwt.Claims
// getters below only adapt the stored values and do not affect encoding.
type desktop2WebTicketClaims struct {
	Version   int    `json:"version"`
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	Subject   string `json:"sub"`
	JTI       string `json:"jti"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	AccessID  string `json:"accessId"`
	GrantExp  int64  `json:"grantExp"`
}

var _ jwt.Claims = desktop2WebTicketClaims{}

func (c desktop2WebTicketClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.ExpiresAt, 0)), nil
}

func (c desktop2WebTicketClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.IssuedAt, 0)), nil
}

func (c desktop2WebTicketClaims) GetNotBefore() (*jwt.NumericDate, error) { return nil, nil }

func (c desktop2WebTicketClaims) GetIssuer() (string, error) { return c.Issuer, nil }

func (c desktop2WebTicketClaims) GetSubject() (string, error) { return c.Subject, nil }

func (c desktop2WebTicketClaims) GetAudience() (jwt.ClaimStrings, error) {
	return jwt.ClaimStrings{c.Audience}, nil
}

// sign builds the compact JWS with golang-jwt, the same library the rest of the
// codebase signs JWTs with. Its ES256 signature is the raw R||S pair (RFC 7518) the
// gateway requires, and its header encoding is byte-identical to the fixed
// {alg,kid,typ} header the gateway pins.
func (c *Desktop2WebClient) sign(claims desktop2WebTicketClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["typ"] = desktop2WebTicketType
	token.Header["kid"] = c.keyID
	signed, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign desktop2web ticket: %w", err)
	}
	return signed, nil
}

// newTicketID returns a jti that satisfies the gateway's stable-id grammar. Hex is
// used rather than base64url because the grammar excludes "_".
func newTicketID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is unrecoverable; fall back to a time-derived id so the
		// caller still gets a syntactically valid (if less unique) ticket.
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

// AccessID is the stable, caller-chosen primary key of an Access. Scoping it to
// (user, account) keeps one row per pair, so a reconnect reuses the same Access.
func desktop2WebAccessID(userID, accountID int64) string {
	return fmt.Sprintf("surplus-u%d-a%d", userID, accountID)
}

// desktop2WebUserID namespaces the SurplusToken user id for the gateway.
func desktop2WebUserID(userID int64) string {
	return desktop2WebUserPrefix + strconv.FormatInt(userID, 10)
}

// usable reports whether the client can sign and authenticate.
func (c *Desktop2WebClient) usable() error {
	if c == nil {
		return ErrRemoteSessionNotConfigured
	}
	if c.configErr != nil {
		return ErrRemoteSessionMisconfigured.WithCause(c.configErr)
	}
	return nil
}

// adminJSON performs an authenticated admin call, logging in on first use and
// re-authenticating once if the cached cookie has gone stale.
func (c *Desktop2WebClient) adminJSON(ctx context.Context, method, path string, body, out any) error {
	cookie, err := c.adminSession(ctx, false)
	if err != nil {
		return err
	}
	status, payload, err := c.do(ctx, method, path, body, cookie)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized {
		cookie, err = c.adminSession(ctx, true)
		if err != nil {
			return err
		}
		if status, payload, err = c.do(ctx, method, path, body, cookie); err != nil {
			return err
		}
	}
	if status < 200 || status >= 300 {
		return ErrRemoteSessionUpstream.WithMetadata(map[string]string{
			"status": strconv.Itoa(status),
			"body":   truncateForError(string(payload)),
		})
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return ErrRemoteSessionUpstream.WithCause(err)
	}
	return nil
}

// adminSession returns a cached admin cookie, logging in when absent, stale, or when
// force is set (the retry path after a 401).
func (c *Desktop2WebClient) adminSession(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.adminCookie != "" && time.Now().Before(c.adminExpiry) {
		return c.adminCookie, nil
	}

	body, err := json.Marshal(map[string]string{"password": c.adminPassword})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+desktop2WebAdminLoginPath, bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	// The login route compares Origin against the gateway's configured public origin.
	request.Header.Set("Origin", c.baseURL)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", ErrRemoteSessionUpstream.WithCause(err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, desktop2WebMaxResponseSize))

	if response.StatusCode != http.StatusOK {
		return "", ErrRemoteSessionUpstream.WithMetadata(map[string]string{
			"status": strconv.Itoa(response.StatusCode),
			"stage":  "admin-login",
		})
	}

	token := ""
	for _, candidate := range response.Cookies() {
		if candidate.Name == desktop2WebAdminCookieName {
			token = candidate.Value
			break
		}
	}
	if token == "" {
		// Login returned 200 but no session cookie: treat as a configuration error
		// rather than caching an empty cookie and retrying forever.
		return "", ErrRemoteSessionMisconfigured.WithCause(
			fmt.Errorf("desktop2web admin login returned no %s cookie", desktop2WebAdminCookieName),
		)
	}

	c.adminCookie = token
	c.adminExpiry = time.Now().Add(desktop2WebAdminSessionTTL)
	return token, nil
}

// do issues one admin request and returns the status and (bounded) body.
func (c *Desktop2WebClient) do(ctx context.Context, method, path string, body any, cookie string) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		// Writes are gated on the Origin header matching the public origin.
		request.Header.Set("Origin", c.baseURL)
	}
	request.AddCookie(&http.Cookie{Name: desktop2WebAdminCookieName, Value: cookie})

	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, nil, ErrRemoteSessionUpstream.WithCause(err)
	}
	defer func() { _ = response.Body.Close() }()
	payload, err := io.ReadAll(io.LimitReader(response.Body, desktop2WebMaxResponseSize))
	if err != nil {
		return 0, nil, ErrRemoteSessionUpstream.WithCause(err)
	}
	return response.StatusCode, payload, nil
}

// truncateForError bounds an upstream body before it reaches a log line or response.
func truncateForError(body string) string {
	const limit = 512
	body = strings.TrimSpace(body)
	if len(body) <= limit {
		return body
	}
	return body[:limit]
}

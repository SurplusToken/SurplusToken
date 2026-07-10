package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// KasmClient talks to a Kasm Workspaces "developer API" (the /api/public/<method>
// endpoints). Every call POSTs JSON including api_key + api_key_secret.
//
// The base URL (cfg.Kasm.APIBase) is reached over Tailscale by IP, so its TLS
// certificate CN does not match — the dedicated *http.Client here skips TLS
// verification ONLY for this client (the shared httpclient pool refuses to skip
// verification by policy). The user-facing connect URL is rewritten to the public
// host (cfg.Kasm.PublicHost) since end-user browsers cannot reach the Tailscale IP.
type KasmClient struct {
	apiBase    string
	apiKey     string
	apiSecret  string
	imageID    string
	publicHost string
	// seedNamespace ("prod"/"staging"/"") isolates seed dirs and Kasm users per
	// environment so deployments sharing one Kasm don't share login state.
	seedNamespace string
	httpClient    *http.Client
}

// KasmSession is one live Kasm session as returned by get_kasms.
type KasmSession struct {
	KasmID            string         `json:"kasm_id"`
	UserID            string         `json:"user_id"`
	Username          string         `json:"username"`
	ImageID           string         `json:"image_id"`
	OperationalStatus string         `json:"operational_status"`
	ConnectionInfo    map[string]any `json:"connection_info"`
	KeepaliveDate     string         `json:"keepalive_date"`
}

// Connected reports whether a streaming client is currently attached to the
// session. Kasm populates connection_info while a viewer is connected and resets
// it to an empty object once the user closes the tab.
func (k KasmSession) Connected() bool {
	return len(k.ConnectionInfo) > 0
}

// NewKasmClient builds a KasmClient from app config. Returns nil when Kasm is not
// configured (empty api_base) so callers can treat the feature as disabled.
func NewKasmClient(cfg *config.Config) *KasmClient {
	if cfg == nil {
		return nil
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.Kasm.APIBase), "/")
	if base == "" {
		return nil
	}
	return &KasmClient{
		apiBase:       base,
		apiKey:        strings.TrimSpace(cfg.Kasm.APIKey),
		apiSecret:     strings.TrimSpace(cfg.Kasm.APISecret),
		imageID:       strings.TrimSpace(cfg.Kasm.ImageID),
		publicHost:    strings.TrimSpace(cfg.Kasm.PublicHost),
		seedNamespace: strings.TrimSpace(cfg.Kasm.SeedNamespace),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				// Tailscale IP base; cert CN is kasm.surplustoken.com, not the IP.
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, //nolint:gosec // intentional: pinned-by-Tailscale IP base
			},
		},
	}
}

// ImageID returns the configured Kasm image id used for remote-browser sessions.
func (c *KasmClient) ImageID() string {
	if c == nil {
		return ""
	}
	return c.imageID
}

// post sends a POST to /api/public/<method> with api_key + api_key_secret merged
// into the supplied body, and decodes the JSON response into out (out may be nil).
func (c *KasmClient) post(ctx context.Context, method string, body map[string]any, out any) error {
	if c == nil {
		return fmt.Errorf("kasm client not configured")
	}
	payload := make(map[string]any, len(body)+2)
	for k, v := range body {
		payload[k] = v
	}
	payload["api_key"] = c.apiKey
	payload["api_key_secret"] = c.apiSecret

	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("kasm marshal %s: %w", method, err)
	}

	url := c.apiBase + "/api/public/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("kasm new request %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("kasm do %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("kasm %s: unexpected status %d", method, resp.StatusCode)
	}

	// Cap the body to avoid unbounded reads; Kasm responses are small JSON objects.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("kasm read %s: %w", method, err)
	}

	// Kasm returns HTTP 200 even for logical errors, carrying {"error_message": "..."}.
	var probe struct {
		ErrorMessage string `json:"error_message"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return fmt.Errorf("kasm decode %s: %w", method, err)
	}
	if strings.TrimSpace(probe.ErrorMessage) != "" {
		return fmt.Errorf("kasm %s: %s", method, probe.ErrorMessage)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("kasm decode %s: %w", method, err)
		}
	}
	return nil
}

// requestKasmResponse is the subset of request_kasm's response we consume.
// kasm_url is a path like "/#/connect/kasm/<kasm_id>/<user_id>/<token>" that, when
// prefixed with the public host, is a no-login direct-into-session URL.
type requestKasmResponse struct {
	KasmID  string `json:"kasm_id"`
	Status  string `json:"status"`
	UserID  string `json:"user_id"`
	KasmURL string `json:"kasm_url"`
}

// RequestKasm starts a session for kasmUserID using the configured image and the
// supplied environment (e.g. KASM_SEED_ACCOUNT / KASM_SEED_MODE). It returns the
// kasm session id and the connect URL (rooted at the public host).
func (c *KasmClient) RequestKasm(ctx context.Context, kasmUserID string, env map[string]string) (kasmID string, connectURL string, err error) {
	if c == nil {
		return "", "", fmt.Errorf("kasm client not configured")
	}
	body := map[string]any{
		"user_id":  kasmUserID,
		"image_id": c.imageID,
	}
	if len(env) > 0 {
		body["environment"] = env
	}
	var out requestKasmResponse
	if err := c.post(ctx, "request_kasm", body, &out); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(out.KasmID) == "" {
		return "", "", fmt.Errorf("kasm request_kasm: empty kasm_id")
	}
	return out.KasmID, c.buildConnectURL(out.KasmURL), nil
}

// buildConnectURL rewrites a Kasm-returned path (kasm_url) to the public host so
// the user's browser can reach it. If kasmPath is already absolute we keep its
// path/fragment but swap the host for the configured public host.
func (c *KasmClient) buildConnectURL(kasmPath string) string {
	path := strings.TrimSpace(kasmPath)
	if path == "" {
		return ""
	}
	// Strip any scheme://host prefix Kasm may include (it usually returns a bare path).
	if idx := strings.Index(path, "://"); idx >= 0 {
		rest := path[idx+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			path = rest[slash:]
		} else {
			path = "/"
		}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	host := c.publicHost
	if host == "" {
		// Fall back to the api base host if no public host is configured.
		host = strings.TrimPrefix(strings.TrimPrefix(c.apiBase, "https://"), "http://")
	}
	return "https://" + host + path
}

// getKasmsResponse is the get_kasms response shape.
type getKasmsResponse struct {
	Kasms []KasmSession `json:"kasms"`
}

// GetKasms returns all currently live Kasm sessions across the deployment.
func (c *KasmClient) GetKasms(ctx context.Context) ([]KasmSession, error) {
	if c == nil {
		return nil, fmt.Errorf("kasm client not configured")
	}
	var out getKasmsResponse
	if err := c.post(ctx, "get_kasms", map[string]any{}, &out); err != nil {
		return nil, err
	}
	return out.Kasms, nil
}

// DestroyKasm tears down a session.
func (c *KasmClient) DestroyKasm(ctx context.Context, kasmID, kasmUserID string) error {
	if c == nil {
		return fmt.Errorf("kasm client not configured")
	}
	return c.post(ctx, "destroy_kasm", map[string]any{
		"kasm_id": kasmID,
		"user_id": kasmUserID,
	}, nil)
}

// getUserResponse is the get_user / create_user response shape. user_id has no
// dashes in create_user/get_user responses, but Kasm accepts it as-is.
type getUserResponse struct {
	User struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
	} `json:"user"`
}

// kasmUsername derives the per-SurplusAI-user Kasm username, namespaced by
// environment when seedNamespace is set (surplus-<ns>-u<id> vs surplus-u<id>) so
// prod and staging don't share Kasm users (and thus sessions).
func (c *KasmClient) kasmUsername(surplusUserID int64) string {
	if c.seedNamespace != "" {
		return fmt.Sprintf("surplus-%s-u%d@kasm.local", c.seedNamespace, surplusUserID)
	}
	return fmt.Sprintf("surplus-u%d@kasm.local", surplusUserID)
}

// usernamePrefix is the common prefix of every Kasm username this deployment owns
// ("surplus-<ns>-u" / "surplus-u"). Used to scope orphan reaping to THIS namespace so
// prod's reconciler never tears down staging's sessions (they share one Kasm).
func (c *KasmClient) usernamePrefix() string {
	if c.seedNamespace != "" {
		return fmt.Sprintf("surplus-%s-u", c.seedNamespace)
	}
	return "surplus-u"
}

// OwnsUsername reports whether a Kasm username belongs to THIS deployment's namespace.
// Note the prefixes are disjoint across namespaces ("surplus-prod-u" vs "surplus-u" vs
// "surplus-staging-u"), so a non-namespaced deployment will not claim namespaced users
// and vice-versa.
func (c *KasmClient) OwnsUsername(username string) bool {
	if c == nil {
		return false
	}
	u := strings.ToLower(strings.TrimSpace(username))
	return u != "" && strings.HasPrefix(u, c.usernamePrefix())
}

// SeedAccountValue is the KASM_SEED_ACCOUNT env value, which ime_env.sh uses as the
// seed dir name (/srv/kasm-seeds/<value>). Namespaced so prod/staging seeds differ:
// "<ns>-<accountID>" vs "<accountID>".
func (c *KasmClient) SeedAccountValue(accountID int64) string {
	if c.seedNamespace != "" {
		return fmt.Sprintf("%s-%d", c.seedNamespace, accountID)
	}
	return fmt.Sprintf("%d", accountID)
}

// EnsureKasmUser returns the Kasm user_id for the given SurplusAI user, creating
// the Kasm user (surplus-u<id>@kasm.local) on first use. Each SurplusAI user maps
// to exactly one Kasm user so each person only sees their own session.
func (c *KasmClient) EnsureKasmUser(ctx context.Context, surplusUserID int64) (string, error) {
	if c == nil {
		return "", fmt.Errorf("kasm client not configured")
	}
	username := c.kasmUsername(surplusUserID)

	// Try to look the user up first (get_user errors with "Invalid Request" when missing).
	var existing getUserResponse
	if err := c.post(ctx, "get_user", map[string]any{
		"target_user": map[string]any{"username": username},
	}, &existing); err == nil && strings.TrimSpace(existing.User.UserID) != "" {
		return existing.User.UserID, nil
	}

	// Create the user.
	var created getUserResponse
	if err := c.post(ctx, "create_user", map[string]any{
		"target_user": map[string]any{
			"username": username,
			"password": "",
			"locked":   false,
			"disabled": false,
		},
	}, &created); err != nil {
		return "", err
	}
	if strings.TrimSpace(created.User.UserID) == "" {
		return "", fmt.Errorf("kasm create_user: empty user_id for %s", username)
	}
	return created.User.UserID, nil
}

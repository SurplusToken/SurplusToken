package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// newTestClient builds a client backed by a freshly generated P-256 key.
func newTestClient(t *testing.T) (*Desktop2WebClient, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	cfg := &config.Config{}
	cfg.Desktop2Web = config.Desktop2WebConfig{
		BaseURL:        "https://d2w.example",
		Issuer:         "https://surplustoken.example",
		KeyID:          "test-key-1",
		PrivateKeyFile: path,
		AdminPassword:  "admin-secret",
	}
	return NewDesktop2WebClient(cfg), key
}

// TestMintTicketMatchesDesktop2WebContract pins the wire format the gateway validates:
// a strict claim set, integer epoch seconds, a fixed header, and a raw R||S ES256
// signature. Each assertion here corresponds to a way a ticket is rejected.
func TestMintTicketMatchesDesktop2WebContract(t *testing.T) {
	client, key := newTestClient(t)
	if !client.Configured() {
		t.Fatalf("client should be configured: %v", client.configErr)
	}

	grant := time.Now().Add(time.Hour)
	ticket, err := client.MintTicket("surplus-u103", "surplus-u103-a146", "slot-1", grant.UnixMilli())
	if err != nil {
		t.Fatalf("MintTicket: %v", err)
	}

	parts := strings.Split(ticket, ".")
	if len(parts) != 3 {
		t.Fatalf("ticket must be a compact JWS with 3 segments, got %d", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		t.Fatalf("parse header: %v", err)
	}
	if header["alg"] != "ES256" {
		t.Errorf("alg = %q, want ES256", header["alg"])
	}
	if header["typ"] != "desktop2web-sso+jwt" {
		t.Errorf("typ = %q, want desktop2web-sso+jwt", header["typ"])
	}
	if header["kid"] != "test-key-1" {
		t.Errorf("kid = %q, want the configured key id", header["kid"])
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(claimsBytes, &raw); err != nil {
		t.Fatalf("parse claims: %v", err)
	}
	// The gateway parses claims with a strict schema: an extra key is fatal.
	wantKeys := []string{"version", "iss", "aud", "sub", "jti", "iat", "exp", "accessId", "grantExp"}
	if len(raw) != len(wantKeys) {
		t.Errorf("claims carry %d keys, want exactly %d: %v", len(raw), len(wantKeys), raw)
	}
	for _, want := range wantKeys {
		if _, ok := raw[want]; !ok {
			t.Errorf("missing claim %q", want)
		}
	}
	// Timestamps must be integers; the gateway rejects fractional numeric dates.
	for _, name := range []string{"version", "iat", "exp", "grantExp"} {
		if value, ok := raw[name]; ok && strings.Contains(string(value), ".") {
			t.Errorf("claim %q must be an integer epoch, got %s", name, value)
		}
	}

	// ES256 signatures are the raw R||S pair. x509 produces DER, which would decode to
	// a different length and fail verification here.
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if len(signature) != 64 {
		t.Fatalf("ES256 signature must be 64 bytes of raw R||S, got %d", len(signature))
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	if !ecdsa.Verify(&key.PublicKey, digest[:], r, s) {
		t.Error("signature does not verify against the signing key")
	}

	var claims desktop2WebTicketClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		t.Fatalf("decode typed claims: %v", err)
	}
	if claims.Audience != "desktop2web-gateway" {
		t.Errorf("aud = %q, want desktop2web-gateway", claims.Audience)
	}
	if claims.Issuer != "https://surplustoken.example" {
		t.Errorf("iss = %q, want the configured issuer", claims.Issuer)
	}
	if claims.Version != 1 {
		t.Errorf("version = %d, want 1", claims.Version)
	}
	if claims.Subject != "surplus-u103" || claims.AccessID != "surplus-u103-a146" {
		t.Errorf("sub/accessId = %q/%q, want surplus-u103/surplus-u103-a146", claims.Subject, claims.AccessID)
	}
	// grantExp is seconds; the Access row carries the same instant in milliseconds.
	if claims.GrantExp != grant.Unix() {
		t.Errorf("grantExp = %d, want %d", claims.GrantExp, grant.Unix())
	}
	// The gateway rejects any lifetime above 120s and any iat in the future.
	if lifetime := claims.ExpiresAt - claims.IssuedAt; lifetime <= 0 || lifetime > 120 {
		t.Errorf("ticket lifetime = %ds, want (0, 120]", lifetime)
	}
	if claims.IssuedAt > time.Now().Unix()+1 {
		t.Errorf("iat = %d is in the future", claims.IssuedAt)
	}
	// jti must satisfy the gateway's stable-id grammar.
	if !stableIDPattern.MatchString(claims.JTI) || len(claims.JTI) > desktop2WebMaxStableIDLength {
		t.Errorf("jti %q is not a valid stable id", claims.JTI)
	}
}

// TestMintTicketUsesFreshIDPerTicket guards the single-use guarantee: the gateway
// rejects a repeated jti, so two tickets must never share one.
func TestMintTicketUsesFreshIDPerTicket(t *testing.T) {
	client, _ := newTestClient(t)
	grant := time.Now().Add(time.Hour).UnixMilli()

	seen := make(map[string]struct{}, 2)
	for i := 0; i < 2; i++ {
		ticket, err := client.MintTicket("surplus-u1", "surplus-u1-a1", "slot-1", grant)
		if err != nil {
			t.Fatalf("MintTicket: %v", err)
		}
		parts := strings.Split(ticket, ".")
		claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			t.Fatalf("decode claims: %v", err)
		}
		var claims desktop2WebTicketClaims
		if err := json.Unmarshal(claimsBytes, &claims); err != nil {
			t.Fatalf("parse claims: %v", err)
		}
		if _, dup := seen[claims.JTI]; dup {
			t.Fatalf("jti %q was reused across tickets", claims.JTI)
		}
		seen[claims.JTI] = struct{}{}
	}
}

func TestAccountSlotIDDerivation(t *testing.T) {
	client, _ := newTestClient(t)

	tests := []struct {
		name    string
		email   string
		extra   map[string]any
		want    string
		wantErr error
	}{
		{
			name:  "email convention rewrites the at sign",
			email: "lyucmu@gmail.com",
			want:  "lyucmu-at-gmail.com",
		},
		{
			name:  "explicit override wins over the derived id",
			email: "lyucmu@gmail.com",
			extra: map[string]any{desktop2WebSlotIDExtraKey: "custom-slot"},
			want:  "custom-slot",
		},
		{
			name:    "no email and no override is unmapped",
			wantErr: ErrRemoteSessionSlotUnmapped,
		},
		{
			name:    "an email that cannot form a slot id is unmapped",
			email:   "a+b@example.com",
			wantErr: ErrRemoteSessionSlotUnmapped,
		},
		{
			name:    "a malformed override is rejected rather than passed through",
			email:   "lyucmu@gmail.com",
			extra:   map[string]any{desktop2WebSlotIDExtraKey: "bad slot id"},
			wantErr: ErrRemoteSessionSlotInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account := &Account{
				Credentials: map[string]any{"email": test.email},
				Extra:       test.extra,
			}
			got, err := client.accountSlotID(account)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("accountSlotID error = %v, want %v", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("accountSlotID: %v", err)
			}
			if got != test.want {
				t.Errorf("accountSlotID = %q, want %q", got, test.want)
			}
		})
	}
}

// TestNewDesktop2WebClientDisabledWithoutBaseURL keeps the feature switch honest:
// an unconfigured deployment must report disabled, not a half-built client.
func TestNewDesktop2WebClientDisabledWithoutBaseURL(t *testing.T) {
	if client := NewDesktop2WebClient(&config.Config{}); client != nil {
		t.Fatal("expected a nil client when desktop2web.base_url is empty")
	}
	if NewDesktop2WebClient(nil) != nil {
		t.Fatal("expected a nil client for a nil config")
	}
}

// TestNewDesktop2WebClientRejectsBadKeyMaterial ensures a broken deployment reports
// misconfiguration instead of signing with an absent key.
func TestNewDesktop2WebClientRejectsBadKeyMaterial(t *testing.T) {
	cfg := &config.Config{}
	cfg.Desktop2Web = config.Desktop2WebConfig{
		BaseURL:        "https://d2w.example",
		Issuer:         "https://surplustoken.example",
		KeyID:          "test-key-1",
		PrivateKeyFile: filepath.Join(t.TempDir(), "missing.pem"),
		AdminPassword:  "admin-secret",
	}
	client := NewDesktop2WebClient(cfg)
	if client == nil {
		t.Fatal("a client with a base URL must not be nil")
	}
	if client.Configured() {
		t.Fatal("expected misconfiguration for an unreadable key file")
	}
	if _, err := client.MintTicket("surplus-u1", "surplus-u1-a1", "slot", time.Now().UnixMilli()); !errors.Is(err, ErrRemoteSessionMisconfigured) {
		t.Fatalf("MintTicket error = %v, want ErrRemoteSessionMisconfigured", err)
	}
}

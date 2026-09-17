package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// desktop2WebSlotIDExtraKey is the accounts.extra override for the Desktop2Web
// account slot id. When absent the slot id is derived from the account's email.
const desktop2WebSlotIDExtraKey = "desktop2web_slot_id"

// Remote-session feature errors (mapped to HTTP by response.ErrorFrom).
var (
	ErrRemoteSessionNotConfigured = infraerrors.New(503, "REMOTE_SESSION_NOT_CONFIGURED", "remote browser is not configured")
	ErrRemoteSessionForbidden     = infraerrors.Forbidden("REMOTE_SESSION_FORBIDDEN", "you are not an owner or co-owner of this account")
	ErrRemoteSessionNotPro        = infraerrors.Forbidden("REMOTE_SESSION_NOT_PRO", "remote browser is only available for Pro accounts")

	// ErrRemoteSessionMisconfigured reports unusable key material or credentials. It is
	// separate from NotConfigured so an operator can tell "feature off" from "feature on
	// but broken", which otherwise both look like a dead button.
	ErrRemoteSessionMisconfigured = infraerrors.New(503, "REMOTE_SESSION_MISCONFIGURED", "remote browser is misconfigured")

	// ErrRemoteSessionUpstream reports a Desktop2Web admin call that failed or returned
	// an unexpected status.
	ErrRemoteSessionUpstream = infraerrors.New(502, "REMOTE_SESSION_UPSTREAM_FAILED", "the remote browser gateway rejected the request")

	// ErrRemoteSessionSlotUnmapped: the account carries no usable email and no explicit
	// slot override, or the derived slot id is not registered on the gateway. The
	// gateway does not validate slot existence when provisioning an Access, so this
	// pre-flight is the only place the problem is reported clearly.
	ErrRemoteSessionSlotUnmapped = infraerrors.New(409, "REMOTE_SESSION_ACCOUNT_UNMAPPED", "this account is not linked to a remote browser account")
	// ErrRemoteSessionSlotDisabled: the slot exists but is disabled on the gateway, so
	// every session for it would be revoked.
	ErrRemoteSessionSlotDisabled = infraerrors.New(409, "REMOTE_SESSION_ACCOUNT_DISABLED", "the linked remote browser account is disabled")
	// ErrRemoteSessionSlotInvalid: the accounts.extra override is not a legal slot id.
	ErrRemoteSessionSlotInvalid = infraerrors.New(400, "REMOTE_SESSION_ACCOUNT_SLOT_INVALID", "the configured remote browser account id is invalid")
)

// RemoteSessionService implements the "远程连接" (remote browser) feature.
//
// It gates the hand-off to Pro accounts the caller owns or co-owns, resolves the
// account's Desktop2Web slot, provisions the Access and mints the single-use SSO
// ticket that carries the user's browser into a session. The gateway owns everything
// after that: runtime startup, session lifetime, idle shutdown and revocation. Nothing
// here tracks a live session, so there is no local queue, keepalive or reconciler.
type RemoteSessionService struct {
	accountSvc *AccountService
	client     *Desktop2WebClient
}

// NewRemoteSessionService constructs the service. client may be nil (feature disabled).
func NewRemoteSessionService(accountSvc *AccountService, client *Desktop2WebClient) *RemoteSessionService {
	return &RemoteSessionService{accountSvc: accountSvc, client: client}
}

// Enabled reports whether the hand-off target is configured. A client that exists but
// has unusable key material still reports enabled so the failure is explicit.
func (s *RemoteSessionService) Enabled() bool {
	return s != nil && s.accountSvc != nil && s.client != nil
}

// RemoteSessionResult is what the frontend needs to enter a session: the ticket is
// form-POSTed to RedeemURL by the browser, never placed in a URL we control.
type RemoteSessionResult struct {
	Ticket    string `json:"ticket"`
	RedeemURL string `json:"redeem_url"`
}

// authorizedAccount loads the account, hydrates its co-owner set, and verifies the
// requesting user is an owner or co-owner. It also enforces the Pro-plan gate.
func (s *RemoteSessionService) authorizedAccount(ctx context.Context, surplusUserID, accountID int64) (*Account, error) {
	if !s.Enabled() {
		return nil, ErrRemoteSessionNotConfigured
	}
	account, err := s.accountSvc.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrAccountNotFound
	}

	// Hydrate co-owners (GetByID does not). On hydrate error, fail closed for co-owners
	// but still allow the primary owner (whose membership doesn't need the co-owner list).
	coOwnerIDs, hydrateErr := s.accountSvc.accountRepo.ListCoOwnerUserIDsByAccount(ctx, accountID)
	if hydrateErr == nil {
		account.CoOwnerUserIDs = coOwnerIDs
	}

	if !account.IsSurplusAIOwner(surplusUserID) {
		return nil, ErrRemoteSessionForbidden
	}

	// Pro gate. Raw credential plan_type is stored as e.g. "chatgptpro"; normalize.
	if normalizeUserAccountPoolPlanType(account.GetCredential("plan_type")) != "pro" {
		return nil, ErrRemoteSessionNotPro
	}
	return account, nil
}

// Connect provisions the caller's Access for this account and mints a ticket for it.
//
// The Access is upserted on every call with a fresh grant, which is required for
// reconnects to work but has a sharp edge on the gateway side: any change to the row
// revokes that Access's live sessions. Reconnecting therefore invalidates the user's
// previous browser session, which is the intended "one device at a time" behaviour.
func (s *RemoteSessionService) Connect(ctx context.Context, surplusUserID, accountID int64) (*RemoteSessionResult, error) {
	account, err := s.authorizedAccount(ctx, surplusUserID, accountID)
	if err != nil {
		return nil, err
	}

	slotID, err := s.client.accountSlotID(account)
	if err != nil {
		return nil, err
	}
	if err := s.client.EnsureAccountSlot(ctx, slotID); err != nil {
		return nil, err
	}

	userID := desktop2WebUserID(surplusUserID)
	accessID := desktop2WebAccessID(surplusUserID, accountID)
	grantExpiresAtMs := time.Now().Add(s.client.grantTTL).UnixMilli()
	if err := s.client.EnsureAccess(ctx, accessID, userID, slotID, grantExpiresAtMs); err != nil {
		return nil, err
	}

	ticket, err := s.client.MintTicket(userID, accessID, slotID, grantExpiresAtMs)
	if err != nil {
		return nil, err
	}
	return &RemoteSessionResult{Ticket: ticket, RedeemURL: s.client.RedeemURL()}, nil
}

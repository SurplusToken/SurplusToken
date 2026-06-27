package service

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// strconv64 formats an int64 as a base-10 string.
func strconv64(v int64) string { return strconv.FormatInt(v, 10) }

// normalizeKasmID strips dashes so the dashed form stored in remote_sessions
// (returned by request_kasm) compares equal to the dash-less form get_kasms
// returns. destroy_kasm accepts either form, so only correlation needs this.
func normalizeKasmID(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "-", "")
}

// Remote-session concurrency limits and status/mode constants.
const (
	RemoteSessionMaxPerAccount = 5
	RemoteSessionMaxGlobal     = 5

	// remoteSessionReconcileGrace protects a freshly-requested session from being
	// reaped by the reconciler before Kasm reports it in get_kasms (a new session
	// takes a few seconds to appear). Without it, newborns get marked ended ~1s
	// after creation and the real container leaks.
	remoteSessionReconcileGrace = 90 * time.Second

	// remoteSessionDisconnectGrace is how long a session may stay alive in Kasm with
	// NO observed activity before the reconciler tears it down. This is the server-side
	// backstop for "user closed the tab → stop the container" that works regardless of
	// what happens to the platform page (close/refresh/network loss).
	//
	// CRITICAL: this MUST exceed Kasm's keepalive_interval (300s). The reconciler's only
	// reliable liveness signal here is keepalive_date advancing (connection_info comes
	// back empty in this deployment), and Kasm only advances it every keepalive_interval.
	// A grace shorter than that window reaps ACTIVELY-CONNECTED sessions (the original
	// 90s value killed every session at ~2 minutes). The frontend also bumps last_seen
	// via a keepalive ping while its tab is open, so a real disconnect is detected well
	// before this backstop fires; the backstop just covers crash/refresh/network-loss.
	remoteSessionDisconnectGrace = 360 * time.Second

	// remoteSessionOrphanStaleGrace is how stale a Kasm session's keepalive_date must be
	// before the reconciler reaps it as an ORPHAN — a session belonging to this namespace
	// that has no live remote_sessions row (e.g. a row-create failed, or a destroy_kasm
	// did not land). Combined with "no viewer attached", this guarantees we never tear
	// down a session anyone is actually using. Generous so it can't race a live session.
	remoteSessionOrphanStaleGrace = 15 * time.Minute

	RemoteSessionStatusStarting = "starting"
	RemoteSessionStatusRunning  = "running"
	RemoteSessionStatusQueued   = "queued"
	RemoteSessionStatusEnded    = "ended"

	RemoteSessionModeSetup = "setup"
	RemoteSessionModeUse   = "use"

	// remoteSeedReadyExtraKey is the accounts.extra flag that records the owner has
	// run setup (the per-account seed Firefox profile holds a persisted login).
	remoteSeedReadyExtraKey = "remote_seed_ready"

	kasmSeedAccountEnvKey = "KASM_SEED_ACCOUNT"
	kasmSeedModeEnvKey    = "KASM_SEED_MODE"
)

// Remote-session feature errors (mapped to HTTP by response.ErrorFrom).
var (
	ErrRemoteSessionNotConfigured = infraerrors.New(503, "REMOTE_SESSION_NOT_CONFIGURED", "remote browser is not configured")
	ErrRemoteSessionForbidden     = infraerrors.Forbidden("REMOTE_SESSION_FORBIDDEN", "you are not an owner or co-owner of this account")
	ErrRemoteSessionNotPro        = infraerrors.Forbidden("REMOTE_SESSION_NOT_PRO", "remote browser is only available for Pro accounts")
	ErrRemoteSessionSeedNotReady  = infraerrors.BadRequest("REMOTE_SESSION_SEED_NOT_READY", "the account owner has not completed remote-browser setup yet")
)

// RemoteSession is one remote-browser session row.
type RemoteSession struct {
	ID            int64
	AccountID     int64
	SurplusUserID int64
	KasmID        string
	KasmUserID    string
	Mode          string
	Status        string
	// ConnectURL is the Kasm auto-login URL (rooted at the public host). Stored so a
	// reconnect can REATTACH to the same live container instead of recreating it.
	ConnectURL string
	CreatedAt  time.Time
	LastSeenAt time.Time
	EndedAt    *time.Time
}

// RemoteSessionRepository persists remote_sessions rows.
type RemoteSessionRepository interface {
	Create(ctx context.Context, s *RemoteSession) error
	// CountLiveByAccount counts non-ended sessions (status in starting/running) for an account.
	CountLiveByAccount(ctx context.Context, accountID int64) (int, error)
	// CountLiveGlobal counts non-ended sessions (status in starting/running) across all accounts.
	CountLiveGlobal(ctx context.Context) (int, error)
	// ListLive returns all non-ended sessions (status in starting/running).
	ListLive(ctx context.Context) ([]RemoteSession, error)
	// GetLiveByAccountAndUser returns the most recent non-ended session for (account, user),
	// or nil if none. Used to reattach a reconnecting user to their existing container.
	GetLiveByAccountAndUser(ctx context.Context, accountID, surplusUserID int64) (*RemoteSession, error)
	// MarkEndedByKasmID marks the most recent live row with kasm_id as ended; no-op if none.
	MarkEndedByKasmID(ctx context.Context, kasmID string) error
	// MarkEndedByIDs marks the given row ids as ended.
	MarkEndedByIDs(ctx context.Context, ids []int64) error
	// TouchLastSeen bumps last_seen_at=NOW() for a live row (called while a viewer is attached).
	TouchLastSeen(ctx context.Context, id int64) error
}

// RemoteSessionService implements the "远程连接" (remote browser) feature: it gates
// access to Pro accounts owned/co-owned by the requesting user, manages per-account
// (≤5) and global (≤5) concurrency with a queue, talks to Kasm to start/stop the
// Firefox sessions, and reconciles the local table against Kasm liveness.
type RemoteSessionService struct {
	repo       RemoteSessionRepository
	accountSvc *AccountService
	kasm       *KasmClient

	// reconciler lifecycle (mirrors AccountExpiryService).
	interval time.Duration
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	// lastKeepalive tracks the last-observed Kasm keepalive_date per kasm_id so the
	// reconciler can tell "client still pinging" from "client gone" even if
	// connection_info is momentarily empty. Touched only by the single reconciler
	// goroutine, so no lock is needed.
	lastKeepalive map[string]string
}

// NewRemoteSessionService constructs the service. kasm may be nil (feature disabled).
func NewRemoteSessionService(repo RemoteSessionRepository, accountSvc *AccountService, kasm *KasmClient, reconcileInterval time.Duration) *RemoteSessionService {
	return &RemoteSessionService{
		repo:          repo,
		accountSvc:    accountSvc,
		kasm:          kasm,
		interval:      reconcileInterval,
		stopCh:        make(chan struct{}),
		lastKeepalive: make(map[string]string),
	}
}

// Enabled reports whether Kasm is configured.
func (s *RemoteSessionService) Enabled() bool {
	return s != nil && s.kasm != nil
}

// authorizedAccount loads the account, hydrates its co-owner set, and verifies the
// requesting user is owner-or-co-owner. requireOwnerOnly restricts to the primary
// owner (used by the setup endpoint). It also enforces the Pro-plan gate.
func (s *RemoteSessionService) authorizedAccount(ctx context.Context, surplusUserID, accountID int64, requireOwnerOnly bool) (*Account, error) {
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

	if requireOwnerOnly {
		if account.OwnerUserID == nil || *account.OwnerUserID != surplusUserID {
			return nil, ErrRemoteSessionForbidden
		}
	} else if !account.IsSurplusAIOwner(surplusUserID) {
		return nil, ErrRemoteSessionForbidden
	}

	// Pro gate. Raw credential plan_type is stored as e.g. "chatgptpro"; normalize.
	if normalizeUserAccountPoolPlanType(account.GetCredential("plan_type")) != "pro" {
		return nil, ErrRemoteSessionNotPro
	}
	return account, nil
}

// RemoteSessionSetupResult is returned by Setup.
type RemoteSessionSetupResult struct {
	ConnectURL string `json:"connect_url"`
	KasmID     string `json:"kasm_id"`
}

// Setup (owner only) starts a MODE=setup Kasm session so the owner can persist the
// account's login into the per-account seed Firefox profile, then marks the account
// extra remote_seed_ready=true.
func (s *RemoteSessionService) Setup(ctx context.Context, surplusUserID, accountID int64) (*RemoteSessionSetupResult, error) {
	account, err := s.authorizedAccount(ctx, surplusUserID, accountID, true)
	if err != nil {
		return nil, err
	}

	// One container per user: tear down any session this owner already has for the
	// account before starting a new one. Also guarantees a single writer of the shared
	// seed profile (concurrent setup containers symlink the same ~/.mozilla and Firefox
	// would refuse to start / corrupt the profile).
	s.destroyUserSessions(ctx, account.ID, surplusUserID)

	kasmUserID, err := s.kasm.EnsureKasmUser(ctx, surplusUserID)
	if err != nil {
		return nil, err
	}
	kasmID, connectURL, err := s.kasm.RequestKasm(ctx, kasmUserID, map[string]string{
		kasmSeedAccountEnvKey: s.kasm.SeedAccountValue(account.ID),
		kasmSeedModeEnvKey:    RemoteSessionModeSetup,
	})
	if err != nil {
		return nil, err
	}

	// Mark the account seed-ready (JSONB merge; won't clobber other extra keys).
	if err := s.accountSvc.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{remoteSeedReadyExtraKey: true}); err != nil {
		// Best-effort; the setup session is already running. Surface the error.
		return nil, err
	}

	_ = s.repo.Create(ctx, &RemoteSession{
		AccountID:     account.ID,
		SurplusUserID: surplusUserID,
		KasmID:        kasmID,
		KasmUserID:    kasmUserID,
		Mode:          RemoteSessionModeSetup,
		Status:        RemoteSessionStatusStarting,
		ConnectURL:    connectURL,
	})

	return &RemoteSessionSetupResult{ConnectURL: connectURL, KasmID: kasmID}, nil
}

// destroyUserSessions best-effort tears down any live Kasm sessions this user already
// has for the account (any mode) and marks their rows ended. This enforces one
// container per user per account (a fresh click replaces the user's previous session
// instead of stacking) and, for setup, guarantees a single writer of the shared seed.
func (s *RemoteSessionService) destroyUserSessions(ctx context.Context, accountID, surplusUserID int64) {
	live, err := s.repo.ListLive(ctx)
	if err != nil {
		return
	}
	for i := range live {
		if live[i].AccountID != accountID || live[i].SurplusUserID != surplusUserID {
			continue
		}
		_ = s.kasm.DestroyKasm(ctx, live[i].KasmID, live[i].KasmUserID)
		_ = s.repo.MarkEndedByKasmID(ctx, live[i].KasmID)
	}
}

// RemoteSessionResult is returned by Connect / Poll.
type RemoteSessionResult struct {
	Status     string `json:"status"` // "ready" or "queued"
	ConnectURL string `json:"connect_url,omitempty"`
	KasmID     string `json:"kasm_id,omitempty"`
	// Position is an approximate 1-based queue position when queued (0 when unknown).
	Position int `json:"position,omitempty"`
}

// Connect (owner+co-owner, Pro, requires remote_seed_ready) tries to allocate a
// MODE=use session. If per-account (≥5) or global (≥5) live capacity is exhausted it
// returns {status:"queued"}; otherwise it starts the session and returns ready.
func (s *RemoteSessionService) Connect(ctx context.Context, surplusUserID, accountID int64) (*RemoteSessionResult, error) {
	account, err := s.authorizedAccount(ctx, surplusUserID, accountID, false)
	if err != nil {
		return nil, err
	}
	if ready, _ := account.Extra[remoteSeedReadyExtraKey].(bool); !ready {
		return nil, ErrRemoteSessionSeedNotReady
	}
	return s.allocate(ctx, surplusUserID, account.ID)
}

// Poll re-runs the capacity check for a queued caller; identical semantics to Connect
// minus the seed-ready check (already validated when the user first queued).
func (s *RemoteSessionService) Poll(ctx context.Context, surplusUserID, accountID int64) (*RemoteSessionResult, error) {
	account, err := s.authorizedAccount(ctx, surplusUserID, accountID, false)
	if err != nil {
		return nil, err
	}
	if ready, _ := account.Extra[remoteSeedReadyExtraKey].(bool); !ready {
		return nil, ErrRemoteSessionSeedNotReady
	}
	return s.allocate(ctx, surplusUserID, account.ID)
}

// kasmSessionLive reports whether Kasm still lists a session with this id (dash-insensitive).
// On a Kasm API error it returns true (fail-safe: don't tear down a possibly-good session).
func (s *RemoteSessionService) kasmSessionLive(ctx context.Context, kasmID string) bool {
	want := normalizeKasmID(kasmID)
	if want == "" {
		return false
	}
	kasms, err := s.kasm.GetKasms(ctx)
	if err != nil {
		return true
	}
	for _, k := range kasms {
		if normalizeKasmID(k.KasmID) == want {
			return true
		}
	}
	return false
}

// allocate either REATTACHES the user to their existing live container, starts a new
// session, or returns queued when capacity is exhausted.
func (s *RemoteSessionService) allocate(ctx context.Context, surplusUserID, accountID int64) (*RemoteSessionResult, error) {
	// Reattach: if the user already has a live session for this account that Kasm still
	// reports, hand back the SAME container (its stored connect URL) instead of
	// destroying + recreating it. This is what preserves the open tab and the
	// in-progress conversation across a reconnect — a fresh container would only ever
	// show server-side history, losing whatever wasn't yet saved.
	if existing, err := s.repo.GetLiveByAccountAndUser(ctx, accountID, surplusUserID); err == nil && existing != nil {
		if existing.Mode == RemoteSessionModeUse && strings.TrimSpace(existing.ConnectURL) != "" && s.kasmSessionLive(ctx, existing.KasmID) {
			_ = s.repo.TouchLastSeen(ctx, existing.ID)
			return &RemoteSessionResult{
				Status:     RemoteSessionStatusRunning,
				ConnectURL: existing.ConnectURL,
				KasmID:     existing.KasmID,
			}, nil
		}
		// Stale row, or a leftover setup-mode session: retire it before reallocating so it
		// doesn't hold a concurrency slot (and a fresh use-mode container is started below).
		_ = s.kasm.DestroyKasm(ctx, existing.KasmID, existing.KasmUserID)
		_ = s.repo.MarkEndedByKasmID(ctx, existing.KasmID)
	}

	perAccount, err := s.repo.CountLiveByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	global, err := s.repo.CountLiveGlobal(ctx)
	if err != nil {
		return nil, err
	}
	if perAccount >= RemoteSessionMaxPerAccount || global >= RemoteSessionMaxGlobal {
		// Approximate position: how many slots over the binding limit, +1.
		over := perAccount - RemoteSessionMaxPerAccount
		if g := global - RemoteSessionMaxGlobal; g > over {
			over = g
		}
		if over < 0 {
			over = 0
		}
		return &RemoteSessionResult{Status: RemoteSessionStatusQueued, Position: over + 1}, nil
	}

	kasmUserID, err := s.kasm.EnsureKasmUser(ctx, surplusUserID)
	if err != nil {
		return nil, err
	}
	kasmID, connectURL, err := s.kasm.RequestKasm(ctx, kasmUserID, map[string]string{
		kasmSeedAccountEnvKey: s.kasm.SeedAccountValue(accountID),
		kasmSeedModeEnvKey:    RemoteSessionModeUse,
	})
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, &RemoteSession{
		AccountID:     accountID,
		SurplusUserID: surplusUserID,
		KasmID:        kasmID,
		KasmUserID:    kasmUserID,
		Mode:          RemoteSessionModeUse,
		Status:        RemoteSessionStatusStarting,
		ConnectURL:    connectURL,
	}); err != nil {
		// Roll back the Kasm session so we don't leak a slot.
		_ = s.kasm.DestroyKasm(ctx, kasmID, kasmUserID)
		return nil, err
	}

	return &RemoteSessionResult{
		Status:     RemoteSessionStatusRunning, // "ready" semantics; the handler reports "ready"
		ConnectURL: connectURL,
		KasmID:     kasmID,
	}, nil
}

// Disconnect (called on tab close / explicit disconnect) destroys the Kasm session
// for kasmID (verifying the caller owns it) and marks the row ended.
func (s *RemoteSessionService) Disconnect(ctx context.Context, surplusUserID, accountID int64, kasmID string) error {
	if _, err := s.authorizedAccount(ctx, surplusUserID, accountID, false); err != nil {
		return err
	}
	kasmID = strings.TrimSpace(kasmID)
	if kasmID == "" {
		return infraerrors.BadRequest("REMOTE_SESSION_KASM_ID_REQUIRED", "kasm_id is required")
	}
	kasmUserID, err := s.kasm.EnsureKasmUser(ctx, surplusUserID)
	if err != nil {
		return err
	}
	// Best-effort destroy; mark the row ended regardless so the slot frees up.
	_ = s.kasm.DestroyKasm(ctx, kasmID, kasmUserID)
	return s.repo.MarkEndedByKasmID(ctx, kasmID)
}

// Keepalive bumps last_seen_at for the caller's live session on this account. The
// frontend calls this every ~30s while its Kasm tab is open, giving the reconciler a
// reliable "user is still here" signal that does NOT depend on Kasm's connection_info
// (empty in this deployment) or its slow 300s keepalive_date. No-op (not an error) when
// the user has no live session.
func (s *RemoteSessionService) Keepalive(ctx context.Context, surplusUserID, accountID int64) error {
	if _, err := s.authorizedAccount(ctx, surplusUserID, accountID, false); err != nil {
		return err
	}
	existing, err := s.repo.GetLiveByAccountAndUser(ctx, accountID, surplusUserID)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	return s.repo.TouchLastSeen(ctx, existing.ID)
}

// Start launches the reconciler goroutine (every s.interval). Mirrors
// AccountExpiryService: no ctx; stop via Stop()/stopCh.
func (s *RemoteSessionService) Start() {
	if s == nil || s.kasm == nil || s.repo == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.reconcileOnce()
		for {
			select {
			case <-ticker.C:
				s.reconcileOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop halts the reconciler.
func (s *RemoteSessionService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

// reconcileOnce keeps the local table and Kasm in sync, and is the server-side
// "user left → stop the container" backstop. For each live row it:
//   - marks ended any row whose kasm_id Kasm no longer reports (with a creation
//     grace so newborns aren't reaped before get_kasms lists them), and
//   - for sessions Kasm still reports, destroys the Kasm container (and marks the
//     row ended) once NO viewer has been attached for remoteSessionDisconnectGrace —
//     detected via connection_info plus keepalive_date movement.
func (s *RemoteSessionService) reconcileOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	live, err := s.repo.ListLive(ctx)
	if err != nil {
		return
	}
	kasms, err := s.kasm.GetKasms(ctx)
	if err != nil {
		return
	}
	// NOTE: request_kasm returns the kasm_id WITH dashes (stored in the DB), but
	// get_kasms returns it WITHOUT dashes. Correlate on the dash-stripped form or
	// every live session looks "gone" and gets ended without being destroyed (leak).
	byID := make(map[string]KasmSession, len(kasms))
	for _, k := range kasms {
		if id := normalizeKasmID(k.KasmID); id != "" {
			byID[id] = k
		}
	}

	now := time.Now()
	seen := make(map[string]struct{}, len(live))
	var ended []int64
	for i := range live {
		sess := live[i]
		kasmID := normalizeKasmID(sess.KasmID)
		seen[kasmID] = struct{}{}

		k, ok := byID[kasmID]
		if !ok {
			// Kasm no longer reports it. Protect newborns from the get_kasms lag race.
			if now.Sub(sess.CreatedAt) >= remoteSessionReconcileGrace {
				ended = append(ended, sess.ID)
				delete(s.lastKeepalive, kasmID)
			}
			continue
		}

		// "Active" = a viewer is attached, or its keepalive_date advanced since the
		// previous reconcile (client still pinging). Either keeps the session alive.
		active := k.Connected()
		if prev, had := s.lastKeepalive[kasmID]; had && prev != k.KeepaliveDate {
			active = true
		}
		s.lastKeepalive[kasmID] = k.KeepaliveDate

		if active {
			_ = s.repo.TouchLastSeen(ctx, sess.ID)
			continue
		}

		// Alive in Kasm but no viewer. Tear it down once the disconnect grace elapses,
		// measured from the last time we saw activity (or creation if never connected).
		ref := sess.CreatedAt
		if sess.LastSeenAt.After(ref) {
			ref = sess.LastSeenAt
		}
		if now.Sub(ref) >= remoteSessionDisconnectGrace {
			_ = s.kasm.DestroyKasm(ctx, sess.KasmID, sess.KasmUserID)
			ended = append(ended, sess.ID)
			delete(s.lastKeepalive, kasmID)
		}
	}

	// Prune keepalive entries for sessions that are no longer live.
	for id := range s.lastKeepalive {
		if _, ok := seen[id]; !ok {
			delete(s.lastKeepalive, id)
		}
	}

	if len(ended) > 0 {
		_ = s.repo.MarkEndedByIDs(ctx, ended)
	}

	// Orphan reaping: tear down Kasm sessions that belong to THIS namespace but have no
	// live row (a row-create failed, or a prior destroy_kasm didn't land — leaving a
	// container running with nobody tracking it). Scoped by username prefix so prod never
	// touches staging's sessions on the shared Kasm.
	s.reapOrphans(ctx, kasms, seen, now)
}

// reapOrphans destroys Kasm sessions that are ours (by username namespace), untracked
// (no live row), have no attached viewer, AND whose keepalive_date is stale beyond
// remoteSessionOrphanStaleGrace. The triple guard makes it impossible to reap a session
// anyone could still be using or one that was just created in a row-create race.
func (s *RemoteSessionService) reapOrphans(ctx context.Context, kasms []KasmSession, seen map[string]struct{}, now time.Time) {
	for _, k := range kasms {
		id := normalizeKasmID(k.KasmID)
		if id == "" {
			continue
		}
		if _, tracked := seen[id]; tracked {
			continue // we have a live row for it
		}
		if !s.kasm.OwnsUsername(k.Username) {
			continue // another deployment's session — never touch
		}
		if k.Connected() {
			continue // a viewer is attached
		}
		ka, ok := parseKasmKeepalive(k.KeepaliveDate)
		if !ok || now.Sub(ka) < remoteSessionOrphanStaleGrace {
			continue // recently active, or unknown freshness — leave it
		}
		_ = s.kasm.DestroyKasm(ctx, k.KasmID, k.UserID)
	}
}

// parseKasmKeepalive parses Kasm's keepalive_date ("2006-01-02 15:04:05.999999"; no
// timezone, interpreted as UTC). Returns ok=false when empty/unparseable.
func parseKasmKeepalive(v string) (time.Time, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02 15:04:05.999999", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

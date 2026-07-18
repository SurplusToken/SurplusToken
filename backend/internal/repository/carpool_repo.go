package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type carpoolRepository struct {
	db *sql.DB
}

func NewCarpoolRepository(db *sql.DB) service.CarpoolRepository {
	return &carpoolRepository{db: db}
}

func (r *carpoolRepository) List(ctx context.Context, userID int64) ([]service.Carpool, error) {
	rows, err := r.db.QueryContext(ctx, carpoolSelectSQL+`
WHERE (c.visibility = 'public' AND c.status <> 'cancelled')
   OR EXISTS (
       SELECT 1 FROM carpool_members visible_member
       WHERE visible_member.carpool_id = c.id AND visible_member.user_id = $1
   )
ORDER BY c.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list carpools: %w", err)
	}
	defer rows.Close()

	items := make([]service.Carpool, 0)
	for rows.Next() {
		item, err := scanCarpool(rows)
		if err != nil {
			return nil, fmt.Errorf("scan carpool: %w", err)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate carpools: %w", err)
	}
	return items, nil
}

func (r *carpoolRepository) GetByInvite(ctx context.Context, userID int64, tokenHash string) (*service.Carpool, error) {
	row := r.db.QueryRowContext(ctx, carpoolSelectSQL+`
WHERE EXISTS (
    SELECT 1 FROM carpool_invites invite
    WHERE invite.carpool_id = c.id
      AND invite.token_hash = $2
      AND invite.revoked_at IS NULL
      AND (invite.expires_at IS NULL OR invite.expires_at > NOW())
      AND (invite.max_uses = 0 OR invite.use_count < invite.max_uses)
)`, userID, tokenHash)
	item, err := scanCarpool(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolInviteInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("resolve carpool invite: %w", err)
	}
	return item, nil
}

func (r *carpoolRepository) Create(ctx context.Context, ownerUserID int64, input service.CreateCarpoolInput, inviteHash, inviteHint string) (*service.CarpoolMutationResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin carpool create: %w", err)
	}
	defer tx.Rollback()

	var groupNameExists bool
	if err := tx.QueryRowContext(ctx, `
SELECT EXISTS(SELECT 1 FROM groups WHERE name = $1 AND deleted_at IS NULL)
    OR EXISTS(SELECT 1 FROM carpools WHERE name = $1 AND status IN ('recruiting', 'starting', 'active'))`, input.Name).Scan(&groupNameExists); err != nil {
		return nil, fmt.Errorf("check carpool group name: %w", err)
	}
	if groupNameExists {
		return nil, service.ErrCarpoolNameConflict
	}

	seatsPerAccount := 5
	baseFee := 130.0
	if input.CarType == service.CarpoolTypeLarge {
		seatsPerAccount = 10
		baseFee = 65
	}
	capacity := input.Level * seatsPerAccount

	var carpoolID int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO carpools (
    name, description, owner_user_id, platform, plan_type, car_type, level,
    capacity, base_fee_cny, usage_pool_cny_per_account, visibility,
    scheduled_start_at
) VALUES ($1, $2, $3, 'openai', 'openai_pro', $4, $5, $6, $7, 750, $8, $9)
RETURNING id`, input.Name, input.Description, ownerUserID, input.CarType, input.Level,
		capacity, baseFee, input.Visibility, input.ScheduledStartAt).Scan(&carpoolID)
	if err != nil {
		return nil, translateCarpoolWriteError(err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO carpool_members (carpool_id, user_id, role, status)
VALUES ($1, $2, 'owner', 'joined')`, carpoolID, ownerUserID); err != nil {
		return nil, fmt.Errorf("create carpool owner membership: %w", err)
	}
	if err := insertCarpoolInvite(ctx, tx, carpoolID, ownerUserID, inviteHash, inviteHint); err != nil {
		return nil, err
	}
	if err := insertCarpoolEvent(ctx, tx, carpoolID, ownerUserID, "created"); err != nil {
		return nil, err
	}
	item, err := getCarpoolByID(ctx, tx, carpoolID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit carpool create: %w", err)
	}
	return &service.CarpoolMutationResult{Carpool: item}, nil
}

func (r *carpoolRepository) CreateInvite(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool, inviteHash, inviteHint string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin carpool invite create: %w", err)
	}
	defer tx.Rollback()

	var ownerUserID sql.NullInt64
	var status string
	var locked bool
	err = tx.QueryRowContext(ctx, `
SELECT owner_user_id, status, join_locked_at IS NOT NULL
FROM carpools WHERE id = $1 FOR UPDATE`, carpoolID).Scan(&ownerUserID, &status, &locked)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrCarpoolNotFound
	}
	if err != nil {
		return fmt.Errorf("load carpool for invite: %w", err)
	}
	if !isAdmin && (!ownerUserID.Valid || ownerUserID.Int64 != actorUserID) {
		return service.ErrCarpoolForbidden
	}
	if status != "recruiting" || locked {
		return service.ErrCarpoolUnavailable
	}
	if err := insertCarpoolInvite(ctx, tx, carpoolID, actorUserID, inviteHash, inviteHint); err != nil {
		return err
	}
	if err := insertCarpoolEvent(ctx, tx, carpoolID, actorUserID, "invite_created"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit carpool invite create: %w", err)
	}
	return nil
}

func (r *carpoolRepository) Join(ctx context.Context, carpoolID, userID int64, inviteHash *string) (*service.CarpoolMutationResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin carpool join: %w", err)
	}
	defer tx.Rollback()

	var inviteID *int64
	if inviteHash != nil {
		var resolvedInviteID int64
		err = tx.QueryRowContext(ctx, `
SELECT id, carpool_id
FROM carpool_invites
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > NOW())
  AND (max_uses = 0 OR use_count < max_uses)
FOR UPDATE`, *inviteHash).Scan(&resolvedInviteID, &carpoolID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrCarpoolInviteInvalid
		}
		if err != nil {
			return nil, fmt.Errorf("resolve carpool invite for join: %w", err)
		}
		inviteID = &resolvedInviteID
	}

	var status, visibility string
	var capacity int
	var locked bool
	err = tx.QueryRowContext(ctx, `
SELECT status, visibility, capacity, join_locked_at IS NOT NULL
FROM carpools WHERE id = $1 FOR UPDATE`, carpoolID).Scan(&status, &visibility, &capacity, &locked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load carpool for join: %w", err)
	}
	if status != "recruiting" || locked || (inviteID == nil && visibility != service.CarpoolVisibilityPublic) {
		return nil, service.ErrCarpoolUnavailable
	}

	var existingStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2`, carpoolID, userID).Scan(&existingStatus)
	if err == nil && (existingStatus == "joined" || existingStatus == "active") {
		return nil, service.ErrCarpoolAlreadyJoined
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("check existing carpool member: %w", err)
	}

	var memberCount int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM carpool_members
WHERE carpool_id = $1 AND status IN ('joined', 'active')`, carpoolID).Scan(&memberCount); err != nil {
		return nil, fmt.Errorf("count carpool members: %w", err)
	}
	if memberCount >= capacity {
		return nil, service.ErrCarpoolUnavailable
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO carpool_members (carpool_id, user_id, role, status, joined_via_invite_id, joined_at, updated_at)
VALUES ($1, $2, 'member', 'joined', $3, NOW(), NOW())
ON CONFLICT (carpool_id, user_id) DO UPDATE SET
    status = 'joined', joined_via_invite_id = EXCLUDED.joined_via_invite_id,
    joined_at = NOW(), left_at = NULL, removed_by_user_id = NULL,
    removal_reason = NULL, updated_at = NOW()`, carpoolID, userID, inviteID)
	if err != nil {
		return nil, fmt.Errorf("join carpool: %w", err)
	}
	if inviteID != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE carpool_invites SET use_count = use_count + 1, updated_at = NOW() WHERE id = $1`, *inviteID); err != nil {
			return nil, fmt.Errorf("consume carpool invite: %w", err)
		}
	}
	if err := insertCarpoolEvent(ctx, tx, carpoolID, userID, "member_joined"); err != nil {
		return nil, err
	}

	result := &service.CarpoolMutationResult{}
	memberCount++
	if memberCount == capacity {
		groupID, userIDs, err := launchCarpool(ctx, tx, carpoolID)
		if err != nil {
			return nil, err
		}
		result.ActivatedGroupID = groupID
		result.ActivatedUserIDs = userIDs
	}
	result.Carpool, err = getCarpoolByID(ctx, tx, carpoolID, userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit carpool join: %w", err)
	}
	return result, nil
}

func (r *carpoolRepository) Cancel(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin carpool cancel: %w", err)
	}
	defer tx.Rollback()

	var ownerUserID sql.NullInt64
	var status string
	err = tx.QueryRowContext(ctx, `SELECT owner_user_id, status FROM carpools WHERE id = $1 FOR UPDATE`, carpoolID).Scan(&ownerUserID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrCarpoolNotFound
	}
	if err != nil {
		return fmt.Errorf("load carpool for cancel: %w", err)
	}
	if !isAdmin && (!ownerUserID.Valid || ownerUserID.Int64 != actorUserID) {
		return service.ErrCarpoolForbidden
	}
	if status != "recruiting" && status != "starting" {
		return service.ErrCarpoolUnavailable
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE carpools SET status = 'cancelled', join_locked_at = NOW(), cancelled_at = NOW(),
    cancelled_by_user_id = $2, version = version + 1, updated_at = NOW()
WHERE id = $1`, carpoolID, actorUserID); err != nil {
		return fmt.Errorf("cancel carpool: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE carpool_members SET status = 'cancelled', updated_at = NOW() WHERE carpool_id = $1 AND status = 'joined'`, carpoolID); err != nil {
		return fmt.Errorf("cancel carpool members: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE carpool_invites SET revoked_at = NOW(), revoked_by_user_id = $2, updated_at = NOW() WHERE carpool_id = $1 AND revoked_at IS NULL`, carpoolID, actorUserID); err != nil {
		return fmt.Errorf("revoke carpool invites: %w", err)
	}
	if err := insertCarpoolEvent(ctx, tx, carpoolID, actorUserID, "cancelled"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit carpool cancel: %w", err)
	}
	return nil
}

func (r *carpoolRepository) SetJoinLocked(ctx context.Context, carpoolID, actorUserID int64, locked bool) error {
	var result sql.Result
	var err error
	if locked {
		result, err = r.db.ExecContext(ctx, `UPDATE carpools SET join_locked_at = NOW(), join_locked_by_user_id = $2, updated_at = NOW() WHERE id = $1 AND status = 'recruiting'`, carpoolID, actorUserID)
	} else {
		result, err = r.db.ExecContext(ctx, `UPDATE carpools SET join_locked_at = NULL, join_locked_by_user_id = NULL, updated_at = NOW() WHERE id = $1 AND status = 'recruiting'`, carpoolID)
	}
	if err != nil {
		return fmt.Errorf("set carpool join lock: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read carpool join lock result: %w", err)
	}
	if affected == 0 {
		return service.ErrCarpoolUnavailable
	}
	return nil
}

func (r *carpoolRepository) getByID(ctx context.Context, carpoolID, userID int64) (*service.Carpool, error) {
	return getCarpoolByID(ctx, r.db, carpoolID, userID)
}

type carpoolQueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getCarpoolByID(ctx context.Context, queryer carpoolQueryRower, carpoolID, userID int64) (*service.Carpool, error) {
	item, err := scanCarpool(queryer.QueryRowContext(ctx, carpoolSelectSQL+` WHERE c.id = $2`, userID, carpoolID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get carpool: %w", err)
	}
	return item, nil
}

func launchCarpool(ctx context.Context, tx *sql.Tx, carpoolID int64) (int64, []int64, error) {
	var name, description string
	var ownerUserID sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT name, description, owner_user_id FROM carpools WHERE id = $1`, carpoolID).Scan(&name, &description, &ownerUserID)
	if err != nil {
		return 0, nil, fmt.Errorf("load carpool launch data: %w", err)
	}

	var groupID int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO groups (
    name, description, platform, rate_multiplier, is_exclusive, status,
    subscription_type, daily_limit_usd, weekly_limit_usd, monthly_limit_usd,
    default_validity_days, require_oauth_only, supported_model_scopes
) VALUES ($1, $2, 'openai', 1, TRUE, 'active', 'subscription', NULL, NULL, NULL, 30, TRUE, '[]'::jsonb)
RETURNING id`, name, "Carpool subscription: "+description).Scan(&groupID)
	if err != nil {
		return 0, nil, translateCarpoolWriteError(err)
	}
	if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventGroupChanged, nil, &groupID, nil); err != nil {
		return 0, nil, fmt.Errorf("enqueue launched carpool group: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `SELECT id, user_id FROM carpool_members WHERE carpool_id = $1 AND status = 'joined' ORDER BY id`, carpoolID)
	if err != nil {
		return 0, nil, fmt.Errorf("list launching carpool members: %w", err)
	}
	type member struct{ id, userID int64 }
	members := make([]member, 0)
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.id, &m.userID); err != nil {
			rows.Close()
			return 0, nil, fmt.Errorf("scan launching carpool member: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, nil, fmt.Errorf("iterate launching carpool members: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, nil, fmt.Errorf("close launching carpool members: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.AddDate(0, 1, 0)
	userIDs := make([]int64, 0, len(members))
	for _, member := range members {
		var subscriptionID int64
		err := tx.QueryRowContext(ctx, `
INSERT INTO user_subscriptions (
    user_id, group_id, starts_at, expires_at, status, assigned_by, assigned_at, notes
) VALUES ($1, $2, $3, $4, 'active', $5, $3, $6)
RETURNING id`, member.userID, groupID, now, expiresAt, ownerUserID, "Automatically assigned when carpool became full").Scan(&subscriptionID)
		if err != nil {
			return 0, nil, fmt.Errorf("assign carpool subscription to user %d: %w", member.userID, err)
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE carpool_members SET status = 'active', subscription_id = $2,
    activated_at = $3, updated_at = $3 WHERE id = $1`, member.id, subscriptionID, now); err != nil {
			return 0, nil, fmt.Errorf("activate carpool member %d: %w", member.userID, err)
		}
		userIDs = append(userIDs, member.userID)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE carpools SET status = 'active', group_id = $2, join_locked_at = $3,
    launched_at = $3, version = version + 1, updated_at = $3 WHERE id = $1`, carpoolID, groupID, now); err != nil {
		return 0, nil, fmt.Errorf("activate carpool: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE carpool_invites SET revoked_at = $2, updated_at = $2 WHERE carpool_id = $1 AND revoked_at IS NULL`, carpoolID, now); err != nil {
		return 0, nil, fmt.Errorf("revoke launched carpool invites: %w", err)
	}
	if err := insertCarpoolEvent(ctx, tx, carpoolID, ownerUserID, "launched"); err != nil {
		return 0, nil, err
	}
	return groupID, userIDs, nil
}

func insertCarpoolInvite(ctx context.Context, tx *sql.Tx, carpoolID, actorUserID int64, hash, hint string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO carpool_invites (carpool_id, created_by_user_id, token_hash, token_hint)
VALUES ($1, $2, $3, $4)`, carpoolID, actorUserID, hash, hint)
	if err != nil {
		return fmt.Errorf("create carpool invite: %w", err)
	}
	return nil
}

func insertCarpoolEvent(ctx context.Context, tx *sql.Tx, carpoolID int64, actorUserID any, action string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO carpool_events (carpool_id, actor_user_id, action) VALUES ($1, $2, $3)`, carpoolID, actorUserID, action)
	if err != nil {
		return fmt.Errorf("create carpool event: %w", err)
	}
	return nil
}

func translateCarpoolWriteError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return service.ErrCarpoolNameConflict
	}
	return fmt.Errorf("persist carpool: %w", err)
}

const carpoolSelectSQL = `
SELECT
    c.id, c.name, c.description,
    COALESCE(NULLIF(u.username, ''), NULLIF(u.email, ''), CONCAT('User #', c.owner_user_id)),
    c.owner_user_id, c.platform, c.plan_type, c.car_type, c.level, c.capacity,
    (SELECT COUNT(*) FROM carpool_members counted_member
     WHERE counted_member.carpool_id = c.id AND counted_member.status IN ('joined', 'active')),
    c.base_fee_cny, c.usage_pool_cny_per_account, c.visibility, c.status,
    c.join_locked_at IS NOT NULL, c.scheduled_start_at, c.launched_at,
    c.group_id, g.name,
    (SELECT current_member.role FROM carpool_members current_member
     WHERE current_member.carpool_id = c.id AND current_member.user_id = $1
     LIMIT 1),
    c.created_at
FROM carpools c
LEFT JOIN users u ON u.id = c.owner_user_id
LEFT JOIN groups g ON g.id = c.group_id AND g.deleted_at IS NULL`

type carpoolScanner interface {
	Scan(dest ...any) error
}

func scanCarpool(scanner carpoolScanner) (*service.Carpool, error) {
	var item service.Carpool
	var ownerUserID, groupID sql.NullInt64
	var groupName, memberRole sql.NullString
	var scheduledStartAt, launchedAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.Name, &item.Description, &item.Organizer,
		&ownerUserID, &item.Platform, &item.PlanType, &item.CarType, &item.Level,
		&item.Capacity, &item.MemberCount, &item.BaseFeeCNY, &item.UsagePoolPerAccountCNY,
		&item.Visibility, &item.Status, &item.JoinLocked, &scheduledStartAt, &launchedAt,
		&groupID, &groupName, &memberRole, &item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if ownerUserID.Valid {
		item.OwnerUserID = &ownerUserID.Int64
	}
	if groupID.Valid {
		item.GroupID = &groupID.Int64
	}
	if groupName.Valid {
		item.GroupName = &groupName.String
	}
	if memberRole.Valid {
		item.MemberRole = &memberRole.String
	}
	if scheduledStartAt.Valid {
		item.ScheduledStartAt = &scheduledStartAt.Time
	}
	if launchedAt.Valid {
		item.LaunchedAt = &launchedAt.Time
	}
	return &item, nil
}

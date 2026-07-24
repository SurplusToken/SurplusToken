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

	var carpoolID int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO carpools (
    name, description, owner_user_id, platform, plan_type, car_type, level,
    visibility, scheduled_start_at,
    weekly_limit_usd, seat_fee_cny, usage_pool_cny, reserve_ratio,
    launch_min_ratio, launch_max_ratio
) VALUES ($1, $2, $3, 'openai', 'openai_pro', $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id`, input.Name, input.Description, ownerUserID, input.CarType, input.Level,
		input.Visibility, input.ScheduledStartAt, input.WeeklyLimitUSD, input.SeatFeeCNY,
		input.UsagePoolCNY, input.ReserveRatio, input.LaunchMinRatio, input.LaunchMaxRatio).Scan(&carpoolID)
	if err != nil {
		return nil, translateCarpoolWriteError(err)
	}

	// owner 申报（可选）：>0 时写入 owner 成员记录并按 1 人记账预付（设计文档 §4.1/§4.4）。
	var ownerPrepaid *float64
	if input.DeclaredWeeklyQuotaUSD > 0 {
		prepaid := service.CarpoolPrepaidCNY(input.SeatFeeCNY, input.UsagePoolCNY, input.WeeklyLimitUSD, input.DeclaredWeeklyQuotaUSD, 1)
		ownerPrepaid = &prepaid
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO carpool_members (carpool_id, user_id, role, status, declared_weekly_quota_usd, prepaid_amount_cny)
VALUES ($1, $2, 'owner', 'joined', $3, $4)`, carpoolID, ownerUserID, input.DeclaredWeeklyQuotaUSD, ownerPrepaid); err != nil {
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

func (r *carpoolRepository) Join(ctx context.Context, carpoolID, userID int64, declaredWeeklyQuotaUSD float64, inviteHash *string) (*service.CarpoolMutationResult, error) {
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
	var locked bool
	var weeklyLimitUSD, launchMaxRatio, seatFeeCNY, usagePoolCNY float64
	err = tx.QueryRowContext(ctx, `
SELECT status, visibility, join_locked_at IS NOT NULL,
    weekly_limit_usd, launch_max_ratio, seat_fee_cny, usage_pool_cny
FROM carpools WHERE id = $1 FOR UPDATE`, carpoolID).Scan(&status, &visibility, &locked,
		&weeklyLimitUSD, &launchMaxRatio, &seatFeeCNY, &usagePoolCNY)
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

	var declaredTotal float64
	var memberCount int
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members
WHERE carpool_id = $1 AND status IN ('joined', 'active')`, carpoolID).Scan(&declaredTotal, &memberCount); err != nil {
		return nil, fmt.Errorf("sum carpool declared quota: %w", err)
	}
	// 上车硬上限（设计文档 §4.1）：Σ申报 + 新申报 > 105%×周限额 时拒绝上车。
	if declaredTotal+declaredWeeklyQuotaUSD > launchMaxRatio*weeklyLimitUSD {
		return nil, service.ErrCarpoolQuotaExceeded
	}

	// 上车即记账（设计文档 §4.4）：预付 = 席位费/人数 + 变动池×(申报/周限额)。
	prepaidAmountCNY := service.CarpoolPrepaidCNY(seatFeeCNY, usagePoolCNY, weeklyLimitUSD, declaredWeeklyQuotaUSD, memberCount+1)

	_, err = tx.ExecContext(ctx, `
INSERT INTO carpool_members (carpool_id, user_id, role, status, joined_via_invite_id,
    declared_weekly_quota_usd, prepaid_amount_cny, joined_at, updated_at)
VALUES ($1, $2, 'member', 'joined', $3, $4, $5, NOW(), NOW())
ON CONFLICT (carpool_id, user_id) DO UPDATE SET
    status = 'joined', joined_via_invite_id = EXCLUDED.joined_via_invite_id,
    declared_weekly_quota_usd = EXCLUDED.declared_weekly_quota_usd,
    prepaid_amount_cny = EXCLUDED.prepaid_amount_cny,
    joined_at = NOW(), left_at = NULL, removed_by_user_id = NULL,
    removal_reason = NULL, updated_at = NOW()`, carpoolID, userID, inviteID, declaredWeeklyQuotaUSD, prepaidAmountCNY)
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

	result := &service.CarpoolMutationResult{
		DeclaredWeeklyQuotaUSD: declaredWeeklyQuotaUSD,
		PrepaidAmountCNY:       prepaidAmountCNY,
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

// Launch 手动发车（设计文档 §4.3）：仅 recruiting 状态可发车，Σ申报须进入
// [launch_min, launch_max]×周限额 区间；force=true 时下限放宽到 80%（降档发车）。
func (r *carpoolRepository) Launch(ctx context.Context, carpoolID, actorUserID int64, isAdmin, force bool) (*service.CarpoolMutationResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin carpool launch: %w", err)
	}
	defer tx.Rollback()

	var ownerUserID sql.NullInt64
	var status string
	var params carpoolLaunchParams
	var launchMinRatio, launchMaxRatio float64
	err = tx.QueryRowContext(ctx, `
SELECT owner_user_id, status, weekly_limit_usd, seat_fee_cny, usage_pool_cny,
    reserve_ratio, launch_min_ratio, launch_max_ratio
FROM carpools WHERE id = $1 FOR UPDATE`, carpoolID).Scan(&ownerUserID, &status,
		&params.weeklyLimitUSD, &params.seatFeeCNY, &params.usagePoolCNY,
		&params.reserveRatio, &launchMinRatio, &launchMaxRatio)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load carpool for launch: %w", err)
	}
	if !isAdmin && (!ownerUserID.Valid || ownerUserID.Int64 != actorUserID) {
		return nil, service.ErrCarpoolForbidden
	}
	if status != "recruiting" {
		return nil, service.ErrCarpoolUnavailable
	}

	var declaredTotal float64
	var memberCount int
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members
WHERE carpool_id = $1 AND status = 'joined'`, carpoolID).Scan(&declaredTotal, &memberCount); err != nil {
		return nil, fmt.Errorf("sum carpool declared quota for launch: %w", err)
	}

	minTotal := launchMinRatio * params.weeklyLimitUSD
	if force {
		minTotal = service.CarpoolForceLaunchMinRatio * params.weeklyLimitUSD
	}
	maxTotal := launchMaxRatio * params.weeklyLimitUSD
	if memberCount == 0 || declaredTotal < minTotal || declaredTotal > maxTotal {
		return nil, service.ErrCarpoolLaunchNotReady
	}

	groupID, userIDs, err := launchCarpool(ctx, tx, carpoolID, params)
	if err != nil {
		return nil, err
	}
	result := &service.CarpoolMutationResult{ActivatedGroupID: groupID, ActivatedUserIDs: userIDs}
	result.Carpool, err = getCarpoolByID(ctx, tx, carpoolID, actorUserID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit carpool launch: %w", err)
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

func (r *carpoolRepository) GetByID(ctx context.Context, carpoolID, userID int64) (*service.Carpool, error) {
	return getCarpoolByID(ctx, r.db, carpoolID, userID)
}

func (r *carpoolRepository) ListSettlementMembers(ctx context.Context, carpoolID int64) ([]service.CarpoolSettlementMemberRow, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT m.user_id, m.role, m.declared_weekly_quota_usd, COALESCE(m.prepaid_amount_cny, 0),
    COALESCE(s.monthly_usage_usd, 0), s.starts_at, s.expires_at
FROM carpool_members m
LEFT JOIN user_subscriptions s ON s.id = m.subscription_id
WHERE m.carpool_id = $1 AND m.status IN ('joined', 'active')
ORDER BY m.id`, carpoolID)
	if err != nil {
		return nil, fmt.Errorf("list carpool settlement members: %w", err)
	}
	defer rows.Close()

	items := make([]service.CarpoolSettlementMemberRow, 0)
	for rows.Next() {
		var item service.CarpoolSettlementMemberRow
		var startsAt, expiresAt sql.NullTime
		if err := rows.Scan(&item.UserID, &item.Role, &item.DeclaredWeeklyQuotaUSD,
			&item.PrepaidAmountCNY, &item.ActualUsageUSD, &startsAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan carpool settlement member: %w", err)
		}
		if startsAt.Valid {
			item.PeriodStart = &startsAt.Time
		}
		if expiresAt.Valid {
			item.PeriodEnd = &expiresAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate carpool settlement members: %w", err)
	}
	return items, nil
}

// GetRecentWeeklyUsageStats 聚合本人最近 7 天的用量（USD）与有记录的天数，
// 供申报推荐（设计文档 §4.1）使用。
func (r *carpoolRepository) GetRecentWeeklyUsageStats(ctx context.Context, userID int64) (float64, int, error) {
	var totalUSD float64
	var days int
	err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(actual_cost), 0),
    COUNT(DISTINCT (created_at AT TIME ZONE 'UTC')::date)
FROM usage_logs
WHERE user_id = $1 AND created_at >= NOW() - INTERVAL '7 days'`, userID).Scan(&totalUSD, &days)
	if err != nil {
		return 0, 0, fmt.Errorf("aggregate recent weekly usage: %w", err)
	}
	return totalUSD, days, nil
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

// carpoolLaunchParams 是发车时锁定到 group/订阅上的额度池参数。
type carpoolLaunchParams struct {
	weeklyLimitUSD float64
	seatFeeCNY     float64
	usagePoolCNY   float64
	reserveRatio   float64
}

// launchCarpool 在事务内完成发车：创建带周限额安全帽的订阅分组，为每位成员
// 创建写入订阅级周限额（reserve×申报 + 公共池 C）的订阅，并按发车人数锁定预付。
func launchCarpool(ctx context.Context, tx *sql.Tx, carpoolID int64, params carpoolLaunchParams) (int64, []int64, error) {
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
) VALUES ($1, $2, 'openai', 1, TRUE, 'active', 'subscription', NULL, $3, NULL, 30, TRUE, '[]'::jsonb)
RETURNING id`, name, "Carpool subscription: "+description, params.weeklyLimitUSD).Scan(&groupID)
	if err != nil {
		return 0, nil, translateCarpoolWriteError(err)
	}
	if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventGroupChanged, nil, &groupID, nil); err != nil {
		return 0, nil, fmt.Errorf("enqueue launched carpool group: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `SELECT id, user_id, declared_weekly_quota_usd FROM carpool_members WHERE carpool_id = $1 AND status = 'joined' ORDER BY id`, carpoolID)
	if err != nil {
		return 0, nil, fmt.Errorf("list launching carpool members: %w", err)
	}
	type member struct {
		id, userID int64
		declared   float64
	}
	members := make([]member, 0)
	declaredTotal := 0.0
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.id, &m.userID, &m.declared); err != nil {
			rows.Close()
			return 0, nil, fmt.Errorf("scan launching carpool member: %w", err)
		}
		declaredTotal += m.declared
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
		// 订阅级周限额 = reserve×申报 + 公共池 C（设计文档 §4.2）。
		weeklyLimitUSD := service.CarpoolMemberWeeklyLimitUSD(params.weeklyLimitUSD, params.reserveRatio, member.declared, declaredTotal)
		var subscriptionID int64
		err := tx.QueryRowContext(ctx, `
INSERT INTO user_subscriptions (
    user_id, group_id, starts_at, expires_at, status, assigned_by, assigned_at, notes, weekly_limit_usd
) VALUES ($1, $2, $3, $4, 'active', $5, $3, $6, $7)
RETURNING id`, member.userID, groupID, now, expiresAt, ownerUserID, "Automatically assigned when carpool launched", weeklyLimitUSD).Scan(&subscriptionID)
		if err != nil {
			return 0, nil, fmt.Errorf("assign carpool subscription to user %d: %w", member.userID, err)
		}
		// 预付按发车时人数锁定（设计文档 §4.4）。
		prepaidAmountCNY := service.CarpoolPrepaidCNY(params.seatFeeCNY, params.usagePoolCNY, params.weeklyLimitUSD, member.declared, len(members))
		if _, err := tx.ExecContext(ctx, `
UPDATE carpool_members SET status = 'active', subscription_id = $2,
    prepaid_amount_cny = $3, activated_at = $4, updated_at = $4 WHERE id = $1`, member.id, subscriptionID, prepaidAmountCNY, now); err != nil {
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
    c.created_at,
    c.weekly_limit_usd, c.seat_fee_cny, c.usage_pool_cny, c.reserve_ratio,
    c.launch_min_ratio, c.launch_max_ratio,
    (SELECT COALESCE(SUM(declared_member.declared_weekly_quota_usd), 0)
     FROM carpool_members declared_member
     WHERE declared_member.carpool_id = c.id AND declared_member.status IN ('joined', 'active'))
FROM carpools c
LEFT JOIN users u ON u.id = c.owner_user_id
LEFT JOIN groups g ON g.id = c.group_id AND g.deleted_at IS NULL`

type carpoolScanner interface {
	Scan(dest ...any) error
}

func scanCarpool(scanner carpoolScanner) (*service.Carpool, error) {
	var item service.Carpool
	var ownerUserID, groupID sql.NullInt64
	var capacity sql.NullInt64
	var groupName, memberRole sql.NullString
	var scheduledStartAt, launchedAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.Name, &item.Description, &item.Organizer,
		&ownerUserID, &item.Platform, &item.PlanType, &item.CarType, &item.Level,
		&capacity, &item.MemberCount, &item.BaseFeeCNY, &item.UsagePoolPerAccountCNY,
		&item.Visibility, &item.Status, &item.JoinLocked, &scheduledStartAt, &launchedAt,
		&groupID, &groupName, &memberRole, &item.CreatedAt,
		&item.WeeklyLimitUSD, &item.SeatFeeCNY, &item.UsagePoolCNY, &item.ReserveRatio,
		&item.LaunchMinRatio, &item.LaunchMaxRatio, &item.DeclaredTotalUSD,
	)
	if err != nil {
		return nil, err
	}
	if ownerUserID.Valid {
		item.OwnerUserID = &ownerUserID.Int64
	}
	if capacity.Valid {
		item.Capacity = int(capacity.Int64)
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

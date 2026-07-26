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
    OR EXISTS(SELECT 1 FROM carpools WHERE name = $1 AND status IN ('recruiting', 'confirmed', 'starting', 'active'))`, input.Name).Scan(&groupNameExists); err != nil {
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
    launch_min_ratio, launch_max_ratio,
    added_admin_wechat, group_qr_code, group_qr_code_content_type
) VALUES ($1, $2, $3, 'openai', 'openai_pro', $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING id`, input.Name, input.Description, ownerUserID, input.CarType, input.Level,
		input.Visibility, input.ScheduledStartAt, input.WeeklyLimitUSD, input.SeatFeeCNY,
		input.UsagePoolCNY, input.ReserveRatio, input.LaunchMinRatio, input.LaunchMaxRatio,
		input.AddedAdminWechat, input.GroupQRCodeBytes, input.GroupQRCodeContentType).Scan(&carpoolID)
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
INSERT INTO carpool_members (carpool_id, user_id, role, status, declared_weekly_quota_usd,
    prepaid_amount_cny, quoted_prepaid_amount_cny)
VALUES ($1, $2, 'owner', 'joined', $3, $4, $4)`, carpoolID, ownerUserID, input.DeclaredWeeklyQuotaUSD, ownerPrepaid); err != nil {
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

func (r *carpoolRepository) Join(ctx context.Context, carpoolID, userID int64, declaredWeeklyQuotaUSD float64, joinedWechatGroup bool, inviteHash *string) (*service.CarpoolMutationResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin carpool join: %w", err)
	}
	defer tx.Rollback()

	// 锁顺序约定：carpools → carpool_invites（父表在前）。Cancel 走的就是这个顺序，
	// 本函数原来反着来（先锁邀请行再锁车行），两条路径并发时构成 AB-BA 死锁。
	// 因此这里先不加锁地解析邀请拿到 carpool_id，锁住车行之后再回头锁邀请行并复核。
	if inviteHash != nil {
		err = tx.QueryRowContext(ctx, `
SELECT carpool_id FROM carpool_invites WHERE token_hash = $1`, *inviteHash).Scan(&carpoolID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrCarpoolInviteInvalid
		}
		if err != nil {
			return nil, fmt.Errorf("resolve carpool invite for join: %w", err)
		}
	}

	var status, visibility string
	var locked bool
	var weeklyLimitUSD, launchMinRatio, launchMaxRatio, seatFeeCNY, usagePoolCNY float64
	err = tx.QueryRowContext(ctx, `
SELECT status, visibility, join_locked_at IS NOT NULL,
    weekly_limit_usd, launch_min_ratio, launch_max_ratio, seat_fee_cny, usage_pool_cny
FROM carpools WHERE id = $1 FOR UPDATE`, carpoolID).Scan(&status, &visibility, &locked,
		&weeklyLimitUSD, &launchMinRatio, &launchMaxRatio, &seatFeeCNY, &usagePoolCNY)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load carpool for join: %w", err)
	}

	// 车行已锁，此时才锁邀请行——有效性（撤销/过期/用完）在锁下复核一次，
	// 保证与原来 FOR UPDATE 解析等价的并发语义。
	var inviteID *int64
	if inviteHash != nil {
		var resolvedInviteID, resolvedCarpoolID int64
		err = tx.QueryRowContext(ctx, `
SELECT id, carpool_id
FROM carpool_invites
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > NOW())
  AND (max_uses = 0 OR use_count < max_uses)
FOR UPDATE`, *inviteHash).Scan(&resolvedInviteID, &resolvedCarpoolID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrCarpoolInviteInvalid
		}
		if err != nil {
			return nil, fmt.Errorf("lock carpool invite for join: %w", err)
		}
		if resolvedCarpoolID != carpoolID {
			return nil, service.ErrCarpoolInviteInvalid
		}
		inviteID = &resolvedInviteID
	}

	// confirmed 全锁：仅 recruiting 可上车。
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
	// 成员数硬上限：发车是单事务、逐成员建订阅，成员数直接决定事务大小。
	if memberCount >= service.CarpoolMaxMembers {
		return nil, service.ErrCarpoolFull
	}
	// 上车硬上限（设计文档 §4.1）：Σ申报 + 新申报 > 105%×周限额 时拒绝上车。
	if declaredTotal+declaredWeeklyQuotaUSD > launchMaxRatio*weeklyLimitUSD {
		return nil, service.ErrCarpoolQuotaExceeded
	}

	// 上车即记账（设计文档 §4.4）：预付 = 席位费/人数 + 变动池×(申报/周限额)。
	// 这个数字是按"上车当时的人数"报给用户、并据此实际收款的，因此同时写入
	// quoted_prepaid_amount_cny 永久留档；prepaid_amount_cny 会在发车时被按
	// 发车人数重写，两者之差就是结算时席位费要找齐的钱。
	prepaidAmountCNY := service.CarpoolPrepaidCNY(seatFeeCNY, usagePoolCNY, weeklyLimitUSD, declaredWeeklyQuotaUSD, memberCount+1)

	_, err = tx.ExecContext(ctx, `
INSERT INTO carpool_members (carpool_id, user_id, role, status, joined_via_invite_id,
    declared_weekly_quota_usd, prepaid_amount_cny, quoted_prepaid_amount_cny,
    joined_wechat_group, joined_at, updated_at)
VALUES ($1, $2, 'member', 'joined', $3, $4, $5, $5, $6, NOW(), NOW())
ON CONFLICT (carpool_id, user_id) DO UPDATE SET
    status = 'joined', joined_via_invite_id = EXCLUDED.joined_via_invite_id,
    declared_weekly_quota_usd = EXCLUDED.declared_weekly_quota_usd,
    prepaid_amount_cny = EXCLUDED.prepaid_amount_cny,
    quoted_prepaid_amount_cny = EXCLUDED.quoted_prepaid_amount_cny,
    joined_wechat_group = EXCLUDED.joined_wechat_group,
    joined_at = NOW(), left_at = NULL, removed_by_user_id = NULL,
    removal_reason = NULL, updated_at = NOW()`, carpoolID, userID, inviteID, declaredWeeklyQuotaUSD, prepaidAmountCNY, joinedWechatGroup)
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
	// 进区间通知：Σ申报 首次进入 [launch_min, launch_max]×周限额 时，同事务置
	// launch_notified_at；service 在提交后据此通知车主确认发车。
	newDeclaredTotal := declaredTotal + declaredWeeklyQuotaUSD
	if newDeclaredTotal >= launchMinRatio*weeklyLimitUSD && newDeclaredTotal <= launchMaxRatio*weeklyLimitUSD {
		res, err := tx.ExecContext(ctx, `
UPDATE carpools SET launch_notified_at = NOW(), updated_at = NOW()
WHERE id = $1 AND launch_notified_at IS NULL`, carpoolID)
		if err != nil {
			return nil, fmt.Errorf("mark carpool launch notified: %w", err)
		}
		if affected, err := res.RowsAffected(); err == nil && affected > 0 {
			result.LaunchBandEntered = true
		}
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

// Leave 下车：仅 recruiting 状态；同事务（FOR UPDATE 锁车行）把成员行置为 left。
// Σ申报 统计只算 joined/active 成员，故下车即释放额度；若因此跌出发车区间，
// 把 launch_notified_at 重置为 NULL（下次再进区间可重新通知车主）。操作幂等：
// 已 left 的成员重复下车返回同样的成功结果。
func (r *carpoolRepository) Leave(ctx context.Context, carpoolID, userID int64) (*service.CarpoolMutationResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin carpool leave: %w", err)
	}
	defer tx.Rollback()

	var ownerUserID sql.NullInt64
	var status string
	var weeklyLimitUSD, launchMinRatio, launchMaxRatio float64
	var launchNotifiedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
SELECT owner_user_id, status, weekly_limit_usd, launch_min_ratio, launch_max_ratio, launch_notified_at
FROM carpools WHERE id = $1 FOR UPDATE`, carpoolID).Scan(&ownerUserID, &status,
		&weeklyLimitUSD, &launchMinRatio, &launchMaxRatio, &launchNotifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load carpool for leave: %w", err)
	}
	// confirmed/active/cancelled 等状态一律 409。
	if status != "recruiting" {
		return nil, service.ErrCarpoolUnavailable
	}
	// 车主不能下车，只能取消整车。
	if ownerUserID.Valid && ownerUserID.Int64 == userID {
		return nil, service.ErrCarpoolOwnerCannotLeave
	}

	var memberStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM carpool_members WHERE carpool_id = $1 AND user_id = $2`, carpoolID, userID).Scan(&memberStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolNotMember
	}
	if err != nil {
		return nil, fmt.Errorf("load carpool member for leave: %w", err)
	}
	if memberStatus != "left" {
		if memberStatus != "joined" {
			return nil, service.ErrCarpoolUnavailable
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE carpool_members SET status = 'left', left_at = NOW(), updated_at = NOW()
WHERE carpool_id = $1 AND user_id = $2 AND status = 'joined'`, carpoolID, userID); err != nil {
			return nil, fmt.Errorf("leave carpool: %w", err)
		}
		if err := insertCarpoolEvent(ctx, tx, carpoolID, userID, "member_left"); err != nil {
			return nil, err
		}
		// 下车释放申报额度：跌出发车区间时重置 launch_notified_at。
		if launchNotifiedAt.Valid {
			var declaredTotal float64
			if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(declared_weekly_quota_usd), 0) FROM carpool_members
WHERE carpool_id = $1 AND status IN ('joined', 'active')`, carpoolID).Scan(&declaredTotal); err != nil {
				return nil, fmt.Errorf("sum carpool declared quota after leave: %w", err)
			}
			if declaredTotal < launchMinRatio*weeklyLimitUSD || declaredTotal > launchMaxRatio*weeklyLimitUSD {
				if _, err := tx.ExecContext(ctx, `
UPDATE carpools SET launch_notified_at = NULL, updated_at = NOW() WHERE id = $1`, carpoolID); err != nil {
					return nil, fmt.Errorf("reset carpool launch notified: %w", err)
				}
			}
		}
	}

	item, err := getCarpoolByID(ctx, tx, carpoolID, userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit carpool leave: %w", err)
	}
	return &service.CarpoolMutationResult{Carpool: item}, nil
}

// Confirm 车主确认发车（两段确认第一段）：仅 owner、recruiting、Σ申报在
// [launch_min, launch_max]×周限额 区间内；确认后状态置 confirmed 并记 confirmed_at/by。
func (r *carpoolRepository) Confirm(ctx context.Context, carpoolID, ownerUserID int64) (*service.CarpoolMutationResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin carpool confirm: %w", err)
	}
	defer tx.Rollback()

	var carpoolOwner sql.NullInt64
	var status string
	var weeklyLimitUSD, launchMinRatio, launchMaxRatio float64
	err = tx.QueryRowContext(ctx, `
SELECT owner_user_id, status, weekly_limit_usd, launch_min_ratio, launch_max_ratio
FROM carpools WHERE id = $1 FOR UPDATE`, carpoolID).Scan(&carpoolOwner, &status,
		&weeklyLimitUSD, &launchMinRatio, &launchMaxRatio)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load carpool for confirm: %w", err)
	}
	if !carpoolOwner.Valid || carpoolOwner.Int64 != ownerUserID {
		return nil, service.ErrCarpoolForbidden
	}
	if status != "recruiting" {
		return nil, service.ErrCarpoolUnavailable
	}

	var declaredTotal float64
	var memberCount int
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(declared_weekly_quota_usd), 0), COUNT(*) FROM carpool_members
WHERE carpool_id = $1 AND status IN ('joined', 'active')`, carpoolID).Scan(&declaredTotal, &memberCount); err != nil {
		return nil, fmt.Errorf("sum carpool declared quota for confirm: %w", err)
	}
	if memberCount == 0 || declaredTotal < launchMinRatio*weeklyLimitUSD || declaredTotal > launchMaxRatio*weeklyLimitUSD {
		return nil, service.ErrCarpoolLaunchNotReady
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE carpools SET status = 'confirmed', confirmed_at = NOW(), confirmed_by = $2,
    version = version + 1, updated_at = NOW()
WHERE id = $1`, carpoolID, ownerUserID); err != nil {
		return nil, fmt.Errorf("confirm carpool: %w", err)
	}
	if err := insertCarpoolEvent(ctx, tx, carpoolID, ownerUserID, "confirmed"); err != nil {
		return nil, err
	}

	item, err := getCarpoolByID(ctx, tx, carpoolID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit carpool confirm: %w", err)
	}
	return &service.CarpoolMutationResult{Carpool: item}, nil
}

// Unconfirm 撤回确认（confirmed → recruiting）：车主或 admin 可用。
// 清空 confirmed_at/confirmed_by 与 launch_notified_at——后者清空后，Σ申报 下次
// 再进入发车区间时会重新通知车主，避免"通知只发一次、车却回到招募态"的死角。
func (r *carpoolRepository) Unconfirm(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*service.CarpoolMutationResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin carpool unconfirm: %w", err)
	}
	defer tx.Rollback()

	var ownerUserID sql.NullInt64
	var status string
	err = tx.QueryRowContext(ctx, `
SELECT owner_user_id, status FROM carpools WHERE id = $1 FOR UPDATE`, carpoolID).Scan(&ownerUserID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load carpool for unconfirm: %w", err)
	}
	if !isAdmin && (!ownerUserID.Valid || ownerUserID.Int64 != actorUserID) {
		return nil, service.ErrCarpoolForbidden
	}
	if status != "confirmed" {
		return nil, service.ErrCarpoolUnavailable
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE carpools SET status = 'recruiting', confirmed_at = NULL, confirmed_by = NULL,
    launch_notified_at = NULL, version = version + 1, updated_at = NOW()
WHERE id = $1`, carpoolID); err != nil {
		return nil, fmt.Errorf("unconfirm carpool: %w", err)
	}
	if err := insertCarpoolEvent(ctx, tx, carpoolID, actorUserID, "unconfirmed"); err != nil {
		return nil, err
	}
	item, err := getCarpoolByID(ctx, tx, carpoolID, actorUserID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit carpool unconfirm: %w", err)
	}
	return &service.CarpoolMutationResult{Carpool: item}, nil
}

// ListPendingLaunch 列出全部等待 admin 启动的车（status = confirmed），最早确认的在前。
func (r *carpoolRepository) ListPendingLaunch(ctx context.Context) ([]service.CarpoolPendingLaunch, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT c.id, c.name, c.owner_user_id, COALESCE(u.email, ''),
    (SELECT COUNT(*) FROM carpool_members m
     WHERE m.carpool_id = c.id AND m.status IN ('joined', 'active')),
    (SELECT COALESCE(SUM(m.declared_weekly_quota_usd), 0) FROM carpool_members m
     WHERE m.carpool_id = c.id AND m.status IN ('joined', 'active')),
    c.weekly_limit_usd, c.confirmed_at,
    EXTRACT(EPOCH FROM (NOW() - c.confirmed_at)) / 3600
FROM carpools c
LEFT JOIN users u ON u.id = c.owner_user_id
WHERE c.status = 'confirmed' AND c.confirmed_at IS NOT NULL
ORDER BY c.confirmed_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list pending launch carpools: %w", err)
	}
	defer rows.Close()

	items := make([]service.CarpoolPendingLaunch, 0)
	for rows.Next() {
		var item service.CarpoolPendingLaunch
		var ownerUserID sql.NullInt64
		if err := rows.Scan(&item.CarpoolID, &item.Name, &ownerUserID, &item.OwnerEmail,
			&item.MemberCount, &item.DeclaredTotalUSD, &item.WeeklyLimitUSD,
			&item.ConfirmedAt, &item.PendingHours); err != nil {
			return nil, fmt.Errorf("scan pending launch carpool: %w", err)
		}
		if ownerUserID.Valid {
			item.OwnerUserID = &ownerUserID.Int64
		}
		item.Overdue = item.PendingHours > service.CarpoolLaunchSLAHours
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending launch carpools: %w", err)
	}
	return items, nil
}

// Launch 管理员启动发车（两段确认第二段）：仅 admin。正常发车要求车已 confirmed
// （owner 已在发车区间内确认）；force=true 用于招募不足的降档发车，要求 recruiting
// 且 Σ申报 ≥ 80%×周限额，跳过确认流程。
func (r *carpoolRepository) Launch(ctx context.Context, carpoolID, actorUserID int64, isAdmin, force bool) (*service.CarpoolMutationResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin carpool launch: %w", err)
	}
	defer tx.Rollback()

	var status string
	var params carpoolLaunchParams
	var launchMinRatio, launchMaxRatio float64
	err = tx.QueryRowContext(ctx, `
SELECT status, weekly_limit_usd, seat_fee_cny, usage_pool_cny,
    reserve_ratio, launch_min_ratio, launch_max_ratio
FROM carpools WHERE id = $1 FOR UPDATE`, carpoolID).Scan(&status,
		&params.weeklyLimitUSD, &params.seatFeeCNY, &params.usagePoolCNY,
		&params.reserveRatio, &launchMinRatio, &launchMaxRatio)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCarpoolNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load carpool for launch: %w", err)
	}
	// 发车入口仅 admin（上一轮 owner 可直接 launch 的行为已被两段确认取代）。
	if !isAdmin {
		return nil, service.ErrCarpoolForbidden
	}
	if force {
		// force 降档发车：面向 recruiting 招募不足（Σ≥80%）的车，跳过确认流程。
		if status != "recruiting" {
			return nil, service.ErrCarpoolUnavailable
		}
	} else if status != "confirmed" {
		return nil, service.ErrCarpoolNotConfirmed
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
	// confirmed 全锁：仅 admin 可强制取消；其余入口仅 recruiting/starting 可取消。
	if status == "confirmed" {
		if !isAdmin {
			return service.ErrCarpoolUnavailable
		}
	} else if status != "recruiting" && status != "starting" {
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

// GetGroupQRCode 读取车辆的微信群二维码字节与内容类型。
func (r *carpoolRepository) GetGroupQRCode(ctx context.Context, carpoolID int64) ([]byte, string, error) {
	var data []byte
	var contentType sql.NullString
	err := r.db.QueryRowContext(ctx, `
SELECT group_qr_code, group_qr_code_content_type FROM carpools WHERE id = $1`, carpoolID).Scan(&data, &contentType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", service.ErrCarpoolNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("get carpool group qr code: %w", err)
	}
	if len(data) == 0 {
		return nil, "", service.ErrCarpoolQRCodeNotFound
	}
	if !contentType.Valid || contentType.String == "" {
		contentType = sql.NullString{String: "application/octet-stream", Valid: true}
	}
	return data, contentType.String, nil
}

func (r *carpoolRepository) ListSettlementMembers(ctx context.Context, carpoolID int64) ([]service.CarpoolSettlementMemberRow, error) {
	// 带上邮箱/用户名：结算单里只有 #userId 的话，车主没法把每一行对应到
	// 微信群里的真人去收款/退款。可见性由 service 层控制（仅 owner/admin 全量）。
	// 末尾一组 settled_* 是结算冻结快照（migration 191），未结算时全为 NULL。
	rows, err := r.db.QueryContext(ctx, `
SELECT m.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
    m.role, m.declared_weekly_quota_usd, COALESCE(m.prepaid_amount_cny, 0),
    COALESCE(m.quoted_prepaid_amount_cny, m.prepaid_amount_cny, 0),
    COALESCE(s.monthly_usage_usd, 0), s.starts_at, s.expires_at,
    m.settled_at, m.settled_floor_usage_usd, m.settled_actual_usage_usd,
    m.settled_billable_usage_usd, m.settled_usage_share_cny,
    m.settled_seat_fee_cny, m.settled_total_delta_cny
FROM carpool_members m
LEFT JOIN users u ON u.id = m.user_id
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
		var startsAt, expiresAt, settledAt sql.NullTime
		var floorUsage, actualUsage, billableUsage, usageShare, seatFee, totalDelta sql.NullFloat64
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username,
			&item.Role, &item.DeclaredWeeklyQuotaUSD,
			&item.PrepaidAmountCNY, &item.QuotedPrepaidCNY,
			&item.ActualUsageUSD, &startsAt, &expiresAt,
			&settledAt, &floorUsage, &actualUsage,
			&billableUsage, &usageShare, &seatFee, &totalDelta); err != nil {
			return nil, fmt.Errorf("scan carpool settlement member: %w", err)
		}
		if startsAt.Valid {
			item.PeriodStart = &startsAt.Time
		}
		if expiresAt.Valid {
			item.PeriodEnd = &expiresAt.Time
		}
		// 快照要求 settled_at 与全部金额列都在——缺任何一列都按"未冻结"处理，
		// 宁可显示实时值，也不要拼出一份半真半假的账单。
		if settledAt.Valid && floorUsage.Valid && actualUsage.Valid && billableUsage.Valid &&
			usageShare.Valid && seatFee.Valid && totalDelta.Valid {
			item.Frozen = &service.CarpoolSettlementFrozenRow{
				FloorUsageUSD:    floorUsage.Float64,
				ActualUsageUSD:   actualUsage.Float64,
				BillableUsageUSD: billableUsage.Float64,
				UsageShareCNY:    usageShare.Float64,
				SeatFeeCNY:       seatFee.Float64,
				TotalDeltaCNY:    totalDelta.Float64,
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate carpool settlement members: %w", err)
	}
	return items, nil
}

// PersistSettlement 冻结结算单：在一个事务里给车打上 settled_at 并写入每位成员的
// 金额快照。车行的 `settled_at IS NULL` 是幂等守卫——两个人同时点"结算"时，
// 后到的那个拿到 ErrCarpoolAlreadySettled，不会覆盖已经冻结的账单。
func (r *carpoolRepository) PersistSettlement(ctx context.Context, carpoolID, actorUserID int64, members []service.CarpoolSettlementMember) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin carpool settle: %w", err)
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM carpools WHERE id = $1 FOR UPDATE`, carpoolID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrCarpoolNotFound
	}
	if err != nil {
		return fmt.Errorf("load carpool for settle: %w", err)
	}
	if status != "active" && status != "ended" {
		return service.ErrCarpoolNotSettleable
	}

	res, err := tx.ExecContext(ctx, `
UPDATE carpools SET settled_at = NOW(), settled_by_user_id = $2, updated_at = NOW()
WHERE id = $1 AND settled_at IS NULL`, carpoolID, actorUserID)
	if err != nil {
		return fmt.Errorf("mark carpool settled: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read carpool settle result: %w", err)
	}
	if affected == 0 {
		return service.ErrCarpoolAlreadySettled
	}

	for _, member := range members {
		if _, err := tx.ExecContext(ctx, `
UPDATE carpool_members SET settled_at = NOW(), settled_by_user_id = $3,
    settled_floor_usage_usd = $4, settled_actual_usage_usd = $5,
    settled_billable_usage_usd = $6, settled_usage_share_cny = $7,
    settled_seat_fee_cny = $8, settled_total_delta_cny = $9, updated_at = NOW()
WHERE carpool_id = $1 AND user_id = $2 AND status IN ('joined', 'active')`,
			carpoolID, member.UserID, actorUserID,
			member.FloorUsageUSD, member.ActualUsageUSD, member.BillableUsageUSD,
			member.UsageFinalShareCNY, member.SeatFeeFinalCNY, member.TotalDeltaCNY); err != nil {
			return fmt.Errorf("freeze carpool settlement for member %d: %w", member.UserID, err)
		}
	}
	if err := insertCarpoolEvent(ctx, tx, carpoolID, actorUserID, "settled"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit carpool settle: %w", err)
	}
	return nil
}

// ClearSettlement 撤销结算：清空车与全部成员上的冻结字段，结算单回到实时预览。
func (r *carpoolRepository) ClearSettlement(ctx context.Context, carpoolID, actorUserID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin carpool unsettle: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
UPDATE carpools SET settled_at = NULL, settled_by_user_id = NULL, updated_at = NOW()
WHERE id = $1 AND settled_at IS NOT NULL`, carpoolID)
	if err != nil {
		return fmt.Errorf("clear carpool settled marker: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read carpool unsettle result: %w", err)
	}
	if affected == 0 {
		// 车不存在或本来就没结算，两种情况都不该静默成功。
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM carpools WHERE id = $1)`, carpoolID).Scan(&exists); err != nil {
			return fmt.Errorf("check carpool for unsettle: %w", err)
		}
		if !exists {
			return service.ErrCarpoolNotFound
		}
		return service.ErrCarpoolNotSettled
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE carpool_members SET settled_at = NULL, settled_by_user_id = NULL,
    settled_floor_usage_usd = NULL, settled_actual_usage_usd = NULL,
    settled_billable_usage_usd = NULL, settled_usage_share_cny = NULL,
    settled_seat_fee_cny = NULL, settled_total_delta_cny = NULL, updated_at = NOW()
WHERE carpool_id = $1`, carpoolID); err != nil {
		return fmt.Errorf("clear carpool member settlement: %w", err)
	}
	if err := insertCarpoolEvent(ctx, tx, carpoolID, actorUserID, "unsettled"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit carpool unsettle: %w", err)
	}
	return nil
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

// launchCarpool 在事务内完成发车：创建带周限额安全帽的订阅分组，为每位成员创建
// 写入保底额度（weekly_reserved_usd = reserve×申报）与订阅级周限额（保底 + 公共池 C，
// 个人绝对上限）的订阅，周窗口起点全车对齐（公共池计数器 key 一致的前提），
// 并按发车人数锁定预付。公共池硬约束由组级 Redis 计数器执行（设计文档 §4.2 v3.2）。
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

	// 只有申报 > 0 的成员上车：declared = 0 的车主是"仅发起、不占额度"，
	// 给他建订阅等于白送一份公共池准入（保底 r = 0、无地板、无预付），
	// 正是地板规则要防的搭便车。他会在下面被置为 left，不进结算、不摊席位费。
	rows, err := tx.QueryContext(ctx, `SELECT id, user_id, declared_weekly_quota_usd FROM carpool_members WHERE carpool_id = $1 AND status = 'joined' ORDER BY id`, carpoolID)
	if err != nil {
		return 0, nil, fmt.Errorf("list launching carpool members: %w", err)
	}
	type member struct {
		id, userID int64
		declared   float64
	}
	members := make([]member, 0)
	skipped := make([]int64, 0)
	declaredTotal := 0.0
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.id, &m.userID, &m.declared); err != nil {
			rows.Close()
			return 0, nil, fmt.Errorf("scan launching carpool member: %w", err)
		}
		if m.declared <= 0 {
			skipped = append(skipped, m.id)
			continue
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
	if len(members) == 0 {
		return 0, nil, service.ErrCarpoolLaunchNotReady
	}
	if len(members) > service.CarpoolMaxMembers {
		return 0, nil, service.ErrCarpoolFull
	}
	if len(skipped) > 0 {
		if _, err := tx.ExecContext(ctx, `
UPDATE carpool_members SET status = 'left', left_at = NOW(),
    removal_reason = 'zero declared quota at launch', updated_at = NOW()
WHERE id = ANY($1)`, pq.Array(skipped)); err != nil {
			return 0, nil, fmt.Errorf("drop zero-declaration carpool members: %w", err)
		}
	}

	now := time.Now().UTC()
	expiresAt := now.AddDate(0, 1, 0)
	// 全车周窗口统一对齐到发车日零点（UTC），之后周重置吸附回同一 7 天网格
	// （subscription_service.CheckAndResetWindows），保证全体成员窗口起点恒等，
	// 组级公共池计数器 key（carpool:commons:{group}:{window_start}）因此一致。
	weeklyWindowStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	userIDs := make([]int64, 0, len(members))
	for _, member := range members {
		// 保底 r = reserve×申报（weekly_reserved_usd）：周用量 < r 无条件放行；
		// 订阅级周限额 = r + 公共池 C（设计文档 §4.2），保留为个人绝对上限防呆。
		reservedUSD := service.CarpoolMemberReservedUSD(params.reserveRatio, member.declared)
		weeklyLimitUSD := service.CarpoolMemberWeeklyLimitUSD(params.weeklyLimitUSD, params.reserveRatio, member.declared, declaredTotal)
		var subscriptionID int64
		err := tx.QueryRowContext(ctx, `
INSERT INTO user_subscriptions (
    user_id, group_id, starts_at, expires_at, status, assigned_by, assigned_at, notes,
    weekly_limit_usd, weekly_reserved_usd, weekly_window_start
) VALUES ($1, $2, $3, $4, 'active', $5, $3, $6, $7, $8, $9)
RETURNING id`, member.userID, groupID, now, expiresAt, ownerUserID, "Automatically assigned when carpool launched",
			weeklyLimitUSD, reservedUSD, weeklyWindowStart).Scan(&subscriptionID)
		if err != nil {
			return 0, nil, fmt.Errorf("assign carpool subscription to user %d: %w", member.userID, err)
		}
		// 预付按发车时人数锁定（设计文档 §4.4）。注意只重写 prepaid_amount_cny，
		// 上车时的报价 quoted_prepaid_amount_cny 保持原值——结算时席位费退补
		// 正是拿这两个数之差算出来的。
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
    (c.join_locked_at IS NOT NULL OR c.status = 'confirmed'), c.scheduled_start_at, c.launched_at,
    c.group_id, g.name,
    (SELECT current_member.role FROM carpool_members current_member
     WHERE current_member.carpool_id = c.id AND current_member.user_id = $1
     LIMIT 1),
    c.created_at,
    c.weekly_limit_usd, c.seat_fee_cny, c.usage_pool_cny, c.reserve_ratio,
    c.launch_min_ratio, c.launch_max_ratio,
    (SELECT COALESCE(SUM(declared_member.declared_weekly_quota_usd), 0)
     FROM carpool_members declared_member
     WHERE declared_member.carpool_id = c.id AND declared_member.status IN ('joined', 'active')),
    c.launch_notified_at, c.confirmed_at, (c.group_qr_code IS NOT NULL),
    c.settled_at, c.settled_by_user_id
FROM carpools c
LEFT JOIN users u ON u.id = c.owner_user_id
LEFT JOIN groups g ON g.id = c.group_id AND g.deleted_at IS NULL`

type carpoolScanner interface {
	Scan(dest ...any) error
}

func scanCarpool(scanner carpoolScanner) (*service.Carpool, error) {
	var item service.Carpool
	var ownerUserID, groupID sql.NullInt64
	var capacity, settledByUserID sql.NullInt64
	var groupName, memberRole sql.NullString
	var scheduledStartAt, launchedAt, launchNotifiedAt, confirmedAt, settledAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.Name, &item.Description, &item.Organizer,
		&ownerUserID, &item.Platform, &item.PlanType, &item.CarType, &item.Level,
		&capacity, &item.MemberCount, &item.BaseFeeCNY, &item.UsagePoolPerAccountCNY,
		&item.Visibility, &item.Status, &item.JoinLocked, &scheduledStartAt, &launchedAt,
		&groupID, &groupName, &memberRole, &item.CreatedAt,
		&item.WeeklyLimitUSD, &item.SeatFeeCNY, &item.UsagePoolCNY, &item.ReserveRatio,
		&item.LaunchMinRatio, &item.LaunchMaxRatio, &item.DeclaredTotalUSD,
		&launchNotifiedAt, &confirmedAt, &item.HasGroupQRCode,
		&settledAt, &settledByUserID,
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
	if launchNotifiedAt.Valid {
		item.LaunchNotifiedAt = &launchNotifiedAt.Time
	}
	if confirmedAt.Valid {
		item.ConfirmedAt = &confirmedAt.Time
	}
	if settledAt.Valid {
		item.SettledAt = &settledAt.Time
	}
	if settledByUserID.Valid {
		item.SettledByUserID = &settledByUserID.Int64
	}
	return &item, nil
}

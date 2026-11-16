package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// GetWebSessionAccount 解析本车网页版会话使用的上游账号：优先 carpools.primary_account_id
// → accounts 行；主账号未绑定时回退到本车 group 内 priority 最高且 credentials 里
// 有邮箱的账号（与 GetCprAccountID 的回退口径一致，只是约束从 cpr_account_id 映射
// 换成 email 存在）。车不存在返回 service.ErrCarpoolNotFound；无可用账号返回 (nil, nil)。
// Email 是 credentials->>'email'（账号邮箱，slot id 推导的默认来源）；
// SlotID 是 extra->>'desktop2web_slot_id' 覆盖值（为空表示走默认推导）。
func (r *carpoolRepository) GetWebSessionAccount(ctx context.Context, carpoolID int64) (*service.CarpoolWebSessionAccount, error) {
	item, err := r.getWebSessionAccount(ctx, carpoolID, false)
	if errors.Is(err, sql.ErrNoRows) {
		// 主账号未绑定：回退到组内 priority 最高的有邮箱账号。
		item, err = r.getWebSessionAccount(ctx, carpoolID, true)
	}
	if errors.Is(err, sql.ErrNoRows) {
		// 区分"车不存在"与"车没有可用账号"：再探一次车的存在性。
		var exists bool
		if checkErr := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM carpools WHERE id = $1)`, carpoolID).Scan(&exists); checkErr == nil && !exists {
			return nil, service.ErrCarpoolNotFound
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *carpoolRepository) getWebSessionAccount(ctx context.Context, carpoolID int64, groupFallback bool) (*service.CarpoolWebSessionAccount, error) {
	query := `
SELECT a.id, a.credentials->>'email', a.extra->>'desktop2web_slot_id'
FROM carpools c
JOIN accounts a ON a.id = c.primary_account_id
WHERE c.id = $1`
	if groupFallback {
		query = `
SELECT a.id, a.credentials->>'email', a.extra->>'desktop2web_slot_id'
FROM carpools c
JOIN account_groups ag ON ag.group_id = c.group_id
JOIN accounts a ON a.id = ag.account_id
WHERE c.id = $1 AND c.primary_account_id IS NULL
  AND btrim(COALESCE(a.credentials->>'email', '')) <> ''
ORDER BY ag.priority DESC, a.id ASC
LIMIT 1`
	}
	var item service.CarpoolWebSessionAccount
	var email, slotID sql.NullString
	err := r.db.QueryRowContext(ctx, query, carpoolID).Scan(&item.AccountID, &email, &slotID)
	if err != nil {
		return nil, err
	}
	item.Email = strings.TrimSpace(email.String)
	item.SlotID = strings.TrimSpace(slotID.String)
	return &item, nil
}

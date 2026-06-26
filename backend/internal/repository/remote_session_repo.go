package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// remoteSessionRepository implements service.RemoteSessionRepository using raw SQL
// (the remote_sessions table has no generated ent entity, mirroring the
// account_co_owners raw-SQL approach in account_repo.go).
type remoteSessionRepository struct {
	sql *sql.DB
}

// NewRemoteSessionRepository creates the repository. It takes *sql.DB (the same
// handle threaded to other raw-SQL repos via repository.ProvideSQLDB).
func NewRemoteSessionRepository(sqlDB *sql.DB) service.RemoteSessionRepository {
	return &remoteSessionRepository{sql: sqlDB}
}

func (r *remoteSessionRepository) Create(ctx context.Context, s *service.RemoteSession) error {
	const q = `
		INSERT INTO remote_sessions
			(account_id, surplus_user_id, kasm_id, kasm_user_id, mode, status, created_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, created_at, last_seen_at
	`
	row := r.sql.QueryRowContext(ctx, q,
		s.AccountID, s.SurplusUserID, s.KasmID, s.KasmUserID, s.Mode, s.Status)
	return row.Scan(&s.ID, &s.CreatedAt, &s.LastSeenAt)
}

func (r *remoteSessionRepository) CountLiveByAccount(ctx context.Context, accountID int64) (int, error) {
	const q = `
		SELECT COUNT(*)
		FROM remote_sessions
		WHERE account_id = $1
		  AND status IN ('starting', 'running')
		  AND ended_at IS NULL
	`
	var n int
	if err := r.sql.QueryRowContext(ctx, q, accountID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *remoteSessionRepository) CountLiveGlobal(ctx context.Context) (int, error) {
	const q = `
		SELECT COUNT(*)
		FROM remote_sessions
		WHERE status IN ('starting', 'running')
		  AND ended_at IS NULL
	`
	var n int
	if err := r.sql.QueryRowContext(ctx, q).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *remoteSessionRepository) ListLive(ctx context.Context) ([]service.RemoteSession, error) {
	const q = `
		SELECT id, account_id, surplus_user_id, kasm_id, kasm_user_id, mode, status,
		       created_at, last_seen_at, ended_at
		FROM remote_sessions
		WHERE status IN ('starting', 'running')
		  AND ended_at IS NULL
		ORDER BY id ASC
	`
	rows, err := r.sql.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []service.RemoteSession
	for rows.Next() {
		var s service.RemoteSession
		var endedAt sql.NullTime
		if err := rows.Scan(
			&s.ID, &s.AccountID, &s.SurplusUserID, &s.KasmID, &s.KasmUserID, &s.Mode, &s.Status,
			&s.CreatedAt, &s.LastSeenAt, &endedAt,
		); err != nil {
			return nil, err
		}
		if endedAt.Valid {
			t := endedAt.Time
			s.EndedAt = &t
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *remoteSessionRepository) MarkEndedByKasmID(ctx context.Context, kasmID string) error {
	const q = `
		UPDATE remote_sessions
		SET status = 'ended', ended_at = NOW(), last_seen_at = NOW()
		WHERE kasm_id = $1
		  AND status IN ('starting', 'running')
		  AND ended_at IS NULL
	`
	_, err := r.sql.ExecContext(ctx, q, kasmID)
	return err
}

func (r *remoteSessionRepository) TouchLastSeen(ctx context.Context, id int64) error {
	const q = `
		UPDATE remote_sessions
		SET last_seen_at = NOW()
		WHERE id = $1
		  AND ended_at IS NULL
	`
	_, err := r.sql.ExecContext(ctx, q, id)
	return err
}

func (r *remoteSessionRepository) MarkEndedByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	const q = `
		UPDATE remote_sessions
		SET status = 'ended', ended_at = NOW(), last_seen_at = NOW()
		WHERE id = ANY($1)
		  AND ended_at IS NULL
	`
	_, err := r.sql.ExecContext(ctx, q, pq.Array(ids))
	return err
}

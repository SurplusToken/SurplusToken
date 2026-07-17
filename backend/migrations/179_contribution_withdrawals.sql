CREATE TABLE IF NOT EXISTS contribution_withdrawals (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    amount DECIMAL(20,8) NOT NULL CHECK (amount > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'rejected', 'cancelled')),
    payment_method VARCHAR(20) NOT NULL
        CHECK (payment_method IN ('alipay', 'wechat', 'bank', 'other')),
    payment_account VARCHAR(255) NOT NULL,
    payee_name VARCHAR(100) NOT NULL,
    request_note VARCHAR(500) NOT NULL DEFAULT '',
    review_note VARCHAR(500) NOT NULL DEFAULT '',
    payment_reference VARCHAR(255) NOT NULL DEFAULT '',
    idempotency_key VARCHAR(128) NOT NULL,
    request_fingerprint VARCHAR(64) NOT NULL,
    reviewed_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ NULL,
    paid_at TIMESTAMPTZ NULL,
    cancelled_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT contribution_withdrawals_user_idempotency UNIQUE (user_id, idempotency_key)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_contribution_withdrawals_one_pending_per_user
    ON contribution_withdrawals(user_id)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_contribution_withdrawals_status_requested
    ON contribution_withdrawals(status, requested_at DESC);

CREATE INDEX IF NOT EXISTS idx_contribution_withdrawals_user_requested
    ON contribution_withdrawals(user_id, requested_at DESC);

ALTER TABLE user_contribution_ledger
    ADD COLUMN IF NOT EXISTS withdrawal_id BIGINT NULL
        REFERENCES contribution_withdrawals(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_user_contribution_ledger_withdrawal
    ON user_contribution_ledger(withdrawal_id)
    WHERE withdrawal_id IS NOT NULL;

COMMENT ON TABLE contribution_withdrawals IS '贡献收益人工提现申请';
COMMENT ON COLUMN contribution_withdrawals.payment_account IS '收款账号，仅对申请人和管理员可见';
COMMENT ON COLUMN contribution_withdrawals.payment_reference IS '管理员确认打款时填写的外部支付流水号';

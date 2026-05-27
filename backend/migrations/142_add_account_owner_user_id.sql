ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS owner_user_id BIGINT NULL;

CREATE INDEX IF NOT EXISTS idx_accounts_owner_user_id
    ON accounts(owner_user_id)
    WHERE deleted_at IS NULL;

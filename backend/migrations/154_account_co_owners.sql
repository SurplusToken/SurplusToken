-- account_co_owners: admin-assigned additional owners of a contributed account.
-- The owner set of an account = its primary owner_user_id ∪ co-owners recorded
-- here. Single-owner and multi-owner accounts coexist in the pool. Co-owners
-- bypass contribution protection on accounts they co-own and share contribution
-- rewards evenly with the other owners. No foreign keys (consistent with the
-- ops/audit table design philosophy in this codebase).
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS account_co_owners (
    id          BIGSERIAL    PRIMARY KEY,
    account_id  BIGINT       NOT NULL,
    user_id     BIGINT       NOT NULL,
    created_by  BIGINT,                                   -- admin user who assigned this co-owner (nullable)
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- One row per (account, owner).
CREATE UNIQUE INDEX IF NOT EXISTS accountcoowner_account_id_user_id ON account_co_owners (account_id, user_id);
-- Reverse lookup: which accounts a user co-owns.
CREATE INDEX IF NOT EXISTS accountcoowner_user_id ON account_co_owners (user_id);

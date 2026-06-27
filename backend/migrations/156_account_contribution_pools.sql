-- account_contribution_pools: per-account "held" contribution-reward pool (Model B).
-- Reward earned from NON-owner consumption of a contributed account is held here per
-- account (NOT credited to the owner's spendable contribution balance) until the
-- primary owner manually distributes it to the account's owner set (primary + co-owners).
-- Distribution credits each recipient's user_contributions.contribution_quota (available
-- immediately, no freeze) and decrements this pool, all in one transaction.
-- No foreign keys (consistent with this codebase's ops-table design philosophy).

CREATE TABLE IF NOT EXISTS account_contribution_pools (
    account_id  BIGINT         PRIMARY KEY,
    pool_amount NUMERIC(20,10) NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Mark contributor "self-use" groups so they can be hidden from admin group
-- lists, the subscription store, and other users' group pickers. Such a group
-- is auto-created (one per contributing user), holds only that user's own
-- contributed accounts, and is configured as subscription-type + unlimited so
-- the owner can use their own accounts free of platform-balance billing.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS auto_self_use boolean NOT NULL DEFAULT false;

-- Partial index: the common lookup/hiding filter only ever selects the handful
-- of self-use groups, never the false majority.
CREATE INDEX IF NOT EXISTS idx_groups_auto_self_use
    ON groups (auto_self_use)
    WHERE auto_self_use = true;

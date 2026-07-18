-- Real multi-user carpool runtime. A full carpool atomically creates one
-- unlimited OpenAI subscription group and one monthly subscription per member.
CREATE TABLE IF NOT EXISTS carpools (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    platform VARCHAR(50) NOT NULL DEFAULT 'openai',
    plan_type VARCHAR(50) NOT NULL DEFAULT 'openai_pro',
    car_type VARCHAR(20) NOT NULL DEFAULT 'small',
    level INTEGER NOT NULL DEFAULT 1,
    capacity INTEGER NOT NULL,
    base_fee_cny DECIMAL(20, 8) NOT NULL DEFAULT 130,
    usage_pool_cny_per_account DECIMAL(20, 8) NOT NULL DEFAULT 750,
    visibility VARCHAR(20) NOT NULL DEFAULT 'public',
    status VARCHAR(20) NOT NULL DEFAULT 'recruiting',
    group_id BIGINT NULL REFERENCES groups(id) ON DELETE SET NULL,
    primary_account_id BIGINT NULL REFERENCES accounts(id) ON DELETE SET NULL,
    scheduled_start_at TIMESTAMPTZ NULL,
    join_locked_at TIMESTAMPTZ NULL,
    join_locked_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    launched_at TIMESTAMPTZ NULL,
    cancelled_at TIMESTAMPTZ NULL,
    cancelled_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    cancel_reason TEXT NULL,
    ended_at TIMESTAMPTZ NULL,
    version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT carpools_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT carpools_capacity_positive CHECK (capacity > 0),
    CONSTRAINT carpools_version_nonnegative CHECK (version >= 0),
    CONSTRAINT carpools_visibility_check CHECK (visibility IN ('public', 'invite_only')),
    CONSTRAINT carpools_status_check CHECK (status IN ('recruiting', 'starting', 'active', 'cancelled', 'ended'))
);

-- Staging briefly carried an earlier entity-only version of these tables.
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS car_type VARCHAR(20) NOT NULL DEFAULT 'small';
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS level INTEGER NOT NULL DEFAULT 1;
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS base_fee_cny DECIMAL(20, 8) NOT NULL DEFAULT 130;
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS usage_pool_cny_per_account DECIMAL(20, 8) NOT NULL DEFAULT 750;

CREATE INDEX IF NOT EXISTS idx_carpools_owner_created ON carpools(owner_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_carpools_status_created ON carpools(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_carpools_visibility_status ON carpools(visibility, status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_carpools_group_unique ON carpools(group_id) WHERE group_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_carpools_name_live_unique
    ON carpools(name) WHERE status IN ('recruiting', 'starting', 'active');

CREATE TABLE IF NOT EXISTS carpool_invites (
    id BIGSERIAL PRIMARY KEY,
    carpool_id BIGINT NOT NULL REFERENCES carpools(id) ON DELETE CASCADE,
    created_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    token_hash VARCHAR(64) NOT NULL,
    token_hint VARCHAR(12) NOT NULL DEFAULT '',
    max_uses INTEGER NOT NULL DEFAULT 0,
    use_count INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NULL,
    revoked_at TIMESTAMPTZ NULL,
    revoked_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT carpool_invites_usage_nonnegative CHECK (max_uses >= 0 AND use_count >= 0),
    CONSTRAINT carpool_invites_usage_within_limit CHECK (max_uses = 0 OR use_count <= max_uses)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_carpool_invites_token_hash ON carpool_invites(token_hash);
CREATE INDEX IF NOT EXISTS idx_carpool_invites_carpool_created ON carpool_invites(carpool_id, created_at DESC);

CREATE TABLE IF NOT EXISTS carpool_members (
    id BIGSERIAL PRIMARY KEY,
    carpool_id BIGINT NOT NULL REFERENCES carpools(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    status VARCHAR(20) NOT NULL DEFAULT 'joined',
    subscription_id BIGINT NULL REFERENCES user_subscriptions(id) ON DELETE SET NULL,
    joined_via_invite_id BIGINT NULL REFERENCES carpool_invites(id) ON DELETE SET NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    activated_at TIMESTAMPTZ NULL,
    left_at TIMESTAMPTZ NULL,
    removed_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    removal_reason TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_carpool_members_carpool_user ON carpool_members(carpool_id, user_id);
CREATE INDEX IF NOT EXISTS idx_carpool_members_carpool_status ON carpool_members(carpool_id, status);
CREATE INDEX IF NOT EXISTS idx_carpool_members_user_status ON carpool_members(user_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_carpool_members_subscription_unique ON carpool_members(subscription_id) WHERE subscription_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS carpool_events (
    id BIGSERIAL PRIMARY KEY,
    carpool_id BIGINT NOT NULL REFERENCES carpools(id) ON DELETE CASCADE,
    actor_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(64) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_carpool_events_carpool_created ON carpool_events(carpool_id, created_at DESC);

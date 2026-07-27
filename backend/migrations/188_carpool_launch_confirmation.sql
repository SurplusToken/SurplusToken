-- Carpool launch confirmation (两段确认发车) + onboarding requirements:
-- members must confirm joining the WeChat group when boarding, carpool
-- creation requires the admin-WeChat confirmation plus a stored group QR
-- code, and launch becomes a two-stage flow: owner confirms while the total
-- declared quota sits inside the launch band (recruiting -> confirmed), then
-- an admin launches (confirmed -> active). A force launch (recruiting, >=80%
-- declared) remains available to admins for under-subscribed cars.

-- carpools: group QR code stored in-DB, plus the "added admin WeChat" flag.
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS group_qr_code BYTEA NULL;
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS group_qr_code_content_type VARCHAR(50) NULL;
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS added_admin_wechat BOOLEAN NOT NULL DEFAULT false;

-- carpools: two-stage launch bookkeeping. launch_notified_at records when the
-- car first entered the launch band (owner notified by email); it resets to
-- NULL when a member leaving drops the total back out of the band.
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS launch_notified_at TIMESTAMPTZ NULL;
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS confirmed_at TIMESTAMPTZ NULL;
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS confirmed_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL;

-- status machine: recruiting -> confirmed -> active (starting/cancelled/ended
-- unchanged). confirmed cars are locked for joins and can only be cancelled
-- by admins.
ALTER TABLE carpools DROP CONSTRAINT IF EXISTS carpools_status_check;
ALTER TABLE carpools ADD CONSTRAINT carpools_status_check
    CHECK (status IN ('recruiting', 'confirmed', 'starting', 'active', 'cancelled', 'ended'));

-- confirmed cars still reserve their name (they have no group yet, so the
-- launch-time group insert must not collide with a freshly created carpool).
DROP INDEX IF EXISTS idx_carpools_name_live_unique;
CREATE UNIQUE INDEX IF NOT EXISTS idx_carpools_name_live_unique
    ON carpools(name) WHERE status IN ('recruiting', 'confirmed', 'starting', 'active');

-- carpool_members: "joined the WeChat group" confirmation captured at join
-- time. DEFAULT false only keeps pre-existing rows valid; new joins always
-- write an explicit true.
ALTER TABLE carpool_members ADD COLUMN IF NOT EXISTS joined_wechat_group BOOLEAN NOT NULL DEFAULT false;

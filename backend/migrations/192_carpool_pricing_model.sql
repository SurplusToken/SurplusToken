-- Carpool pricing model discriminator (自定义规则车).
--
-- migration 187 replaced the seat-based model (人头席位制: capacity seats at
-- base_fee_cny each, auto-launch when full) with the quota reservation model
-- (额度预约制: declare a weekly USD quota, ¥400 seat fee + ¥1000 usage pool).
-- It gave every pre-existing row the new model's defaults, and crucially set
-- carpool_members.declared_weekly_quota_usd = 0 for every existing member.
--
-- That silently misrepresents every carpool that predates the switch:
--   * the card renders a 0 / 2400 quota bar and a ¥0 average price;
--   * the settlement statement bills them on the ¥400 + ¥1000 model they never
--     agreed to -- with declared = 0 the floor is 0, the usage prepay is 0, and
--     every member shows a several-hundred-yuan top-up out of nowhere.
--
-- So pre-existing carpools are moved into a 'custom' bucket: no automatic
-- settlement math, no quota-bar UI, and a human-readable note stating the rule
-- they actually run under. The note is generated from each car's own legacy
-- columns (capacity / base_fee_cny / usage_pool_cny_per_account), which are all
-- still present -- that is truthful per car, unlike a single frozen sentence.
--
--   quota  -> the reservation model; the default for everything created from now on
--   custom -> settled manually against rule_note (legacy cars, and any car an
--             administrator sets up under negotiated terms)
--
-- NOTE: "every row that exists right now is legacy" is only correct on a
-- database that has not yet run the quota-model code. That holds for a
-- deployment upgrading from before the switch; it does NOT hold for a dev
-- database that already created quota-model carpools.
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS pricing_model VARCHAR(16);
ALTER TABLE carpools ADD COLUMN IF NOT EXISTS rule_note TEXT;

UPDATE carpools SET pricing_model = 'custom' WHERE pricing_model IS NULL;

UPDATE carpools
SET rule_note = concat(
        '旧版席位规则（平台升级前建立）：共 ',
        COALESCE(capacity::text, '若干'),
        ' 席，基础费 ¥',
        to_char(base_fee_cny, 'FM999999990.99'),
        '/席，用量池 ¥',
        to_char(usage_pool_cny_per_account, 'FM999999990.99'),
        '/账号，按实际用量比例分摊。本车不适用额度预约制的保底与自动退补，由车主按上述规则人工结算。')
WHERE pricing_model = 'custom' AND rule_note IS NULL;

ALTER TABLE carpools ALTER COLUMN pricing_model SET DEFAULT 'quota';
ALTER TABLE carpools ALTER COLUMN pricing_model SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_carpools_pricing_model
    ON carpools (pricing_model)
    WHERE pricing_model <> 'quota';

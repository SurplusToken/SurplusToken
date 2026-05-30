-- Backfill contribution reward ledger rows for usage logs that were recorded
-- before contributor rewards were written transactionally.
WITH config AS (
    SELECT
        LEAST(100::numeric, GREATEST(0::numeric, COALESCE((
            SELECT CASE
                WHEN BTRIM(value) ~ '^-?([0-9]+(\.[0-9]*)?|\.[0-9]+)$' THEN BTRIM(value)::numeric
                ELSE NULL
            END
            FROM settings
            WHERE key = 'contribution_reward_rate'
            LIMIT 1
        ), 80::numeric))) AS reward_rate,
        LEAST(720, GREATEST(0, COALESCE((
            SELECT CASE
                WHEN BTRIM(value) ~ '^-?[0-9]+$' THEN BTRIM(value)::integer
                ELSE NULL
            END
            FROM settings
            WHERE key = 'contribution_reward_freeze_hours'
            LIMIT 1
        ), 0))) AS freeze_hours
),
eligible AS (
    SELECT
        a.owner_user_id AS user_id,
        ROUND((
            COALESCE(ul.account_stats_cost, ul.total_cost, 0)
            * COALESCE(ul.account_rate_multiplier, 1)
            * (config.reward_rate / 100)
        )::numeric, 8) AS amount,
        COALESCE(NULLIF(ul.request_id, ''), 'usage-log:' || ul.id::text) AS usage_billing_request_id,
        ul.api_key_id,
        ul.account_id,
        ul.user_id AS consumer_user_id,
        ul.model,
        COALESCE(ul.input_tokens, 0) AS input_tokens,
        COALESCE(ul.output_tokens, 0) AS output_tokens,
        COALESCE(ul.cache_creation_tokens, 0) AS cache_creation_tokens,
        COALESCE(ul.cache_read_tokens, 0) AS cache_read_tokens,
        COALESCE(ul.image_count, 0) AS image_count,
        COALESCE(ul.total_cost, 0) AS total_cost,
        ul.account_stats_cost,
        COALESCE(ul.account_rate_multiplier, 1) AS account_rate_multiplier,
        config.reward_rate,
        CASE
            WHEN config.freeze_hours > 0
                 AND ul.created_at + make_interval(hours => config.freeze_hours) > NOW()
            THEN ul.created_at + make_interval(hours => config.freeze_hours)
            ELSE NULL
        END AS frozen_until,
        ul.created_at
    FROM usage_logs ul
    JOIN accounts a ON a.id = ul.account_id
    CROSS JOIN config
    WHERE a.owner_user_id IS NOT NULL
      AND a.owner_user_id > 0
      AND a.owner_user_id <> ul.user_id
      AND COALESCE(ul.actual_cost, 0) > 0
      AND ROUND((
          COALESCE(ul.account_stats_cost, ul.total_cost, 0)
          * COALESCE(ul.account_rate_multiplier, 1)
          * (config.reward_rate / 100)
      )::numeric, 8) > 0
      AND NOT EXISTS (
          SELECT 1
          FROM user_contribution_ledger existing
          WHERE existing.action = 'accrue'
            AND existing.user_id = a.owner_user_id
            AND existing.usage_billing_request_id = COALESCE(NULLIF(ul.request_id, ''), 'usage-log:' || ul.id::text)
            AND existing.api_key_id IS NOT DISTINCT FROM ul.api_key_id
      )
),
ensure_summaries AS (
    INSERT INTO user_contributions (user_id, created_at, updated_at)
    SELECT DISTINCT user_id, NOW(), NOW()
    FROM eligible
    ON CONFLICT (user_id) DO NOTHING
),
inserted AS (
    INSERT INTO user_contribution_ledger (
        user_id, action, amount, usage_billing_request_id, api_key_id, account_id, consumer_user_id,
        model, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, image_count,
        total_cost, account_stats_cost, account_rate_multiplier, reward_rate, frozen_until,
        created_at, updated_at
    )
    SELECT
        user_id, 'accrue', amount, usage_billing_request_id, api_key_id, account_id, consumer_user_id,
        model, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, image_count,
        total_cost, account_stats_cost, account_rate_multiplier, reward_rate, frozen_until,
        created_at, NOW()
    FROM eligible
    ON CONFLICT DO NOTHING
    RETURNING user_id, amount, frozen_until
),
reward_totals AS (
    SELECT
        user_id,
        SUM(amount) AS total_amount,
        SUM(CASE WHEN frozen_until IS NULL THEN amount ELSE 0 END) AS available_amount,
        SUM(CASE WHEN frozen_until IS NULL THEN 0 ELSE amount END) AS frozen_amount
    FROM inserted
    GROUP BY user_id
)
UPDATE user_contributions uc
SET contribution_quota = uc.contribution_quota + reward_totals.available_amount,
    contribution_frozen_quota = uc.contribution_frozen_quota + reward_totals.frozen_amount,
    contribution_history_quota = uc.contribution_history_quota + reward_totals.total_amount,
    updated_at = NOW()
FROM reward_totals
WHERE uc.user_id = reward_totals.user_id;

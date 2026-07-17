-- Promote legacy OpenAI-compatible Kimi accounts to a first-class platform.
ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi'));

UPDATE accounts
SET platform = 'kimi', updated_at = NOW()
WHERE platform = 'openai'
  AND lower(COALESCE(extra->>'openai_compatible_provider', '')) = 'kimi';

-- A group can change platform only when every live member is a Kimi account.
WITH kimi_groups AS (
    SELECT ag.group_id
    FROM account_groups ag
    JOIN accounts a ON a.id = ag.account_id AND a.deleted_at IS NULL
    GROUP BY ag.group_id
    HAVING bool_or(a.platform = 'kimi')
       AND bool_and(a.platform = 'kimi')
)
UPDATE groups g
SET platform = 'kimi',
    allow_messages_dispatch = TRUE,
    messages_dispatch_model_config = jsonb_build_object(
        'opus_mapped_model', 'kimi-for-coding',
        'sonnet_mapped_model', 'kimi-for-coding',
        'haiku_mapped_model', 'kimi-for-coding'
    ),
    models_list_config = jsonb_build_object(
        'enabled', FALSE,
        'models', jsonb_build_array('kimi-for-coding', 'kimi-for-coding-highspeed')
    ),
    updated_at = NOW()
FROM kimi_groups kg
WHERE g.id = kg.group_id
  AND g.deleted_at IS NULL;

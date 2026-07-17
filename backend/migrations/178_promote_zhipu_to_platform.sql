-- Promote legacy OpenAI-compatible Zhipu accounts to a first-class platform.
ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu'));

UPDATE accounts
SET platform = 'zhipu', updated_at = NOW()
WHERE platform = 'openai'
  AND lower(COALESCE(extra->>'openai_compatible_provider', '')) = 'zhipu';

-- A group can change platform only when every live member is a Zhipu account.
WITH zhipu_groups AS (
    SELECT ag.group_id
    FROM account_groups ag
    JOIN accounts a ON a.id = ag.account_id AND a.deleted_at IS NULL
    GROUP BY ag.group_id
    HAVING bool_or(a.platform = 'zhipu')
       AND bool_and(a.platform = 'zhipu')
)
UPDATE groups g
SET platform = 'zhipu',
    allow_messages_dispatch = TRUE,
    messages_dispatch_model_config = jsonb_build_object(
        'opus_mapped_model', 'glm-5.2',
        'sonnet_mapped_model', 'glm-5.2',
        'haiku_mapped_model', 'glm-4.7'
    ),
    models_list_config = jsonb_build_object(
        'enabled', FALSE,
        'models', jsonb_build_array('glm-5.2', 'glm-5-turbo', 'glm-4.7')
    ),
    updated_at = NOW()
FROM zhipu_groups zg
WHERE g.id = zg.group_id
  AND g.deleted_at IS NULL;

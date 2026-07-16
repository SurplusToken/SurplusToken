ALTER TABLE chat_conversations
    ADD COLUMN IF NOT EXISTS reasoning_effort VARCHAR(20) NOT NULL DEFAULT '';

ALTER TABLE chat_conversations
    DROP CONSTRAINT IF EXISTS chat_conversations_reasoning_effort_check;

ALTER TABLE chat_conversations
    ADD CONSTRAINT chat_conversations_reasoning_effort_check
    CHECK (reasoning_effort IN ('', 'none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'));

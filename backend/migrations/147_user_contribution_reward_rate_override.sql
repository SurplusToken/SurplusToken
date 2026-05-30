ALTER TABLE user_contributions
    ADD COLUMN IF NOT EXISTS contribution_reward_rate DECIMAL(10,4) NULL;

COMMENT ON COLUMN user_contributions.contribution_reward_rate IS '用户贡献账号收益比例覆盖，NULL 表示跟随系统默认';

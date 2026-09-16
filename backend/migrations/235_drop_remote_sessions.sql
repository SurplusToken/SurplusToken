-- 远程连接从 Kasm 迁移到 Desktop2Web 后，会话生命周期完全由网关持有：
-- ticket 一次性兑换、Web Session 与 runtime 由 d2w 自行回收。本地不再落任何行，
-- 因此 remote_sessions 表没有消费者。
DROP TABLE IF EXISTS remote_sessions;

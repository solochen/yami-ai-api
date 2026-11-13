-- 205_upstream_account_login：记录 API Key 对应的上游登录账号。
-- 用途：同一上游可用多个登录账号各建 Key 来分流，避免单账号高并发触发风控。
ALTER TABLE upstream_accounts ADD COLUMN IF NOT EXISTS login_account varchar(190) NOT NULL DEFAULT '';

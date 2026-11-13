-- 208_upstream_account_health：上游账号级健康状态（主动探测 + 熔断）。
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS health_status varchar(16) NOT NULL DEFAULT 'healthy',
  ADD COLUMN IF NOT EXISTS consecutive_failures int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cooldown_until timestamptz,
  ADD COLUMN IF NOT EXISTS last_probe_at timestamptz;

ALTER TABLE upstream_accounts
  DROP COLUMN IF EXISTS health_status,
  DROP COLUMN IF EXISTS consecutive_failures,
  DROP COLUMN IF EXISTS cooldown_until,
  DROP COLUMN IF EXISTS last_probe_at;

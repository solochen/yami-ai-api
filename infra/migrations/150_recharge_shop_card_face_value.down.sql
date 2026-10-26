DELETE FROM system_configs current
USING recharge_v2_config_backup backup
WHERE current.key = backup.key AND NOT backup.existed;

INSERT INTO system_configs (key, value, updated_at)
SELECT key, value, now()
FROM recharge_v2_config_backup
WHERE existed
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_at = now();

DROP TABLE recharge_v2_config_backup;

-- The one-time card and balance reset performed by the up migration is
-- intentionally irreversible. Rolling the schema back must not recreate card
-- secrets or balances from incomplete historical data.

ALTER TABLE recharge_cards
  DROP CONSTRAINT IF EXISTS recharge_cards_compute_rate_positive,
  DROP CONSTRAINT IF EXISTS recharge_cards_compute_credits_positive,
  DROP CONSTRAINT IF EXISTS recharge_cards_face_value_positive,
  DROP COLUMN IF EXISTS compute_rate,
  DROP COLUMN IF EXISTS compute_credits,
  DROP COLUMN IF EXISTS face_value_cny;

ALTER TABLE recharge_card_batches
  DROP CONSTRAINT IF EXISTS recharge_card_batches_compute_rate_positive,
  DROP CONSTRAINT IF EXISTS recharge_card_batches_compute_credits_positive,
  DROP CONSTRAINT IF EXISTS recharge_card_batches_face_value_positive,
  DROP COLUMN IF EXISTS compute_rate,
  DROP COLUMN IF EXISTS compute_credits,
  DROP COLUMN IF EXISTS face_value_cny;

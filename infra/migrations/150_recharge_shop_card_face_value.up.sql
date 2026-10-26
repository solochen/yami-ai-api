-- Recharge shop and RMB-denominated card redemption.
-- New cards store both the RMB face value and the exact compute amount
-- promised when the batch is generated. The reset at the end of this
-- migration removes all cards created under the previous conversion rule.

ALTER TABLE recharge_card_batches
  ADD COLUMN IF NOT EXISTS face_value_cny NUMERIC(18,2),
  ADD COLUMN IF NOT EXISTS compute_credits NUMERIC(18,6),
  ADD COLUMN IF NOT EXISTS compute_rate NUMERIC(18,6);

ALTER TABLE recharge_cards
  ADD COLUMN IF NOT EXISTS face_value_cny NUMERIC(18,2),
  ADD COLUMN IF NOT EXISTS compute_credits NUMERIC(18,6),
  ADD COLUMN IF NOT EXISTS compute_rate NUMERIC(18,6);

DROP TABLE IF EXISTS recharge_v2_config_backup;
CREATE TABLE recharge_v2_config_backup (
  key TEXT PRIMARY KEY,
  existed BOOLEAN NOT NULL,
  value JSONB
);

INSERT INTO recharge_v2_config_backup (key, existed, value)
SELECT wanted.key, (cfg.key IS NOT NULL), cfg.value
FROM (VALUES
  ('payment_compute_rate'),
  ('payment_currency'),
  ('model_usd_to_compute_rate'),
  ('recharge_shop_enabled'),
  ('recharge_shop_url')
) AS wanted(key)
LEFT JOIN system_configs cfg ON cfg.key = wanted.key;

ALTER TABLE recharge_card_batches
  ADD CONSTRAINT recharge_card_batches_face_value_positive
    CHECK (face_value_cny IS NULL OR face_value_cny > 0) NOT VALID,
  ADD CONSTRAINT recharge_card_batches_compute_credits_positive
    CHECK (compute_credits IS NULL OR compute_credits > 0) NOT VALID,
  ADD CONSTRAINT recharge_card_batches_compute_rate_positive
    CHECK (compute_rate IS NULL OR compute_rate > 0) NOT VALID;

ALTER TABLE recharge_cards
  ADD CONSTRAINT recharge_cards_face_value_positive
    CHECK (face_value_cny IS NULL OR face_value_cny > 0) NOT VALID,
  ADD CONSTRAINT recharge_cards_compute_credits_positive
    CHECK (compute_credits IS NULL OR compute_credits > 0) NOT VALID,
  ADD CONSTRAINT recharge_cards_compute_rate_positive
    CHECK (compute_rate IS NULL OR compute_rate > 0) NOT VALID;

INSERT INTO system_configs (key, value, updated_at)
SELECT 'model_usd_to_compute_rate',
       COALESCE((
         SELECT CASE
           WHEN jsonb_typeof(value) = 'number' AND (value #>> '{}')::numeric > 0 THEN value
           ELSE NULL
         END
         FROM recharge_v2_config_backup
         WHERE key = 'payment_compute_rate' AND existed
       ), '7.2'::jsonb),
       now()
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, updated_at)
VALUES
  ('payment_compute_rate', '1000'::jsonb, now()),
  ('payment_currency', '"CNY"'::jsonb, now()),
  ('recharge_shop_enabled', 'true'::jsonb, now()),
  ('recharge_shop_url', '"https://catfk.com/shop/yardai"'::jsonb, now())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_at = now();

-- The product has not been promoted yet, so this rollout intentionally starts
-- card inventory and compute balances from a clean baseline. Preserve the
-- wallet audit invariant by recording a final zero-balance transaction for
-- every non-zero wallet. Historical recharge transactions are reclassified so
-- the referral reconciler cannot award an old recharge after the reset.
UPDATE balance_freezes
SET status = 'released', released_at = COALESCE(released_at, now())
WHERE status = 'frozen';

UPDATE wallet_transactions
SET type = 'legacy_recharge_reset',
    remark = CASE
      WHEN COALESCE(remark, '') = '' THEN '充值倍率升级前的历史充值记录'
      ELSE remark || '（充值倍率升级前历史记录）'
    END
WHERE direction = 'in'
  AND type IN ('card_recharge', 'online_recharge');

INSERT INTO wallet_transactions
  (user_id, type, direction, amount, balance_after, ref_type, ref_id, remark)
SELECT user_id,
       'balance_reset',
       CASE WHEN compute_balance > 0 THEN 'out' ELSE 'in' END,
       ABS(compute_balance),
       0,
       'migration',
       'recharge-v2',
       '充值倍率升级，历史算力清零'
FROM wallets
WHERE compute_balance <> 0;

UPDATE wallets
SET compute_balance = 0, frozen_compute = 0, updated_at = now()
WHERE compute_balance <> 0 OR frozen_compute <> 0;

DELETE FROM referral_rewards WHERE reward_account = 'compute';
DELETE FROM recharge_cards;
DELETE FROM recharge_card_batches;

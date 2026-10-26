-- Rescale the whole platform from "1 CNY = 1000 credits" to "1 CNY = 1 credit".
--
-- Migration 152 normalized every user-facing price into the 1000-credit unit
-- (CNY x1000, USD x7200). This migration divides every credit-denominated
-- amount by 1000 so one credit equals one CNY cent-for-cent:
--   payment_compute_rate    1000 -> 1
--   model_usd_to_compute_rate 7200 -> 7.2  (USD x exchange rate x 1)
-- Raw source_* fields (source_unit_cost, source_input_price_per_m, ...) stay
-- untouched in their original currency.
--
-- Wallet balances, wallet transactions and balance freezes are divided as
-- well so the ledger stays coherent with the new unit. Cash balances are real
-- CNY and are NOT touched.

-- ========= backups (kept for 154 down) =========
DROP TABLE IF EXISTS pricing_scale_migration_backup_154;
CREATE TABLE pricing_scale_migration_backup_154 AS
SELECT 'config'::text AS kind, key AS ref, value AS payload FROM system_configs
WHERE key IN ('payment_compute_rate', 'model_usd_to_compute_rate',
              'vidu_video_price_per_minute', 'vidu_voice_price_per_minute')
UNION ALL
SELECT 'model', m.id::text, m.price_rule FROM models m WHERE m.price_rule <> '{}'::jsonb
UNION ALL
SELECT 'route', r.id::text, r.cost_rule FROM model_routes r WHERE r.cost_rule <> '{}'::jsonb
UNION ALL
SELECT 'workflow', w.id::text, w.price_rule FROM workflow_definitions w WHERE w.price_rule <> '{}'::jsonb
UNION ALL
SELECT 'wallet', w.user_id::text, to_jsonb(w.compute_balance) FROM wallets w WHERE w.compute_balance <> 0
UNION ALL
SELECT 'wallet_tx', t.id::text, to_jsonb(t.amount) FROM wallet_transactions t WHERE t.amount <> 0
UNION ALL
SELECT 'freeze', f.id::text, to_jsonb(f.amount) FROM balance_freezes f WHERE f.amount <> 0;

-- ========= system configs =========
UPDATE system_configs
SET value = '1'::jsonb, updated_at = now()
WHERE key = 'payment_compute_rate';

UPDATE system_configs
SET value = '7.2'::jsonb, updated_at = now()
WHERE key = 'model_usd_to_compute_rate';

UPDATE system_configs
SET value = to_jsonb((value #>> '{}')::numeric / 1000), updated_at = now()
WHERE key IN ('vidu_video_price_per_minute', 'vidu_voice_price_per_minute')
  AND COALESCE(value #>> '{}', '') ~ '^-?[0-9]+([.][0-9]+)?$';

-- ========= models.price_rule =========
UPDATE models m
SET price_rule = (
      SELECT COALESCE(jsonb_object_agg(key, value), '{}'::jsonb)
      FROM (
        SELECT key,
               CASE
                 WHEN key = ANY(ARRAY[
                   'unit_price', 'input_price', 'output_price',
                   'cache_read_price', 'cache_write_price',
                   'audio_input_price', 'audio_output_price',
                   'input_price_per_m', 'output_price_per_m',
                   'cache_read_price_per_m', 'cache_write_price_per_m',
                   'audio_input_price_per_m', 'audio_output_price_per_m',
                   'surcharge_per_m', 'fallback_cost'
                 ]) AND COALESCE(value #>> '{}', '') ~ '^-?[0-9]+([.][0-9]+)?$'
                      THEN to_jsonb((value #>> '{}')::numeric / 1000)
                 WHEN key = ANY(ARRAY['unit_price_by_size', 'unit_price_by_resolution', 'unit_price_by_variant'])
                      THEN (
                        SELECT COALESCE(jsonb_object_agg(nested_key, to_jsonb(nested_value::numeric / 1000)), '{}'::jsonb)
                        FROM jsonb_each_text(value) AS nested(nested_key, nested_value)
                        WHERE nested_value ~ '^-?[0-9]+([.][0-9]+)?$'
                      )
                 WHEN key = 'usd_to_compute' AND COALESCE(value #>> '{}', '') ~ '^-?[0-9]+([.][0-9]+)?$'
                      THEN to_jsonb((value #>> '{}')::numeric / 1000)
                 WHEN key = 'compute_per_cny'
                      THEN '1'::jsonb
                 WHEN key = 'points_per_cny'
                      THEN '1'::jsonb
                 ELSE value
               END AS value
        FROM jsonb_each(m.price_rule) current(key, value)
      ) rescaled
    ),
    updated_at = now()
WHERE m.price_rule <> '{}'::jsonb;

-- ========= model_routes.cost_rule =========
UPDATE model_routes r
SET cost_rule = (
      SELECT COALESCE(jsonb_object_agg(key, value), '{}'::jsonb)
      FROM (
        SELECT key,
               CASE
                 WHEN key = ANY(ARRAY[
                   'unit_cost', 'input_cost_per_m', 'output_cost_per_m',
                   'cache_read_cost_per_m', 'cache_write_cost_per_m',
                   'audio_input_cost_per_m', 'audio_output_cost_per_m',
                   'surcharge_per_m', 'fallback_cost'
                 ]) AND COALESCE(value #>> '{}', '') ~ '^-?[0-9]+([.][0-9]+)?$'
                      THEN to_jsonb((value #>> '{}')::numeric / 1000)
                 WHEN key = ANY(ARRAY['unit_cost_by_size', 'unit_cost_by_resolution', 'unit_cost_by_variant'])
                      THEN (
                        SELECT COALESCE(jsonb_object_agg(nested_key, to_jsonb(nested_value::numeric / 1000)), '{}'::jsonb)
                        FROM jsonb_each_text(value) AS nested(nested_key, nested_value)
                        WHERE nested_value ~ '^-?[0-9]+([.][0-9]+)?$'
                      )
                 WHEN key = 'usd_to_compute' AND COALESCE(value #>> '{}', '') ~ '^-?[0-9]+([.][0-9]+)?$'
                      THEN to_jsonb((value #>> '{}')::numeric / 1000)
                 WHEN key = 'compute_per_cny'
                      THEN '1'::jsonb
                 WHEN key = 'points_per_cny'
                      THEN '1'::jsonb
                 ELSE value
               END AS value
        FROM jsonb_each(r.cost_rule) current(key, value)
      ) rescaled
    ),
    updated_at = now()
WHERE r.cost_rule <> '{}'::jsonb;

-- ========= workflow_definitions.price_rule =========
UPDATE workflow_definitions w
SET price_rule = (
      SELECT COALESCE(jsonb_object_agg(key, value), '{}'::jsonb)
      FROM (
        SELECT key,
               CASE
                 WHEN key = 'unit_price' AND COALESCE(value #>> '{}', '') ~ '^-?[0-9]+([.][0-9]+)?$'
                      THEN to_jsonb((value #>> '{}')::numeric / 1000)
                 WHEN key = 'points_per_cny'
                      THEN '1'::jsonb
                 ELSE value
               END AS value
        FROM jsonb_each(w.price_rule) current(key, value)
      ) rescaled
    ),
    updated_at = now()
WHERE w.price_rule <> '{}'::jsonb;

-- ========= wallet ledger (compute only, cash untouched) =========
UPDATE wallets
SET compute_balance = compute_balance / 1000,
    frozen_compute = frozen_compute / 1000
WHERE compute_balance <> 0 OR frozen_compute <> 0;

-- Sub-micro dust amounts would round to zero at the column's 6-decimal
-- scale; keep those historic rows untouched instead of violating amount > 0.
UPDATE wallet_transactions
SET amount = amount / 1000,
    balance_after = balance_after / 1000
WHERE amount <> 0
  AND ROUND(amount / 1000, 6) <> 0;

UPDATE balance_freezes
SET amount = amount / 1000
WHERE amount <> 0
  AND ROUND(amount / 1000, 6) <> 0;

-- Leftover recharge cards keep their CNY face value; the credit amounts move
-- to the new unit.
UPDATE recharge_cards
SET value = value / 1000
WHERE value <> 0
  AND ROUND(value / 1000, 6) <> 0;

UPDATE recharge_cards
SET compute_credits = compute_credits / 1000
WHERE compute_credits IS NOT NULL AND compute_credits <> 0;

UPDATE recharge_card_batches
SET value = value / 1000
WHERE value <> 0;

UPDATE recharge_card_batches
SET compute_credits = compute_credits / 1000
WHERE compute_credits IS NOT NULL AND compute_credits <> 0;

-- Normalize every user-facing model and workflow price to compute credits.
--
-- Historically price_rule mixed CNY, USD, legacy compute-like integers, and
-- unitless demo values. The billing path treated every stored amount as the
-- user's compute price, so CNY 1.67 deducted 1.67 compute instead of 1,670.
--
-- OpenRouter and BigModel rows were created at earlier USD/CNY conversion
-- rates. Their raw source fields are authoritative and are used to rebuild the
-- credit values rather than multiplying stale converted values again.

DROP TABLE IF EXISTS model_price_credit_migration_backup_152;
CREATE TABLE model_price_credit_migration_backup_152 (
  model_id BIGINT PRIMARY KEY,
  price_rule JSONB NOT NULL,
  route_costs JSONB NOT NULL,
  backed_up_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO model_price_credit_migration_backup_152 (model_id, price_rule, route_costs)
SELECT m.id, m.price_rule,
       COALESCE(
         jsonb_object_agg(r.id::text, r.cost_rule)
           FILTER (WHERE r.id IS NOT NULL),
         '{}'::jsonb
       )
FROM models m
LEFT JOIN model_routes r ON r.model_id = m.id
GROUP BY m.id, m.price_rule;

DROP TABLE IF EXISTS workflow_price_credit_migration_backup_152;
CREATE TABLE workflow_price_credit_migration_backup_152 (
  workflow_id BIGINT PRIMARY KEY,
  price_rule JSONB NOT NULL,
  backed_up_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO workflow_price_credit_migration_backup_152 (workflow_id, price_rule)
SELECT id, price_rule
FROM workflow_definitions;

DROP TABLE IF EXISTS system_config_price_credit_migration_backup_152;
CREATE TABLE system_config_price_credit_migration_backup_152 (
  key TEXT PRIMARY KEY,
  existed BOOLEAN NOT NULL,
  value JSONB,
  backed_up_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO system_config_price_credit_migration_backup_152 (key, existed, value)
SELECT wanted.key, cfg.key IS NOT NULL, cfg.value
FROM (VALUES
  ('payment_compute_rate'),
  ('payment_currency'),
  ('recharge_shop_enabled'),
  ('recharge_shop_url'),
  ('model_usd_to_compute_rate')
) AS wanted(key)
LEFT JOIN system_configs cfg ON cfg.key = wanted.key;

-- Canonical DB configuration. Runtime code reads these values; no UI price
-- multiplier remains hard-coded at 7.2 or 60.
INSERT INTO system_configs (key, value, updated_at)
VALUES
  ('payment_compute_rate', '1000'::jsonb, now()),
  ('payment_currency', '"CNY"'::jsonb, now()),
  ('recharge_shop_enabled', 'true'::jsonb, now()),
  ('recharge_shop_url', '"https://catfk.com/shop/yardai"'::jsonb, now()),
  ('model_usd_to_compute_rate', '7200'::jsonb, now())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_at = now();

-- Convert CNY/USD user prices. USD source rows are handled below from their
-- source_* fields because some were already converted using the old 60x rate.
WITH priced AS (
  SELECT m.id,
         m.price_rule,
         CASE
           WHEN lower(COALESCE(m.price_rule->>'currency', '')) IN ('¥', 'cny', 'rmb')
             THEN 1000::numeric
           WHEN lower(COALESCE(m.price_rule->>'currency', '')) IN ('$', 'usd')
             THEN 7200::numeric
           ELSE 1::numeric
         END AS factor
  FROM models m
  WHERE m.price_rule->>'billing_type' IS DISTINCT FROM 'dynamic'
    AND upper(COALESCE(m.price_rule->>'source_currency', '')) <> 'USD'
)
UPDATE models m
SET price_rule = (
      SELECT COALESCE(jsonb_object_agg(key, value), '{}'::jsonb)
      FROM (
        SELECT key,
               CASE
                 WHEN p.factor <> 1 AND key = ANY(ARRAY[
                   'unit_price', 'input_price', 'output_price',
                   'cache_read_price', 'cache_write_price',
                   'audio_input_price', 'audio_output_price',
                   'input_price_per_m', 'output_price_per_m',
                   'cache_read_price_per_m', 'cache_write_price_per_m',
                   'audio_input_price_per_m', 'audio_output_price_per_m',
                   'surcharge_per_m', 'fallback_cost'
                 ])
                      THEN to_jsonb((value #>> '{}')::numeric * p.factor)
                 WHEN p.factor <> 1 AND key = ANY(ARRAY[
                   'unit_price_by_size', 'unit_price_by_resolution', 'unit_price_by_variant'
                 ])
                      THEN (
                        SELECT COALESCE(jsonb_object_agg(nested_key, to_jsonb(nested_value::numeric * p.factor)), '{}'::jsonb)
                        FROM jsonb_each_text(value) AS nested(nested_key, nested_value)
                        WHERE nested_value ~ '^-?[0-9]+([.][0-9]+)?$'
                      )
                 ELSE value
               END AS value
        FROM jsonb_each(p.price_rule) current(key, value)
        UNION ALL
        SELECT 'currency', '"CREDITS"'::jsonb
        WHERE NOT p.price_rule ? 'currency'
           OR lower(COALESCE(p.price_rule->>'currency', '')) IN ('¥', 'cny', 'rmb', '$', 'usd')
        UNION ALL
        SELECT 'price_unit_normalized', 'true'::jsonb
      ) normalized
    )
FROM priced p
WHERE m.id = p.id;

-- Rebuild OpenRouter USD user prices from raw source values. This covers audio
-- models, GPT audio token prices, and any future importer using source_*.
UPDATE models
SET price_rule = price_rule || jsonb_strip_nulls(jsonb_build_object(
  'unit_price',
    CASE WHEN price_rule ? 'source_unit_usd'
      THEN (price_rule->>'source_unit_usd')::numeric * 7200
      ELSE NULL
    END,
  'input_price_per_m',
    CASE WHEN price_rule ? 'source_input_price_per_m'
      THEN (price_rule->>'source_input_price_per_m')::numeric * 7200
      ELSE NULL
    END,
  'output_price_per_m',
    CASE WHEN price_rule ? 'source_output_price_per_m'
      THEN (price_rule->>'source_output_price_per_m')::numeric * 7200
      ELSE NULL
    END,
  'audio_input_price_per_m',
    CASE WHEN price_rule ? 'source_audio_input_price_per_m'
      THEN (price_rule->>'source_audio_input_price_per_m')::numeric * 7200
      ELSE NULL
    END,
  'audio_output_price_per_m',
    CASE WHEN price_rule ? 'source_audio_output_price_per_m'
      THEN (price_rule->>'source_audio_output_price_per_m')::numeric * 7200
      ELSE NULL
    END
)) || '{"usd_to_compute":7200,"currency":"CREDITS","price_unit_normalized":true}'::jsonb,
updated_at = now()
WHERE upper(COALESCE(price_rule->>'source_currency', '')) = 'USD';

-- Older imports copied the USD-to-credit rate onto every row, including CNY
-- models. A model-level rate overrides the system configuration at billing
-- time, so remove that stale marker from all non-USD models.
UPDATE models
SET price_rule = price_rule - 'usd_to_compute',
    updated_at = now()
WHERE upper(COALESCE(price_rule->>'source_currency', '')) <> 'USD'
  AND price_rule ? 'usd_to_compute';

-- BigModel was seeded with payment_compute_rate=60. Rebuild both model prices
-- and the associated official route from published CNY prices.
UPDATE models
SET price_rule = CASE code
    WHEN 'glm-5-3-flash' THEN jsonb_build_object(
      'billing_type', 'per_token',
      'currency', 'CREDITS',
      'input_price_per_m', 800,
      'output_price_per_m', 2800,
      'cache_read_price_per_m', 230,
      'source_currency', 'CNY',
      'source_input_price_per_m', 0.8,
      'source_output_price_per_m', 2.8,
      'source_cache_read_price_per_m', 0.23,
      'price_unit_normalized', true
    )
    WHEN 'glm-5-3-flashx' THEN jsonb_build_object(
      'billing_type', 'per_token',
      'currency', 'CREDITS',
      'input_price_per_m', 2000,
      'output_price_per_m', 7000,
      'cache_read_price_per_m', 570,
      'source_currency', 'CNY',
      'source_input_price_per_m', 2,
      'source_output_price_per_m', 7,
      'source_cache_read_price_per_m', 0.57,
      'price_unit_normalized', true
    )
  END,
  updated_at = now()
WHERE code IN ('glm-5-3-flash', 'glm-5-3-flashx');

UPDATE model_routes r
SET cost_rule = CASE m.code
    WHEN 'glm-5-3-flash' THEN jsonb_build_object(
      'billing_type', 'per_token',
      'currency', 'CREDITS',
      'input_cost_per_m', 800,
      'output_cost_per_m', 2800,
      'cache_read_cost_per_m', 230,
      'source_currency', 'CNY',
      'source_input_cost_per_m', 0.8,
      'source_output_cost_per_m', 2.8,
      'source_cache_read_cost_per_m', 0.23,
      'price_unit_normalized', true
    )
    WHEN 'glm-5-3-flashx' THEN jsonb_build_object(
      'billing_type', 'per_token',
      'currency', 'CREDITS',
      'input_cost_per_m', 2000,
      'output_cost_per_m', 7000,
      'cache_read_cost_per_m', 570,
      'source_currency', 'CNY',
      'source_input_cost_per_m', 2,
      'source_output_cost_per_m', 7,
      'source_cache_read_cost_per_m', 0.57,
      'price_unit_normalized', true
    )
  END,
  updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND m.code IN ('glm-5-3-flash', 'glm-5-3-flashx')
  AND r.provider = 'bigmodel';

-- Dynamic pricing components are published in CNY and were multiplied by
-- points_per_cny. Keep the components intact and change that multiplier once.
-- The fallback estimate was previously stored as a final charge, so convert
-- that one field before marking the row as normalized.
UPDATE models
SET price_rule = jsonb_set(
  price_rule,
  '{fallback_cost}',
  to_jsonb((price_rule->>'fallback_cost')::numeric * 1000),
  true
)
WHERE price_rule->>'billing_type' = 'dynamic'
  AND price_rule ? 'points_per_cny'
  AND NOT COALESCE((price_rule->>'price_unit_normalized')::boolean, false)
  AND COALESCE(price_rule->>'fallback_cost', '') ~ '^-?[0-9]+([.][0-9]+)?$'
  AND (price_rule->>'fallback_cost')::numeric > 0;

UPDATE models
SET price_rule = jsonb_set(
  jsonb_set(
    jsonb_set(price_rule, '{points_per_cny}', '1000'::jsonb, true),
    '{currency}', '"CREDITS"'::jsonb, true
  ),
  '{price_unit_normalized}', 'true'::jsonb, true
)
WHERE price_rule->>'billing_type' = 'dynamic'
  AND price_rule ? 'points_per_cny';

-- New/legacy unitless demo prices keep their numeric amount but gain an
-- explicit unit. They are visible to the user as credits.
UPDATE models
SET price_rule = price_rule
  || '{"currency":"CREDITS","price_unit_normalized":true}'::jsonb,
  updated_at = now()
WHERE price_rule <> '{}'::jsonb
  AND (NOT price_rule ? 'currency' OR price_rule->>'currency' NOT IN ('CREDITS', 'CNY', 'RMB', '¥'));

-- These workflow fees were seeded as small RMB values before the platform
-- moved to a 1 CNY = 1000 credit unit. Convert only this known set.
-- extract_product_images is excluded because migration 136 already set it
-- explicitly to 50 credits.
UPDATE workflow_definitions
SET price_rule = price_rule
  || jsonb_build_object(
       'unit_price', (price_rule->>'unit_price')::numeric * 1000,
       'currency', 'CREDITS',
       'price_unit_normalized', true
     ),
    updated_at = now()
WHERE COALESCE((price_rule->>'unit_price')::numeric, 0) > 0
  AND NOT COALESCE((price_rule->>'price_unit_normalized')::boolean, false)
  AND COALESCE(price_rule->>'currency', '') <> 'CREDITS'
  AND code IN (
    'ecommerce_video',
    'ecommerce_image',
    'general_image',
    'ai_novel_workshop',
    'ai_photo_studio',
    'ai_virtual_tryon',
    'viral_video_breakdown'
  );

-- Route costs are for margin reporting, not user charging. Rebuild USD and
-- CNY source rows from raw values and retain an explicit source currency.
UPDATE model_routes r
SET cost_rule = CASE
  WHEN upper(COALESCE(r.cost_rule->>'source_currency', '')) = 'USD' THEN
    r.cost_rule || jsonb_strip_nulls(jsonb_build_object(
      'unit_cost',
        CASE WHEN r.cost_rule ? 'source_unit_cost'
          THEN (r.cost_rule->>'source_unit_cost')::numeric * 7200
          ELSE NULL
        END,
      'input_cost_per_m',
        CASE WHEN r.cost_rule ? 'source_input_cost_per_m'
          THEN (r.cost_rule->>'source_input_cost_per_m')::numeric * 7200
          ELSE NULL
        END,
      'output_cost_per_m',
        CASE WHEN r.cost_rule ? 'source_output_cost_per_m'
          THEN (r.cost_rule->>'source_output_cost_per_m')::numeric * 7200
          ELSE NULL
        END,
      'audio_input_cost_per_m',
        CASE WHEN r.cost_rule ? 'source_audio_input_cost_per_m'
          THEN (r.cost_rule->>'source_audio_input_cost_per_m')::numeric * 7200
          ELSE NULL
        END,
      'audio_output_cost_per_m',
        CASE WHEN r.cost_rule ? 'source_audio_output_cost_per_m'
          THEN (r.cost_rule->>'source_audio_output_cost_per_m')::numeric * 7200
          ELSE NULL
        END
    )) || '{"currency":"CREDITS","usd_to_compute":7200,"price_unit_normalized":true}'::jsonb
  WHEN upper(COALESCE(r.cost_rule->>'currency', '')) IN ('CNY', 'RMB')
       OR r.cost_rule->>'currency' = '¥' THEN
    r.cost_rule || jsonb_strip_nulls(jsonb_build_object(
      'unit_cost',
        CASE WHEN r.cost_rule ? 'unit_cost' THEN (r.cost_rule->>'unit_cost')::numeric * 1000 ELSE NULL END,
      'input_cost_per_m',
        CASE WHEN r.cost_rule ? 'input_cost_per_m' THEN (r.cost_rule->>'input_cost_per_m')::numeric * 1000 ELSE NULL END,
      'output_cost_per_m',
        CASE WHEN r.cost_rule ? 'output_cost_per_m' THEN (r.cost_rule->>'output_cost_per_m')::numeric * 1000 ELSE NULL END,
      'cache_read_cost_per_m',
        CASE WHEN r.cost_rule ? 'cache_read_cost_per_m' THEN (r.cost_rule->>'cache_read_cost_per_m')::numeric * 1000 ELSE NULL END,
      'cache_write_cost_per_m',
        CASE WHEN r.cost_rule ? 'cache_write_cost_per_m' THEN (r.cost_rule->>'cache_write_cost_per_m')::numeric * 1000 ELSE NULL END,
      'audio_input_cost_per_m',
        CASE WHEN r.cost_rule ? 'audio_input_cost_per_m' THEN (r.cost_rule->>'audio_input_cost_per_m')::numeric * 1000 ELSE NULL END,
      'audio_output_cost_per_m',
        CASE WHEN r.cost_rule ? 'audio_output_cost_per_m' THEN (r.cost_rule->>'audio_output_cost_per_m')::numeric * 1000 ELSE NULL END
    )) || '{"currency":"CREDITS","compute_per_cny":1000,"price_unit_normalized":true}'::jsonb
  WHEN upper(COALESCE(r.cost_rule->>'source_currency', '')) = 'CNY' THEN
    r.cost_rule || '{"currency":"CREDITS","compute_per_cny":1000,"price_unit_normalized":true}'::jsonb
  WHEN r.provider <> 'openrouter' THEN
    r.cost_rule || '{"currency":"CREDITS","price_unit_normalized":true}'::jsonb
  ELSE r.cost_rule
END,
updated_at = now()
WHERE r.cost_rule <> '{}'::jsonb;

-- The import scripts historically stored USD amounts in OpenRouter image and
-- video route costs without a source_currency marker. Convert those direct
-- USD costs to their credit equivalent and preserve the source amount.
UPDATE model_routes r
SET cost_rule = jsonb_build_object(
      'billing_type', COALESCE(r.cost_rule->>'billing_type', 'per_request'),
      'currency', 'CREDITS',
      'source_currency', 'USD',
      'usd_to_compute', 7200,
      'unit_cost',
        COALESCE((r.cost_rule->>'unit_cost')::numeric, 0) * 7200,
      'source_unit_cost',
        COALESCE((r.cost_rule->>'unit_cost')::numeric, 0),
      'unit_cost_by_size',
        COALESCE((
          SELECT jsonb_object_agg(key, to_jsonb(value::numeric * 7200))
          FROM jsonb_each_text(COALESCE(r.cost_rule->'unit_cost_by_size', '{}'::jsonb))
          WHERE value ~ '^-?[0-9]+([.][0-9]+)?$'
        ), '{}'::jsonb),
      'unit_cost_by_resolution',
        COALESCE((
          SELECT jsonb_object_agg(key, to_jsonb(value::numeric * 7200))
          FROM jsonb_each_text(COALESCE(r.cost_rule->'unit_cost_by_resolution', '{}'::jsonb))
          WHERE value ~ '^-?[0-9]+([.][0-9]+)?$'
        ), '{}'::jsonb),
      'unit_cost_by_variant',
        COALESCE((
          SELECT jsonb_object_agg(key, to_jsonb(value::numeric * 7200))
          FROM jsonb_each_text(COALESCE(r.cost_rule->'unit_cost_by_variant', '{}'::jsonb))
          WHERE value ~ '^-?[0-9]+([.][0-9]+)?$'
        ), '{}'::jsonb),
      'price_unit_normalized', true
    ) || jsonb_strip_nulls(jsonb_build_object(
      'source_input_cost_per_m',
        CASE WHEN r.cost_rule ? 'input_cost_per_m' THEN (r.cost_rule->>'input_cost_per_m')::numeric ELSE NULL END,
      'source_output_cost_per_m',
        CASE WHEN r.cost_rule ? 'output_cost_per_m' THEN (r.cost_rule->>'output_cost_per_m')::numeric ELSE NULL END,
      'input_cost_per_m',
        CASE WHEN r.cost_rule ? 'input_cost_per_m' THEN (r.cost_rule->>'input_cost_per_m')::numeric * 7200 ELSE NULL END,
      'output_cost_per_m',
        CASE WHEN r.cost_rule ? 'output_cost_per_m' THEN (r.cost_rule->>'output_cost_per_m')::numeric * 7200 ELSE NULL END
    )),
    updated_at = now()
WHERE r.provider = 'openrouter'
  AND upper(COALESCE(r.cost_rule->>'source_currency', '')) <> 'USD'
  AND NOT COALESCE((r.cost_rule->>'price_unit_normalized')::boolean, false)
  AND (
    r.cost_rule ? 'unit_cost'
    OR r.cost_rule ? 'input_cost_per_m'
    OR r.cost_rule ? 'unit_cost_by_size'
    OR r.cost_rule ? 'unit_cost_by_resolution'
    OR r.cost_rule ? 'unit_cost_by_variant'
  );

-- Every workflow already deducts from the same credit wallet. Mark all rows
-- explicitly as CREDITS. Fixed CNY demo fees were converted above; model_actual,
-- zero-price, and the already-credit-priced extract_product_images workflow
-- keep their existing amount and only receive the normalized unit marker.
UPDATE workflow_definitions
SET price_rule = price_rule || '{"currency":"CREDITS","price_unit_normalized":true}'::jsonb,
    updated_at = now()
WHERE price_rule <> '{}'::jsonb
  AND (
    NOT price_rule ? 'currency'
    OR upper(COALESCE(price_rule->>'currency', '')) <> 'CREDITS'
  );

-- Keep all three backup tables for rollback migration 152 down.

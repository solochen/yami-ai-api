-- GLM-5.3 Flash/FlashX use BigModel's official OpenAI-compatible API as the
-- primary route. Keep the existing OpenRouter routes as lower-priority
-- failover paths. BigModel prices are CNY per million tokens; route cost and
-- user price are converted to the platform's current compute-credit rate.
WITH billing AS (
  SELECT COALESCE(
    (SELECT NULLIF((value #>> '{}')::numeric, 0)
       FROM system_configs
      WHERE key = 'payment_compute_rate'),
    60::numeric
  ) AS compute_per_cny
)
UPDATE models AS m
SET
  new_api_model = CASE m.code
    WHEN 'glm-5-3-flash' THEN 'glm-5.3-flash'
    WHEN 'glm-5-3-flashx' THEN 'glm-5.3-flashx'
  END,
  new_api_endpoint = '/chat/completions',
  new_api_extra_params = jsonb_set(
    COALESCE(m.new_api_extra_params, '{}'::jsonb),
    '{connection}',
    jsonb_build_object(
      'provider', 'bigmodel',
      'protocol', 'openai_compatible',
      'base_url', 'https://open.bigmodel.cn/api/paas/v4',
      'api_key', '',
      'auth_type', 'bearer',
      'api_key_header', 'Authorization',
      'models_endpoint', '/models'
    ),
    true
  ),
  price_rule = CASE m.code
    WHEN 'glm-5-3-flash' THEN jsonb_build_object(
      'billing_type', 'per_token',
      'input_price_per_m', 0.8 * billing.compute_per_cny,
      'output_price_per_m', 2.8 * billing.compute_per_cny,
      'cache_read_price_per_m', 0.23 * billing.compute_per_cny
    )
    WHEN 'glm-5-3-flashx' THEN jsonb_build_object(
      'billing_type', 'per_token',
      'input_price_per_m', 2 * billing.compute_per_cny,
      'output_price_per_m', 7 * billing.compute_per_cny,
      'cache_read_price_per_m', 0.57 * billing.compute_per_cny
    )
  END,
  runtime_rule = jsonb_set(
    COALESCE(m.runtime_rule, '{}'::jsonb),
    '{reasoning}',
    COALESCE(m.runtime_rule->'reasoning', '{}'::jsonb) || jsonb_build_object(
      'mode', 'reasoning_effort',
      'on_effort', 'max',
      'off_effort', 'low',
      'default_enabled', true
    ),
    true
  ),
  updated_at = now()
FROM billing
WHERE m.code IN ('glm-5-3-flash', 'glm-5-3-flashx');

WITH billing AS (
  SELECT COALESCE(
    (SELECT NULLIF((value #>> '{}')::numeric, 0)
       FROM system_configs
      WHERE key = 'payment_compute_rate'),
    60::numeric
  ) AS compute_per_cny
)
INSERT INTO model_routes (
  model_id,
  route_name,
  provider,
  protocol,
  upstream_model,
  endpoint,
  base_url,
  api_key,
  auth_type,
  api_key_header,
  headers,
  extra_params,
  runtime_rule,
  cost_rule,
  priority,
  weight,
  timeout_seconds,
  max_retries,
  is_enabled,
  health_status,
  consecutive_failures,
  cooldown_until,
  updated_at
)
SELECT
  m.id,
  'BigModel 官方',
  'bigmodel',
  'openai',
  CASE m.code
    WHEN 'glm-5-3-flash' THEN 'glm-5.3-flash'
    WHEN 'glm-5-3-flashx' THEN 'glm-5.3-flashx'
  END,
  '/chat/completions',
  'https://open.bigmodel.cn/api/paas/v4',
  '',
  'bearer',
  'Authorization',
  '{}'::jsonb,
  '{}'::jsonb,
  '{}'::jsonb,
  CASE m.code
    WHEN 'glm-5-3-flash' THEN jsonb_build_object(
      'billing_type', 'per_token',
      'input_cost_per_m', 0.8 * billing.compute_per_cny,
      'output_cost_per_m', 2.8 * billing.compute_per_cny,
      'cache_read_cost_per_m', 0.23 * billing.compute_per_cny,
      'source_currency', 'CNY',
      'source_input_cost_per_m', 0.8,
      'source_output_cost_per_m', 2.8,
      'source_cache_read_cost_per_m', 0.23,
      'compute_per_cny', billing.compute_per_cny
    )
    WHEN 'glm-5-3-flashx' THEN jsonb_build_object(
      'billing_type', 'per_token',
      'input_cost_per_m', 2 * billing.compute_per_cny,
      'output_cost_per_m', 7 * billing.compute_per_cny,
      'cache_read_cost_per_m', 0.57 * billing.compute_per_cny,
      'source_currency', 'CNY',
      'source_input_cost_per_m', 2,
      'source_output_cost_per_m', 7,
      'source_cache_read_cost_per_m', 0.57,
      'compute_per_cny', billing.compute_per_cny
    )
  END,
  10,
  100,
  600,
  0,
  true,
  'healthy',
  0,
  NULL,
  now()
FROM models AS m
CROSS JOIN billing
WHERE m.code IN ('glm-5-3-flash', 'glm-5-3-flashx')
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider,
  protocol = EXCLUDED.protocol,
  upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint,
  base_url = EXCLUDED.base_url,
  api_key = CASE
    WHEN model_routes.api_key = '' THEN EXCLUDED.api_key
    ELSE model_routes.api_key
  END,
  auth_type = EXCLUDED.auth_type,
  api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule,
  cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority,
  weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled,
  health_status = 'healthy',
  consecutive_failures = 0,
  cooldown_until = NULL,
  updated_at = now();

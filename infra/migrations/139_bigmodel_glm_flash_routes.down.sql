UPDATE models AS m
SET
  new_api_model = CASE m.code
    WHEN 'glm-5-3-flash' THEN 'z-ai/glm-5.3-flash'
    WHEN 'glm-5-3-flashx' THEN 'z-ai/glm-5.3-flashx'
  END,
  new_api_endpoint = '/v1/chat/completions',
  new_api_extra_params = jsonb_set(
    COALESCE(m.new_api_extra_params, '{}'::jsonb),
    '{connection}',
    jsonb_build_object(
      'provider', 'openrouter',
      'protocol', 'openai_compatible',
      'base_url', 'https://openrouter.ai/api/v1',
      'api_key', COALESCE((
        SELECT r.api_key
        FROM model_routes AS r
        WHERE r.model_id = m.id AND r.route_name = 'OpenRouter'
        ORDER BY r.id
        LIMIT 1
      ), ''),
      'auth_type', 'bearer',
      'api_key_header', 'Authorization',
      'models_endpoint', '/v1/models'
    ),
    true
  ),
  price_rule = CASE m.code
    WHEN 'glm-5-3-flash' THEN '{"currency":"$","billing_type":"per_token","input_price_per_m":0.045,"output_price_per_m":0.6,"cache_read_price_per_m":0.0285}'::jsonb
    WHEN 'glm-5-3-flashx' THEN '{"currency":"$","billing_type":"per_token","input_price_per_m":0.37,"output_price_per_m":1.25,"cache_read_price_per_m":0.09}'::jsonb
  END,
  runtime_rule = jsonb_set(
    COALESCE(m.runtime_rule, '{}'::jsonb),
    '{reasoning,on_effort}',
    '"high"'::jsonb,
    true
  ),
  updated_at = now()
WHERE m.code IN ('glm-5-3-flash', 'glm-5-3-flashx');

DELETE FROM model_routes
WHERE route_name = 'BigModel 官方'
  AND model_id IN (
    SELECT id FROM models WHERE code IN ('glm-5-3-flash', 'glm-5-3-flashx')
  );

-- 回滚 160：编码与上游还原为 0731 版本。
UPDATE models
SET code = 'deepseek-v4-flash-0731',
    display_name = 'DeepSeek-V4-Flash-0731',
    new_api_model = 'deepseek/deepseek-v4-flash-0731',
    updated_at = now()
WHERE code = 'deepseek-v4-1-flash-1017';

UPDATE system_configs
SET value = '"deepseek-v4-flash-0731"'::jsonb, updated_at = now()
WHERE key = 'chat_compression_model_code'
  AND value #>> '{}' = 'deepseek-v4-1-flash-1017';

UPDATE workflow_definitions
SET runtime_config = jsonb_set(runtime_config, '{chat_model_code}', '"deepseek-v4-flash-0731"'::jsonb),
    updated_at = now()
WHERE runtime_config ->> 'chat_model_code' = 'deepseek-v4-1-flash-1017';

UPDATE model_routes r
SET upstream_model = 'deepseek/deepseek-v4-flash-0731',
    updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND r.provider = 'tokenhub'
  AND r.upstream_model = 'deepseek/deepseek-flash';

UPDATE model_routes r
SET upstream_model = 'deepseek/deepseek-v4-flash-0731',
    cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":2,"output_cost_per_m":8,"cache_read_cost_per_m":0.2,"source_input_cost_per_m":2,"source_output_cost_per_m":8}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND r.provider = 'openrouter'
  AND r.upstream_model = 'deepseek/deepseek-v4.1-flash-1017';

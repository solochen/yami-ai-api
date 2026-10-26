-- 回滚 161：移除 V4-Pro 的 TokenHub 线路，成本价还原为此前的值。
DELETE FROM model_routes r
USING models m
WHERE m.id = r.model_id AND m.code = 'deepseek-v4-pro-0813' AND r.provider = 'tokenhub';

UPDATE model_routes r
SET cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":2,"output_cost_per_m":8,"cache_read_cost_per_m":0.2,"source_input_cost_per_m":2,"source_output_cost_per_m":8}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id AND m.code = 'deepseek-v4-pro-0813' AND r.provider = 'openrouter';

UPDATE models
SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":2,"output_price_per_m":8,"cache_read_price_per_m":0.2}'::jsonb,
    updated_at = now()
WHERE code = 'deepseek-v4-pro-0813';

UPDATE model_routes r
SET cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":2,"output_cost_per_m":8,"cache_read_cost_per_m":0.04,"source_input_cost_per_m":2,"source_output_cost_per_m":8}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id AND m.code = 'deepseek-v4-1-flash-1017' AND r.provider = 'tokenhub';

UPDATE models
SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":2,"output_price_per_m":8,"cache_read_price_per_m":0.2}'::jsonb,
    updated_at = now()
WHERE code = 'deepseek-v4-1-flash-1017';

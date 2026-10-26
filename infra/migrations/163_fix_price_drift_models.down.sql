-- 回滚 163：恢复校正前的过期配置（仅用于回退，值本身是错的）。
UPDATE model_routes r SET cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"input_cost_per_m":14.4,"output_cost_per_m":72,"cache_read_cost_per_m":1.44}'::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gpt-5-6-sol' AND r.provider = 'openrouter';
UPDATE models SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":14.4,"output_price_per_m":72,"cache_read_price_per_m":1.44}'::jsonb, updated_at = now() WHERE code = 'gpt-5-6-sol';

UPDATE model_routes r SET cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"input_cost_per_m":3.024,"output_cost_per_m":21.6,"cache_read_cost_per_m":0.612}'::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'qwen3-8-27b' AND r.provider = 'openrouter';
UPDATE models SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":3.024,"output_price_per_m":21.6,"cache_read_price_per_m":0.612}'::jsonb, updated_at = now() WHERE code = 'qwen3-8-27b';

UPDATE model_routes r SET cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"input_cost_per_m":5.4,"output_cost_per_m":27,"cache_read_cost_per_m":0.54}'::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gemini-3-8-flash' AND r.provider = 'openrouter';
UPDATE models SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":5.4,"output_price_per_m":27,"cache_read_price_per_m":0.54}'::jsonb, updated_at = now() WHERE code = 'gemini-3-8-flash';

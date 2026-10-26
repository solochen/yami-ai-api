-- 校正 3 个模型的过期上游成本（价格漂移巡检首次发现）。
--
-- 巡检比对 OpenRouter 实时价与线路配置成本，发现：
--   gpt-5-6-sol      配置 $2/$10/缓存 $0.2   → 现价 $4/$20/缓存 $0.4（+100%，一直在亏）
--   qwen3-8-27b      配置 $0.42/$3/缓存 $0.085 → 现价 $0.2/$2.5/缓存 $0.09（上游降价，我们超收）
--   gemini-3-8-flash 配置 $0.75/$3.75/缓存 $0.075 → 现价 $0.825/$4.125/缓存 $0.0825（+10%）
--
-- 统一改为上游现价折算（1 USD = 7.2 算力），并写入 source_* USD 原值，
-- 便于后续巡检直接按原值比对。这些模型没有 user_price_rule，用户售价
-- 自动随成本更新（成本 ÷ (1 − 3% 毛利)）。

-- ========= gpt-5-6-sol：$4 / $20 / 缓存 $0.4 =========
UPDATE model_routes r
SET cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"USD","usd_to_compute":7.2,"input_cost_per_m":28.8,"output_cost_per_m":144,"cache_read_cost_per_m":2.88,"source_input_cost_per_m":4,"source_output_cost_per_m":20,"source_cache_read_cost_per_m":0.4}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id AND m.code = 'gpt-5-6-sol' AND r.provider = 'openrouter';

UPDATE models
SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":28.8,"output_price_per_m":144,"cache_read_price_per_m":2.88}'::jsonb,
    updated_at = now()
WHERE code = 'gpt-5-6-sol';

-- ========= qwen3-8-27b：$0.2 / $2.5 / 缓存 $0.09 =========
UPDATE model_routes r
SET cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"USD","usd_to_compute":7.2,"input_cost_per_m":1.44,"output_cost_per_m":18,"cache_read_cost_per_m":0.648,"source_input_cost_per_m":0.2,"source_output_cost_per_m":2.5,"source_cache_read_cost_per_m":0.09}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id AND m.code = 'qwen3-8-27b' AND r.provider = 'openrouter';

UPDATE models
SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":1.44,"output_price_per_m":18,"cache_read_price_per_m":0.648}'::jsonb,
    updated_at = now()
WHERE code = 'qwen3-8-27b';

-- ========= gemini-3-8-flash：$0.825 / $4.125 / 缓存 $0.0825 =========
UPDATE model_routes r
SET cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"USD","usd_to_compute":7.2,"input_cost_per_m":5.94,"output_cost_per_m":29.7,"cache_read_cost_per_m":0.594,"source_input_cost_per_m":0.825,"source_output_cost_per_m":4.125,"source_cache_read_cost_per_m":0.0825}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id AND m.code = 'gemini-3-8-flash' AND r.provider = 'openrouter';

UPDATE models
SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":5.94,"output_price_per_m":29.7,"cache_read_price_per_m":0.594}'::jsonb,
    updated_at = now()
WHERE code = 'gemini-3-8-flash';

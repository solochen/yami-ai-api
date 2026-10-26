-- DeepSeek-V4-Pro-0813 接入腾讯云 TokenHub 并修正两条线路的真实价格。
--
-- 存量问题：该模型此前的成本价是 ¥2/¥8，与 DeepSeek-V4-Flash 完全相同（明显是
-- 导入时的错价），而 OpenRouter 现价是 $0.66/$1.98（≈¥4.75/¥14.26）——按 2/8
-- 计费等于每笔都在亏。
--
-- 真实价格：
--   腾讯云 TokenHub（V4-Pro 0813 原厂直供）：闲时 ¥4.5/13.5（缓存 0.15），
--     峰时 ¥9/27（缓存 0.3），峰时倍率正好 2；峰谷时段为北京时间工作日
--     9:00-12:00、14:00-18:00。
--   OpenRouter：$0.66/$1.98/缓存 $0.022 → 按 1 USD = 7.2 折算 4.752/14.256/0.1584。
--
-- 平台成本模型是静态的，因此 TokenHub 线路按【闲时价】写入，并用
-- peak_multiplier / peak_ranges 声明峰时倍率，由计费引擎按当前时刻放大
-- （见 routeCostTimeMultiplier，API 与 worker 两侧都已支持）。

-- ========= 1. 新增腾讯云 TokenHub 线路（优先级 20，优先于 OpenRouter）=========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, '腾讯云 TokenHub', 'tokenhub', 'openai', 'deepseek/deepseek-v4-pro-0813',
       '/v1/chat/completions', 'https://tokenhub.tencentmaas.com/v1',
       'REPLACE_ME_TENCENT_TOKENHUB_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
       '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":4.5,"output_cost_per_m":13.5,"cache_read_cost_per_m":0.15,"source_input_cost_per_m":4.5,"source_output_cost_per_m":13.5,"source_cache_read_cost_per_m":0.15,"peak_multiplier":2,"peak_ranges":["09:00-12:00","14:00-18:00"],"peak_weekdays_only":true}'::jsonb,
       20, 100, 600, 0, true
FROM models m
WHERE m.code = 'deepseek-v4-pro-0813'
  AND NOT EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.provider = 'tokenhub');

-- ========= 2. OpenRouter 线路按现价修正成本 =========
UPDATE model_routes r
SET cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"USD","usd_to_compute":7.2,"input_cost_per_m":4.752,"output_cost_per_m":14.256,"cache_read_cost_per_m":0.1584,"source_input_cost_per_m":0.66,"source_output_cost_per_m":1.98,"source_cache_read_cost_per_m":0.022}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND m.code = 'deepseek-v4-pro-0813'
  AND r.provider = 'openrouter';

-- ========= 3. 模型自身价格对齐主线路上游（闲时基价）=========
UPDATE models
SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":4.5,"output_price_per_m":13.5,"cache_read_price_per_m":0.15}'::jsonb,
    updated_at = now()
WHERE code = 'deepseek-v4-pro-0813';

-- ========= 4. V4.1-Flash 的 TokenHub 线路改用「闲时基价 + 峰时倍率」=========
-- 此前为避免峰时亏损写死了峰时价 2/8，现在计费引擎支持峰谷倍率，
-- 改成闲时 1/4 + peak_multiplier 2：闲时用户更便宜，峰时自动翻倍，仍不亏。
UPDATE model_routes r
SET cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":1,"output_cost_per_m":4,"cache_read_cost_per_m":0.02,"source_input_cost_per_m":1,"source_output_cost_per_m":4,"peak_multiplier":2,"peak_ranges":["09:00-12:00","14:00-18:00"],"peak_weekdays_only":true}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND m.code = 'deepseek-v4-1-flash-1017'
  AND r.provider = 'tokenhub';

-- ========= 5. 模型自身价格同步为闲时基价 =========
UPDATE models
SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":1,"output_price_per_m":4,"cache_read_price_per_m":0.02}'::jsonb,
    updated_at = now()
WHERE code = 'deepseek-v4-1-flash-1017';

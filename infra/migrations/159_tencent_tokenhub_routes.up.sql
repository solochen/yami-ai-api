-- 腾讯云 TokenHub 直连线路（国产模型）。
--
-- 背景：这些模型此前只走 OpenRouter，而 TokenHub 卖的是原厂官方人民币价，
-- 比 OpenRouter 便宜（DeepSeek-V4-Flash-0731 便宜 37%~68%，GLM 与智谱直连同价
-- 但可作为 BigModel 的国内备份线路）。
--
-- 官方定价：https://cloud.tencent.com/document/product/1823/130055
-- 接口协议：OpenAI 兼容，Base URL https://tokenhub.tencentmaas.com/v1，
--           鉴权 Authorization: Bearer <API KEY>（密钥在
--           https://console.cloud.tencent.com/tokenhub/apikey 创建）。
--
-- 注意：
-- 1) api_key 先写入占位符，需在后台「模型管理 → 线路」里替换为真实密钥。
--    占位符期间该线路会返回 401，平台的线路故障转移会自动回退到原线路，
--    不影响线上可用性（只是每次多一次失败请求）。
-- 2) DeepSeek 在 TokenHub 有峰谷计价（工作日 9-12、14-18 为峰时），
--    平台成本模型是静态的，这里按【峰时价】配置，保证任何时段都不亏；
--    若要按闲时价计费（更便宜但峰时会亏），改小 input/output_cost_per_m 即可。
-- 3) priority=20：低于原直连线路（BigModel=10），高于 OpenRouter（100），
--    即优先用国内直连，失败再回退 OpenRouter。

-- ========= DeepSeek-V4-Flash-0731（原厂直供：闲 1/4、峰 2/8，缓存命中 0.04）=========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, '腾讯云 TokenHub', 'tokenhub', 'openai', 'deepseek/deepseek-v4-flash-0731',
       '/v1/chat/completions', 'https://tokenhub.tencentmaas.com/v1',
       'REPLACE_ME_TENCENT_TOKENHUB_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
       '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":2,"output_cost_per_m":8,"cache_read_cost_per_m":0.04,"source_input_cost_per_m":2,"source_output_cost_per_m":8}'::jsonb,
       20, 100, 600, 0, true
FROM models m
WHERE m.code = 'deepseek-v4-flash-0731'
  AND NOT EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.provider = 'tokenhub');

-- ========= GLM-5.3-Flash（0.8 / 2.8，缓存命中 0.23；与智谱直连同价）=========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, '腾讯云 TokenHub', 'tokenhub', 'openai', 'glm-5.3-flash',
       '/v1/chat/completions', 'https://tokenhub.tencentmaas.com/v1',
       'REPLACE_ME_TENCENT_TOKENHUB_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
       '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.8,"output_cost_per_m":2.8,"cache_read_cost_per_m":0.23,"source_input_cost_per_m":0.8,"source_output_cost_per_m":2.8}'::jsonb,
       20, 100, 600, 0, true
FROM models m
WHERE m.code = 'glm-5-3-flash'
  AND NOT EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.provider = 'tokenhub');

-- ========= GLM-5.3-FlashX（2 / 7，缓存命中 0.57）=========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, '腾讯云 TokenHub', 'tokenhub', 'openai', 'glm-5.3-flashx',
       '/v1/chat/completions', 'https://tokenhub.tencentmaas.com/v1',
       'REPLACE_ME_TENCENT_TOKENHUB_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
       '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":2,"output_cost_per_m":7,"cache_read_cost_per_m":0.57,"source_input_cost_per_m":2,"source_output_cost_per_m":7}'::jsonb,
       20, 100, 600, 0, true
FROM models m
WHERE m.code = 'glm-5-3-flashx'
  AND NOT EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.provider = 'tokenhub');

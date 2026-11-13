-- 203_model_gpt_6_1_sol：接入 OpenAI gpt-6.1-sol（HL 6.1 Sol），多上游线路。
--
-- 官方价（OpenRouter 口径）：输入 $2 / 输出 $10 / 缓存读 $0.1 / 缓存写 $2.5 每百万 token。
-- 售价沿用 7.2 汇率：14.4 / 72 / 0.72 / 18（CNY/百万 token），与 gpt-6-sol 相同。
--
-- 线路（三上游）：
--   PackyCode   azure-officially 分组  ¥10/¥50   优先级 2（主线路，Key 有效分组）
--   CodexAuv    GPT-Azure 分组         ¥27/¥135  优先级 100（分组已绑定，便于价格监控按组取成本）
--   新加坡中转  openai/gpt-6.1-sol     ¥13.5/¥67.5 优先级 120（中转站尚未上架该模型，先停用）
--
-- 回滚：见 203_model_gpt_6_1_sol.down.sql

INSERT INTO models (
  code, display_name, public_alias, icon_url, category, request_mode, description, tags,
  new_api_model, new_api_endpoint, new_api_extra_params,
  runtime_rule, input_schema, default_params, price_rule, is_enabled, sort_order
)
SELECT
  'gpt-6-1-sol', 'HL 6.1 Sol', 'hl-6.1-sol', '/assets/brand/hl.svg', 'chat', 'chat_completions',
  '6.1 代 Sol：位于旗舰 Astra 之下、快速档 Luna 之上的高性价比型号，面向要求较高的专业任务。',
  '["对话","HL 系列"]'::jsonb,
  'openai/gpt-6.1-sol', '/v1/chat/completions', '{}'::jsonb,
  '{"reasoning":{"mode":"reasoning_effort","on_effort":"high","off_effort":"none","default_enabled":true,"omit_temperature":true},"capabilities":{"vision":true,"deep_think":true,"web_search":false,"image_input":true,"audio_analysis":false,"video_analysis":false}}'::jsonb,
  '{}'::jsonb, '{}'::jsonb,
  '{"currency":"CREDITS","billing_type":"per_token","input_price_per_m":14.4,"output_price_per_m":72,"price_unit_normalized":true,"cache_read_price_per_m":0.72,"cache_write_price_per_m":18}'::jsonb,
  true, 199
WHERE NOT EXISTS (SELECT 1 FROM models WHERE code = 'gpt-6-1-sol');

-- ---------- PackyCode（主线路）----------
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule, price_group,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai', 'gpt-6.1-sol', '/v1/chat/completions', 'https://www.packyapi.ai',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='packycode' AND api_key<>'' LIMIT 1), ''),
  'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
  '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY","input_cost_per_m":10,"output_cost_per_m":50,"cache_read_cost_per_m":1,"cache_write_cost_per_m":12.5,"price_unit_normalized":true,"source_input_cost_per_m":10,"source_output_cost_per_m":50,"source_pricing_note":"PackyCode azure-officially 分组同步价"}'::jsonb,
  'azure-officially', 2, 100, 120, 1, true
FROM models m WHERE m.code='gpt-6-1-sol'
ON CONFLICT (model_id, route_name) DO NOTHING;

-- ---------- CodexAuv ----------
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule, price_group,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'gpt-6.1-sol', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''),
  'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
  '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY","input_cost_per_m":27,"output_cost_per_m":135,"price_unit_normalized":true,"source_input_cost_per_m":27,"source_output_cost_per_m":135,"source_pricing_note":"CodexAuv GPT-Azure 分组同步价；若账号分组不同请在后台修改价格分组"}'::jsonb,
  'GPT-Azure', 100, 100, 600, 0, true
FROM models m WHERE m.code='gpt-6-1-sol'
ON CONFLICT (model_id, route_name) DO NOTHING;

-- ---------- 新加坡中转（停用，待中转站上架）----------
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule, price_group,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'openai/gpt-6.1-sol', '/chat/completions', 'https://hub.yardai.cc/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='new_api' AND api_key<>'' LIMIT 1), ''),
  'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
  '{"currency":"CREDITS","billing_type":"per_token","usd_to_compute":6.75,"compute_per_cny":1,"source_currency":"USD","input_cost_per_m":13.5,"output_cost_per_m":67.5,"price_unit_normalized":true,"source_input_cost_per_m":2,"source_output_cost_per_m":10,"source_pricing_note":"OpenRouter 官方价×6.75；中转站上架 openai/gpt-6.1-sol 后启用"}'::jsonb,
  '', 120, 100, 1800, 0, false
FROM models m WHERE m.code='gpt-6-1-sol'
ON CONFLICT (model_id, route_name) DO NOTHING;

-- ---------- 价格监控映射 ----------
INSERT INTO upstream_model_mappings (provider, upstream_model, model_id, match_method, confidence, is_confirmed)
SELECT v.provider, v.upstream_model, m.id, 'manual', 1.0, true
FROM (VALUES
  ('packycode', 'gpt-6.1-sol'),
  ('codexauv',  'gpt-6.1-sol'),
  ('openrouter','openai/gpt-6.1-sol')
) AS v(provider, upstream_model)
JOIN models m ON m.code = 'gpt-6-1-sol'
ON CONFLICT (provider, upstream_model) DO NOTHING;

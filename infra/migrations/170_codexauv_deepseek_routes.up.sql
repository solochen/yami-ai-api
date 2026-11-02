-- 接入 CodexAuv 中转站（codexauv.com）作为 DeepSeek 两个模型的第一优先线路。
--
-- 背景：CodexAuv 是 OpenAI 兼容聚合站（API 基址 https://codexauv.com/v1），
-- 其「国产模型聚合」分组下 DeepSeek 报价（人民币/百万 token）：
--   deepseek-v4-pro-0813   输入 4.5 / 输出 13.5 / 缓存读 0.15（平价，无峰谷）
--   deepseek-v4.1-flash-1017 输入 1.0 / 输出 4.0 / 缓存读 0.02（平价，无峰谷）
-- 对比现有线路：
--   OpenRouter  v4-pro ¥4.752/¥14.256、flash ¥1.08/¥4.32（美元折算，平价）
--   TokenHub    v4-pro 闲时 ¥4.5/¥13.5 忙时 ×2、flash 闲时 ¥1/¥4 忙时 ×2
-- 结论：CodexAuv 与 TokenHub 闲时同价，但全程平价（免峰谷翻倍），优于 OpenRouter。
--
-- 优先级：本站 priority=10（最小者最优先），OpenRouter 20 保持次选，TokenHub 100 兜底。
-- 注意：api_key 为占位符，线路默认停用；在后台「模型线路管理」填入真实 Key 后再启用。
--   计费分组由 CodexAuv 账号的 Key 所属分组决定（模型广场的「国产模型聚合-大厂渠道」
--   分组倍率 0.43）；若账号被分到其他分组，价格可能不同，启用前请以模型广场
--   「登录后专属分组」页显示的单价为准，必要时同步调整本迁移中的 cost_rule。
-- 回滚：见 170_codexauv_deepseek_routes.down.sql

INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai',
       'deepseek-v4-pro-0813', '/v1/chat/completions', 'https://codexauv.com',
       'REPLACE_ME_CODEXAUV_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
       '{
         "currency": "CREDITS",
         "billing_type": "per_token",
         "compute_per_cny": 1,
         "source_currency": "CNY",
         "input_cost_per_m": 4.5,
         "output_cost_per_m": 13.5,
         "cache_read_cost_per_m": 0.15,
         "price_unit_normalized": true,
         "source_input_cost_per_m": 4.5,
         "source_output_cost_per_m": 13.5,
         "source_cache_read_cost_per_m": 0.15
       }'::jsonb,
       10, 100, 120, 1, false
FROM models m WHERE m.code = 'deepseek-v4-pro-0813';

INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai',
       'deepseek-v4.1-flash-1017', '/v1/chat/completions', 'https://codexauv.com',
       'REPLACE_ME_CODEXAUV_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
       '{
         "currency": "CREDITS",
         "billing_type": "per_token",
         "compute_per_cny": 1,
         "source_currency": "CNY",
         "input_cost_per_m": 1,
         "output_cost_per_m": 4,
         "cache_read_cost_per_m": 0.02,
         "price_unit_normalized": true,
         "source_input_cost_per_m": 1,
         "source_output_cost_per_m": 4,
         "source_cache_read_cost_per_m": 0.02
       }'::jsonb,
       10, 100, 120, 1, false
FROM models m WHERE m.code = 'deepseek-v4-1-flash-1017';

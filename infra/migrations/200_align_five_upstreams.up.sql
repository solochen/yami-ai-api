-- 200_align_five_upstreams: 以 PackyCode / CodexAuv / aiboli / OpenRouter / 七牛云 五家为唯一上游体系。
--
-- 1) 给现有模型补上五家中缺失的线路（56 条；aiboli 与 PackyCode 视频因缺 Key/协议未定默认停用）
-- 2) 删除五家之外的上游线路（41 条：tokenhub / tencent_aigc / bailian / new_api / volcano / bigmodel）
--    先备份到 model_routes_backup_200，down 迁移可精确恢复。
-- 3) 豁免：14 个模型（2 音频 + 12 向量）在五家无供应，保留原线路不动。
-- 4) 注意：gpt-image-2-5-flare / gpt-image-2-5-sunburst 删除 new_api 后，其替代线路
--    （PackyCode 图像分组 / aiboli）因 Key 缺失默认停用，两模型将暂不可售；
--    待 PackyCode 图像分组 Key 或 aiboli Key 到位后，启用对应线路即恢复上架。
--
-- 成本口径：
--   PackyCode = 官方组（含 official 字样优先，其次 bailian，否则最低组），公式 输入=model_ratio×2×组倍率
--   CodexAuv  = 最低分组价（人民币/百万 token，按次模型为 元/张）
--   OpenRouter= 挂牌 USD × 6.75 记人民币
--   七牛云    = 官方市场价（人民币，闲时档）
--   aiboli    = 占位 0（缺 Key 与充值倍率，线路默认停用，启用前须校准）

-- ============ 1. 新增五家线路 ============

-- gpt-6-luna += aiboli (luna-6)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'luna-6', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-luna'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-6-luna += codexauv (gpt-6-luna)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'gpt-6-luna', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0.1, "output_cost_per_m": 0.5, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 0.1, "source_output_cost_per_m": 0.5, "source_pricing_note": "CodexAuv 最低分组 GPT-Azure"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-luna'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-6-luna += openrouter (openai/gpt-6-luna)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'OpenRouter 主线路', 'openrouter', 'openai', 'openai/gpt-6-luna', '/chat/completions', 'https://openrouter.ai/api/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='openrouter' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0.675, "output_cost_per_m": 3.375, "usd_to_compute": 6.75, "source_currency": "USD", "source_input_cost_per_m": 0.1, "source_output_cost_per_m": 0.5}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-luna'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- glm-5-3-flash += aiboli (glm-5.3-flash)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'glm-5.3-flash', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'glm-5-3-flash'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-fable-5-1 += codexauv (claude-fable-5-1)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'claude-fable-5-1', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 10.0, "output_cost_per_m": 50.0, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 10.0, "source_output_cost_per_m": 50.0, "source_pricing_note": "CodexAuv 最低分组 Claude-Cursor反代"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'claude-fable-5-1'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-image-2-5-flare += aiboli (gpt-image-2.5-flare)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'gpt-image-2.5-flare', '/v1/images/generations', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 300, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gpt-image-2-5-flare'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-image-2-5-flare += packycode (gpt-image-2.5-flare)  [；PackyCode 分组 Key 未验证，待绑定正确分组 Key 后启用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai', 'gpt-image-2.5-flare', '/v1/images/generations', 'https://www.packyapi.ai',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='packycode' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{\"request_transform\": {\"size\": null, \"resolution\": \"1k\"}}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_image", "unit_cost": 0.0294, "source_currency": "CNY", "source_unit_cost": 0.0294, "source_pricing_note": "PackyCode 官方组 image x5，单价=挂牌价×组倍率（保守口径）"}'::jsonb,
  100, 100, 300, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gpt-image-2-5-flare'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-5-6-sol += aiboli (gpt-5.6-sol)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'gpt-5.6-sol', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gpt-5-6-sol'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-5-6-sol += codexauv (gpt-5.6-sol)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'gpt-5.6-sol', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 4.0, "output_cost_per_m": 20.0, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 4.0, "source_output_cost_per_m": 20.0, "source_pricing_note": "CodexAuv 最低分组 GPT-Azure"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-5-6-sol'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-5-6-sol += openrouter (openai/gpt-5.6-sol)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'OpenRouter 主线路', 'openrouter', 'openai', 'openai/gpt-5.6-sol', '/chat/completions', 'https://openrouter.ai/api/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='openrouter' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 27.0, "output_cost_per_m": 135.0, "usd_to_compute": 6.75, "source_currency": "USD", "source_input_cost_per_m": 4.0, "source_output_cost_per_m": 20.0}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-5-6-sol'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- qwen3-8-omni-flash += qiniu (qwen/qwen3.8-omni-flash)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '七牛云 Modelink', 'qiniu', 'openai', 'qwen/qwen3.8-omni-flash', '/v1/chat/completions', 'https://api.qnaigc.com/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='qiniu' AND auth_type='bearer' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0.8, "output_cost_per_m": 2.7, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 0.8, "source_output_cost_per_m": 2.7, "source_pricing_note": "七牛官方市场价"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'qwen3-8-omni-flash'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- wan-3-0 += aiboli (wan3.0)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'wan3.0', '/v1/video/generations', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 1800, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'wan-3-0'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gemini-3-7-flash += codexauv (gemini-3.7-flash-high)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'gemini-3.7-flash-high', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0.75, "output_cost_per_m": 3.75, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 0.75, "source_output_cost_per_m": 3.75, "source_pricing_note": "CodexAuv 最低分组 Gemini-Ultra-支持4a"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gemini-3-7-flash'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-image-2-5-sunburst += aiboli (gpt-image-2.5-sunburst)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'gpt-image-2.5-sunburst', '/v1/images/generations', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 300, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gpt-image-2-5-sunburst'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-image-2-5-sunburst += packycode (gpt-image-2.5-sunburst)  [；PackyCode 分组 Key 未验证，待绑定正确分组 Key 后启用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai', 'gpt-image-2.5-sunburst', '/v1/images/generations', 'https://www.packyapi.ai',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='packycode' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{\"request_transform\": {\"size\": null, \"resolution\": \"1k\"}}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_image", "unit_cost": 0.0294, "source_currency": "CNY", "source_unit_cost": 0.0294, "source_pricing_note": "PackyCode 官方组 image x5，单价=挂牌价×组倍率（保守口径）"}'::jsonb,
  100, 100, 300, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gpt-image-2-5-sunburst'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- deepseek-v4-1-flash-1017 += aiboli (deepseek-flash)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'deepseek-flash', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'deepseek-v4-1-flash-1017'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-image-2 += aiboli (gpt-image-2)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'gpt-image-2', '/v1/images/generations', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 300, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gpt-image-2'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-image-2 += codexauv (gpt-image-2)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'gpt-image-2', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 5.0, "output_cost_per_m": 10.0, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 5.0, "source_output_cost_per_m": 10.0, "source_pricing_note": "CodexAuv 最低分组 codex-速刷不降智"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-image-2'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-image-2 += packycode (gpt-image-2)  [；PackyCode 分组 Key 未验证，待绑定正确分组 Key 后启用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai', 'gpt-image-2', '/v1/images/generations', 'https://www.packyapi.ai',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='packycode' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{\"request_transform\": {\"size\": null, \"resolution\": \"1k\"}}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_image", "unit_cost": 0.5, "source_currency": "CNY", "source_unit_cost": 0.5, "source_pricing_note": "PackyCode 官方组 image x5，单价=挂牌价×组倍率（保守口径）"}'::jsonb,
  100, 100, 300, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gpt-image-2'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gemini-3-pro-image += aiboli (gemini-3-pro-image-2k)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'gemini-3-pro-image-2k', '/v1/images/generations', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 300, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gemini-3-pro-image'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gemini-3-pro-image += openrouter (google/gemini-3-pro-image)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'OpenRouter 主线路', 'openrouter', 'openai', 'google/gemini-3-pro-image', '/chat/completions', 'https://openrouter.ai/api/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='openrouter' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 13.5, "output_cost_per_m": 81.0, "usd_to_compute": 6.75, "source_currency": "USD", "source_input_cost_per_m": 2.0, "source_output_cost_per_m": 12.0}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gemini-3-pro-image'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- audio_openrouter_gpt_audio += openrouter (openai/gpt-audio)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'OpenRouter 主线路', 'openrouter', 'openai', 'openai/gpt-audio', '/chat/completions', 'https://openrouter.ai/api/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='openrouter' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{\"upstream\": {\"adapter\": \"openrouter_audio\"}}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 16.875, "output_cost_per_m": 67.5, "usd_to_compute": 6.75, "source_currency": "USD", "source_input_cost_per_m": 2.5, "source_output_cost_per_m": 10.0}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'audio_openrouter_gpt_audio'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- audio_openrouter_gpt_audio_mini += openrouter (openai/gpt-audio-mini)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'OpenRouter 主线路', 'openrouter', 'openai', 'openai/gpt-audio-mini', '/chat/completions', 'https://openrouter.ai/api/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='openrouter' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{\"upstream\": {\"adapter\": \"openrouter_audio\"}}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 4.05, "output_cost_per_m": 16.2, "usd_to_compute": 6.75, "source_currency": "USD", "source_input_cost_per_m": 0.6, "source_output_cost_per_m": 2.4}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'audio_openrouter_gpt_audio_mini'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- grok-imagine-image-2-0 += codexauv (grok-imagine-image-2.0)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'grok-imagine-image-2.0', '/v1/images/generations', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_image", "unit_cost": 0.05, "source_currency": "CNY", "source_unit_cost": 0.05, "source_pricing_note": "CodexAuv 按次 ¥0.05 · 分组 Grok"}'::jsonb,
  100, 100, 300, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'grok-imagine-image-2-0'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-5-6-luna += aiboli (gpt-5.6-luna)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'gpt-5.6-luna', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gpt-5-6-luna'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-5-6-luna += codexauv (gpt-5.6-luna)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'gpt-5.6-luna', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0.2, "output_cost_per_m": 1.2, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 0.2, "source_output_cost_per_m": 1.2, "source_pricing_note": "CodexAuv 最低分组 GPT-Azure"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-5-6-luna'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-5-6-luna += openrouter (openai/gpt-5.6-luna)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'OpenRouter 主线路', 'openrouter', 'openai', 'openai/gpt-5.6-luna', '/chat/completions', 'https://openrouter.ai/api/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='openrouter' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 1.35, "output_cost_per_m": 8.1, "usd_to_compute": 6.75, "source_currency": "USD", "source_input_cost_per_m": 0.2, "source_output_cost_per_m": 1.2}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-5-6-luna'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-6-astra += aiboli (astra-6)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'astra-6', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-astra'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-6-astra += codexauv (gpt-6-astra)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'gpt-6-astra', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 10.0, "output_cost_per_m": 50.0, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 10.0, "source_output_cost_per_m": 50.0, "source_pricing_note": "CodexAuv 最低分组 codex-速刷不降智"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-astra'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-6-astra += openrouter (openai/gpt-6-astra)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'OpenRouter 主线路', 'openrouter', 'openai', 'openai/gpt-6-astra', '/chat/completions', 'https://openrouter.ai/api/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='openrouter' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 67.5, "output_cost_per_m": 337.5, "usd_to_compute": 6.75, "source_currency": "USD", "source_input_cost_per_m": 10.0, "source_output_cost_per_m": 50.0}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-astra'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-opus-5-5 += aiboli (claude-opus-5-5)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'claude-opus-5-5', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'claude-opus-5-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-opus-5-5 += codexauv (claude-opus-5-5)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'claude-opus-5-5', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 4.0, "output_cost_per_m": 20.0, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 4.0, "source_output_cost_per_m": 20.0, "source_pricing_note": "CodexAuv 最低分组 Claude-Cursor反代"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'claude-opus-5-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-sonnet-5 += aiboli (claude-sonnet-5)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'claude-sonnet-5', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'claude-sonnet-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-sonnet-5 += codexauv (claude-sonnet-5)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'claude-sonnet-5', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 2.0, "output_cost_per_m": 10.0, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 2.0, "source_output_cost_per_m": 10.0, "source_pricing_note": "CodexAuv 最低分组 Claude-Max（保真正价）"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'claude-sonnet-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-fable-5 += aiboli (claude-fable-5)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'claude-fable-5', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'claude-fable-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-fable-5 += codexauv (claude-fable-5)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'claude-fable-5', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 10.0, "output_cost_per_m": 50.0, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 10.0, "source_output_cost_per_m": 50.0, "source_pricing_note": "CodexAuv 最低分组 Claude-Cursor反代"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'claude-fable-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- hailuo-3-max += packycode (MiniMax-H3-Max)  [PackyCode minimax-video 协议待适配，默认停用；PackyCode 分组 Key 未验证，待绑定正确分组 Key 后启用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai', 'MiniMax-H3-Max', '/v1/video_generation', 'https://www.packyapi.ai',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='packycode' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_image", "unit_cost": 0.5, "source_currency": "CNY", "source_unit_cost": 0.5, "source_pricing_note": "PackyCode 官方组 video x1，单价=挂牌价×组倍率（保守口径）"}'::jsonb,
  100, 100, 1800, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'hailuo-3-max'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gemini-3-8-flash += aiboli (gemini-3.8-flash-high)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'gemini-3.8-flash-high', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gemini-3-8-flash'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gemini-3-8-flash += codexauv (gemini-3.8-flash)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'gemini-3.8-flash', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0.75, "output_cost_per_m": 3.75, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 0.75, "source_output_cost_per_m": 3.75, "source_pricing_note": "CodexAuv 最低分组 Gemini-VT"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gemini-3-8-flash'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-opus-4-8 += aiboli (claude-opus-4-8)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'claude-opus-4-8', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'claude-opus-4-8'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-opus-4-8 += codexauv (claude-opus-4-8)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'claude-opus-4-8', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 5.0, "output_cost_per_m": 25.0, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 5.0, "source_output_cost_per_m": 25.0, "source_pricing_note": "CodexAuv 最低分组 Claude-Max（保真正价）"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'claude-opus-4-8'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-6-sol += aiboli (sol-6)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'sol-6', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-sol'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-6-sol += codexauv (gpt-6-sol)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'gpt-6-sol', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 2.0, "output_cost_per_m": 10.0, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 2.0, "source_output_cost_per_m": 10.0, "source_pricing_note": "CodexAuv 最低分组 codex-速刷不降智"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-sol'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-6-sol += openrouter (openai/gpt-6-sol)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'OpenRouter 主线路', 'openrouter', 'openai', 'openai/gpt-6-sol', '/chat/completions', 'https://openrouter.ai/api/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='openrouter' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 13.5, "output_cost_per_m": 67.5, "usd_to_compute": 6.75, "source_currency": "USD", "source_input_cost_per_m": 2.0, "source_output_cost_per_m": 10.0}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-sol'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- seedance-2-0 += aiboli (sd2.0-933-pro)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'sd2.0-933-pro', '/v1/video/generations', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 1800, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'seedance-2-0'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- hailuo-3 += packycode (MiniMax-H3)  [PackyCode minimax-video 协议待适配，默认停用；PackyCode 分组 Key 未验证，待绑定正确分组 Key 后启用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai', 'MiniMax-H3', '/v1/video_generation', 'https://www.packyapi.ai',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='packycode' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_image", "unit_cost": 0.5, "source_currency": "CNY", "source_unit_cost": 0.5, "source_pricing_note": "PackyCode 官方组 video x1，单价=挂牌价×组倍率（保守口径）"}'::jsonb,
  100, 100, 1800, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'hailuo-3'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- qwen3-6-plus += openrouter (qwen/qwen3.6-plus)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'OpenRouter 主线路', 'openrouter', 'openai', 'qwen/qwen3.6-plus', '/chat/completions', 'https://openrouter.ai/api/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='openrouter' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 2.19375, "output_cost_per_m": 13.1625, "usd_to_compute": 6.75, "source_currency": "USD", "source_input_cost_per_m": 0.325, "source_output_cost_per_m": 1.95}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'qwen3-6-plus'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- qwen3-6-plus += packycode (qwen3.6-plus)  [；PackyCode 分组 Key 未验证，待绑定正确分组 Key 后启用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai', 'qwen3.6-plus', '/v1/chat/completions', 'https://www.packyapi.ai',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='packycode' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 1.0, "output_cost_per_m": 6.0, "cache_read_cost_per_m": 0.1, "source_currency": "CNY", "source_input_cost_per_m": 1.0, "source_output_cost_per_m": 6.0, "source_cache_read_cost_per_m": 0.1, "source_pricing_note": "PackyCode 官方组 bailian x0.5"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'qwen3-6-plus'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- glm-5 += openrouter (z-ai/glm-5)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'OpenRouter 主线路', 'openrouter', 'openai', 'z-ai/glm-5', '/chat/completions', 'https://openrouter.ai/api/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='openrouter' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 6.75, "output_cost_per_m": 21.6, "usd_to_compute": 6.75, "source_currency": "USD", "source_input_cost_per_m": 1.0, "source_output_cost_per_m": 3.2}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'glm-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- glm-5 += packycode (glm-5)  [；PackyCode 分组 Key 未验证，待绑定正确分组 Key 后启用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai', 'glm-5', '/v1/chat/completions', 'https://www.packyapi.ai',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='packycode' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 4, "output_cost_per_m": 18.0, "cache_read_cost_per_m": 0.8, "source_currency": "CNY", "source_input_cost_per_m": 4, "source_output_cost_per_m": 18.0, "source_cache_read_cost_per_m": 0.8, "source_pricing_note": "PackyCode 官方组 zai-officially x1"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'glm-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- kimi-k2-6 += codexauv (kimi-k2.6)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'CodexAuv 低价聚合 · 主线路', 'codexauv', 'openai', 'kimi-k2.6', '/v1/chat/completions', 'https://codexauv.com',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='codexauv' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 6.5, "output_cost_per_m": 27.0, "cache_read_cost_per_m": 0, "source_currency": "CNY", "source_input_cost_per_m": 6.5, "source_output_cost_per_m": 27.0, "source_pricing_note": "CodexAuv 最低分组 国产模型聚合-阿里满血渠道"}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'kimi-k2-6'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- kimi-k2-6 += openrouter (moonshotai/kimi-k2.6)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'OpenRouter 主线路', 'openrouter', 'openai', 'moonshotai/kimi-k2.6', '/chat/completions', 'https://openrouter.ai/api/v1',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='openrouter' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 4.3875, "output_cost_per_m": 23.0175, "usd_to_compute": 6.75, "source_currency": "USD", "source_input_cost_per_m": 0.65, "source_output_cost_per_m": 3.41}'::jsonb,
  100, 100, 600, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'kimi-k2-6'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- kimi-k2-6 += packycode (kimi-k2.6)  [；PackyCode 分组 Key 未验证，待绑定正确分组 Key 后启用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai', 'kimi-k2.6', '/v1/messages', 'https://www.packyapi.ai',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='packycode' AND api_key<>'' LIMIT 1), ''), 'bearer', 'x-api-key', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 6.175, "output_cost_per_m": 25.65, "cache_read_cost_per_m": 1.043575, "source_currency": "CNY", "source_input_cost_per_m": 6.175, "source_output_cost_per_m": 25.65, "source_cache_read_cost_per_m": 1.043575, "source_pricing_note": "PackyCode 官方组 kimi-officially x0.95"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'kimi-k2-6'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- glm-5-3-flashx += aiboli (glm-5.3-flashx)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'glm-5.3-flashx', '/v1/chat/completions', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'glm-5-3-flashx'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- glm-5-3-flashx += packycode (glm-5.3-flashx)  [；PackyCode 分组 Key 未验证，待绑定正确分组 Key 后启用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai', 'glm-5.3-flashx', '/v1/chat/completions', 'https://www.packyapi.ai',
  COALESCE((SELECT api_key FROM model_routes WHERE provider='packycode' AND api_key<>'' LIMIT 1), ''), 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 2, "output_cost_per_m": 7.0, "cache_read_cost_per_m": 0.57, "source_currency": "CNY", "source_input_cost_per_m": 2, "source_output_cost_per_m": 7.0, "source_cache_read_cost_per_m": 0.57, "source_pricing_note": "PackyCode 官方组 zai-officially x1"}'::jsonb,
  100, 100, 600, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'glm-5-3-flashx'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- seedance-2-5 += aiboli (sd2.5-720p)  [aiboli 无 API Key，默认停用]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, 'aiboli（待启用）', 'aiboli', 'openai', 'sd2.5-720p', '/v1/video/generations', 'https://api.aiboli.xyz',
  '', 'bearer', 'Authorization', '{}'::jsonb, '"{}"'::jsonb, '"{}"'::jsonb, '{"currency": "CREDITS", "compute_per_cny": 1, "price_unit_normalized": true, "billing_type": "per_token", "input_cost_per_m": 0, "output_cost_per_m": 0, "source_currency": "CNY", "source_pricing_note": "aiboli 站内美元价，待提供 Key 后校准"}'::jsonb,
  100, 100, 1800, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'seedance-2-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url,
  api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- ============ 2. 删除五家之外的上游线路（备份后删除）============

DROP TABLE IF EXISTS model_routes_backup_200;
CREATE TABLE model_routes_backup_200 AS
SELECT r.* FROM model_routes r JOIN models m ON m.id = r.model_id
WHERE r.provider IN ('tokenhub','tencent_aigc','bailian','new_api','volcano','bigmodel')
  AND m.is_enabled = true
  AND m.code NOT IN ('audio_minimax_speech_28_hd', 'audio_minimax_music_26', 'kinfra-text-embedding-0-6b', 'kinfra-text-embedding-4b', 'kinfra-vl-embedding-2b', 'kinfra-vl-embedding-8b', 'qwen3-7-text-embedding', 'qwen3-7-text-embedding-flash', 'text-embedding-v2', 'text-embedding-v3', 'text-embedding-v4', 'qwen3-vl-embedding', 'tongyi-embedding-vision-flash-2026-03-06', 'tongyi-embedding-vision-plus-2026-03-06');

DELETE FROM model_routes r
USING model_routes_backup_200 b
WHERE r.id = b.id;

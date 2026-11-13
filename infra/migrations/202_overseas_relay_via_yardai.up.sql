-- 202_overseas_relay_via_yardai: 海外模型统一改走新加坡中转站 hub.yardai.cc（New API 网关，回源 OpenRouter）。
--
-- 1) 海外模型的 OpenRouter 直连线路物理删除（香港/大陆调不通），先备份到 model_routes_backup_202
-- 2) 为每个海外模型新建 new_api 中转线路（base=https://hub.yardai.cc），成本沿用 OpenRouter 官方价×6.75
--     api_key 留空，用户后续手动填入（填完即可用）。视频线路经中转站 video-bridge 转发 OpenRouter
-- 3) 国产模型的 OpenRouter 线路全部保留不动
-- 4) 价格监控 new_api 源改指 hub.yardai.cc（其 /api/pricing 公开）

-- ============ 1. 新增中转线路 ============

-- claude-fable-5 += 新加坡中转 (anthropic/claude-fable-5)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'anthropic/claude-fable-5', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.0000, "billing_type": "per_token", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 67.50000000000000000000, "output_cost_per_m": 337.50000000000000000000, "cache_read_cost_per_m": 6.75000000000000000000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 84.37500000000000000000, "audio_output_cost_per_m": 0.0000}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'claude-fable-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-fable-5-1 += 新加坡中转 (anthropic/claude-fable-5.1)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'anthropic/claude-fable-5.1', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.0000, "billing_type": "per_token", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 67.50000000000000000000, "output_cost_per_m": 337.50000000000000000000, "cache_read_cost_per_m": 1.68750000000000000000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 84.37500000000000000000, "audio_output_cost_per_m": 0.0000}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'claude-fable-5-1'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-opus-4-8 += 新加坡中转 (anthropic/claude-opus-4.8)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'anthropic/claude-opus-4.8', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.0000, "billing_type": "per_token", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 33.75000000000000000000, "output_cost_per_m": 168.75000000000000000000, "cache_read_cost_per_m": 3.37500000000000000000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 42.18750000000000000000, "audio_output_cost_per_m": 0.0000}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'claude-opus-4-8'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-opus-5-5 += 新加坡中转 (anthropic/claude-opus-5.5)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'anthropic/claude-opus-5.5', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.0000, "billing_type": "per_token", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 27.00000000000000000000, "output_cost_per_m": 135.00000000000000000000, "cache_read_cost_per_m": 1.35000000000000000000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 33.75000000000000000000, "audio_output_cost_per_m": 0.0000}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'claude-opus-5-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- claude-sonnet-5 += 新加坡中转 (anthropic/claude-sonnet-5)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'anthropic/claude-sonnet-5', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.0000, "billing_type": "per_token", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 13.50000000000000000000, "output_cost_per_m": 67.50000000000000000000, "cache_read_cost_per_m": 1.35000000000000000000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 16.87500000000000000000, "audio_output_cost_per_m": 0.0000}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'claude-sonnet-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-5-6-luna += 新加坡中转 (openai/gpt-5.6-luna)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'openai/gpt-5.6-luna', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "billing_type": "per_token", "usd_to_compute": 6.75, "compute_per_cny": 1, "source_currency": "USD", "input_cost_per_m": 1.35, "output_cost_per_m": 8.1, "price_unit_normalized": true, "source_input_cost_per_m": 0.2, "source_output_cost_per_m": 1.2}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-5-6-luna'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-5-6-sol += 新加坡中转 (openai/gpt-5.6-sol)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'openai/gpt-5.6-sol', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "billing_type": "per_token", "usd_to_compute": 6.75, "compute_per_cny": 1, "source_currency": "USD", "input_cost_per_m": 27.0, "output_cost_per_m": 135.0, "price_unit_normalized": true, "source_input_cost_per_m": 4.0, "source_output_cost_per_m": 20.0}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-5-6-sol'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-6-astra += 新加坡中转 (openai/gpt-6-astra)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'openai/gpt-6-astra', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "billing_type": "per_token", "usd_to_compute": 6.75, "compute_per_cny": 1, "source_currency": "USD", "input_cost_per_m": 67.5, "output_cost_per_m": 337.5, "price_unit_normalized": true, "source_input_cost_per_m": 10.0, "source_output_cost_per_m": 50.0}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-astra'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-6-luna += 新加坡中转 (openai/gpt-6-luna)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'openai/gpt-6-luna', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "billing_type": "per_token", "usd_to_compute": 6.75, "compute_per_cny": 1, "source_currency": "USD", "input_cost_per_m": 0.675, "output_cost_per_m": 3.375, "price_unit_normalized": true, "source_input_cost_per_m": 0.1, "source_output_cost_per_m": 0.5}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-luna'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-6-sol += 新加坡中转 (openai/gpt-6-sol)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'openai/gpt-6-sol', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "billing_type": "per_token", "usd_to_compute": 6.75, "compute_per_cny": 1, "source_currency": "USD", "input_cost_per_m": 13.5, "output_cost_per_m": 67.5, "price_unit_normalized": true, "source_input_cost_per_m": 2.0, "source_output_cost_per_m": 10.0}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-6-sol'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gemini-3-7-flash += 新加坡中转 (google/gemini-3-7-flash)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'google/gemini-3-7-flash', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.0000, "billing_type": "per_token", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 5.06250000000000000000, "output_cost_per_m": 25.31250000000000000000, "cache_read_cost_per_m": 0.506250000000000000000000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.281250000000000225000000, "audio_output_cost_per_m": 0.0000}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gemini-3-7-flash'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gemini-3-8-flash += 新加坡中转 (google/gemini-3-8-flash)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'google/gemini-3-8-flash', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.0000, "billing_type": "per_token", "usd_to_compute": 6.75, "compute_per_cny": 1, "source_currency": "USD", "input_cost_per_m": 5.568750, "output_cost_per_m": 27.84375, "unit_cost_by_size": {}, "unit_cost_by_variant": {}, "cache_read_cost_per_m": 0.5568750, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.0000, "audio_output_cost_per_m": 0.0000, "source_input_cost_per_m": 0.825, "unit_cost_by_resolution": {}, "source_output_cost_per_m": 4.125, "source_cache_read_cost_per_m": 0.0825}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gemini-3-8-flash'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- audio_openrouter_gpt_audio += 新加坡中转 (openai/gpt-audio)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'openai/gpt-audio', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{"upstream": {"adapter": "openrouter_audio"}}'::jsonb, '{"currency": "CREDITS", "billing_type": "per_token", "usd_to_compute": 6.75, "compute_per_cny": 1, "source_currency": "USD", "input_cost_per_m": 16.875, "output_cost_per_m": 67.5, "price_unit_normalized": true, "source_input_cost_per_m": 2.5, "source_output_cost_per_m": 10.0}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'audio_openrouter_gpt_audio'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- audio_openrouter_gpt_audio_mini += 新加坡中转 (openai/gpt-audio-mini)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'openai/gpt-audio-mini', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{"upstream": {"adapter": "openrouter_audio"}}'::jsonb, '{"currency": "CREDITS", "billing_type": "per_token", "usd_to_compute": 6.75, "compute_per_cny": 1, "source_currency": "USD", "input_cost_per_m": 4.05, "output_cost_per_m": 16.2, "price_unit_normalized": true, "source_input_cost_per_m": 0.6, "source_output_cost_per_m": 2.4}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'audio_openrouter_gpt_audio_mini'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- audio_openrouter_lyria_3_clip += 新加坡中转 (google/lyria-3-clip-preview)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'google/lyria-3-clip-preview', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{"upstream": {"adapter": "openrouter_audio"}}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.270000000000000000000000, "billing_type": "per_request", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 0.0000, "source_unit_cost": 0.04, "output_cost_per_m": 0.0000, "unit_cost_by_size": {}, "unit_cost_by_variant": {}, "cache_read_cost_per_m": 0.0000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.0000, "audio_output_cost_per_m": 0.0000, "unit_cost_by_resolution": {}}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'audio_openrouter_lyria_3_clip'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- audio_openrouter_lyria_3_pro += 新加坡中转 (google/lyria-3-pro-preview)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'google/lyria-3-pro-preview', '/chat/completions', 'https://hub.yardai.cc/v1',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{"upstream": {"adapter": "openrouter_audio"}}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.540000000000000000000000, "billing_type": "per_request", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 0.0000, "source_unit_cost": 0.08, "output_cost_per_m": 0.0000, "unit_cost_by_size": {}, "unit_cost_by_variant": {}, "cache_read_cost_per_m": 0.0000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.0000, "audio_output_cost_per_m": 0.0000, "unit_cost_by_resolution": {}}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'audio_openrouter_lyria_3_pro'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-image-2 += 新加坡中转 (openai/gpt-image-2)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'openai/gpt-image-2', '/v1/images/generations', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{"upstream": {"adapter": "openai_images", "edit_endpoint": "/v1/images/edits"}}'::jsonb, '{"currency": "CREDITS", "billing_type": "per_image", "unit_cost": 0.27, "compute_per_cny": 1, "usd_to_compute": 6.75, "source_currency": "USD", "source_unit_cost": 0.04, "source_pricing_note": "OpenRouter 1K 官方价×6.75 估算，待账单校准"}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-image-2'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-image-2-5-flare += 新加坡中转 (openai/gpt-image-2.5-flare)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'openai/gpt-image-2.5-flare', '/v1/images/generations', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{"upstream": {"adapter": "openai_images", "edit_endpoint": "/v1/images/edits"}}'::jsonb, '{"currency": "CREDITS", "billing_type": "per_image", "unit_cost": 0.0365, "compute_per_cny": 1, "usd_to_compute": 6.75, "source_currency": "USD", "source_unit_cost": 0.005407, "source_pricing_note": "OpenRouter 1K 官方价×6.75 估算，待账单校准"}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-image-2-5-flare'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gpt-image-2-5-sunburst += 新加坡中转 (openai/gpt-image-2.5-sunburst)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'openai/gpt-image-2.5-sunburst', '/v1/images/generations', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{"upstream": {"adapter": "openai_images", "edit_endpoint": "/v1/images/edits"}}'::jsonb, '{"currency": "CREDITS", "billing_type": "per_image", "unit_cost": 0.0365, "compute_per_cny": 1, "usd_to_compute": 6.75, "source_currency": "USD", "source_unit_cost": 0.005407, "source_pricing_note": "OpenRouter 1K 官方价×6.75 估算，待账单校准"}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gpt-image-2-5-sunburst'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gemini-2-5-flash-image += 新加坡中转 (google/gemini-2.5-flash-image)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'google/gemini-2.5-flash-image', '/v1/images/generations', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{"upstream": {"adapter": "openrouter_image", "reference_field": "image"}}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.000000000000000000000000, "billing_type": "per_token", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 2.02500000000000000000, "source_unit_cost": 0, "output_cost_per_m": 202.50000000000000000000, "unit_cost_by_size": {}, "unit_cost_by_variant": {}, "cache_read_cost_per_m": 0.0000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.0000, "audio_output_cost_per_m": 0.0000, "source_input_cost_per_m": 0.3, "unit_cost_by_resolution": {}, "source_output_cost_per_m": 30.0}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gemini-2-5-flash-image'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gemini-3-pro-image += 新加坡中转 (google/gemini-3-pro-image)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'google/gemini-3-pro-image', '/v1/images/generations', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{"upstream": {"adapter": "openrouter_image", "reference_field": "image"}}'::jsonb, '{"currency": "CREDITS", "billing_type": "per_token", "usd_to_compute": 6.75, "compute_per_cny": 1, "source_currency": "USD", "input_cost_per_m": 13.5, "output_cost_per_m": 81.0, "price_unit_normalized": true, "source_input_cost_per_m": 2.0, "source_output_cost_per_m": 12.0}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'gemini-3-pro-image'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- grok-imagine-image-2-0 += 新加坡中转 (x-ai/grok-imagine-image-2.0)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'x-ai/grok-imagine-image-2.0', '/v1/images/generations', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{"upstream": {"adapter": "openrouter_image", "reference_field": "image"}}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.270000000000000000000000, "billing_type": "per_image", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 0.0000, "source_unit_cost": 0.04, "output_cost_per_m": 0.0000, "unit_cost_by_size": {}, "unit_cost_by_variant": {"low_1k": 0.270000000000000000000000, "low_2k": 0.405000000000000000000000, "medium_1k": 0.405000000000000000000000, "medium_2k": 0.540000000000000000000000}, "cache_read_cost_per_m": 0.0000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.0000, "audio_output_cost_per_m": 0.0000, "unit_cost_by_resolution": {}}'::jsonb,
  120, 100, 1800, 0, true, 'healthy', 0
FROM models m WHERE m.code = 'grok-imagine-image-2-0'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- veo-3-1 += 新加坡中转 (google/veo-3.1)  [经中转站 video-bridge 转发 OpenRouter /videos]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'google/veo-3.1', '/v1/videos', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 2.70000000000000000000, "billing_type": "per_second", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 0.0000, "source_unit_cost": 0.4, "output_cost_per_m": 0.0000, "unit_cost_by_size": {}, "unit_cost_by_variant": {}, "cache_read_cost_per_m": 0.0000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.0000, "audio_output_cost_per_m": 0.0000, "unit_cost_by_resolution": {"4K": 4.05000000000000000000, "720p": 2.70000000000000000000, "1080p": 2.70000000000000000000}}'::jsonb,
  120, 100, 1800, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'veo-3-1'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- veo-3-1-fast += 新加坡中转 (google/veo-3.1-fast)  [经中转站 video-bridge 转发 OpenRouter /videos]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'google/veo-3.1-fast', '/v1/videos', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.675000000000000000000000, "billing_type": "per_second", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 0.0000, "source_unit_cost": 0.1, "output_cost_per_m": 0.0000, "unit_cost_by_size": {}, "unit_cost_by_variant": {}, "cache_read_cost_per_m": 0.0000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.0000, "audio_output_cost_per_m": 0.0000, "unit_cost_by_resolution": {"4K": 2.02500000000000000000, "720p": 0.675000000000000000000000, "1080p": 0.810000000000000000000000}}'::jsonb,
  120, 100, 1800, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'veo-3-1-fast'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- veo-3-1-lite += 新加坡中转 (google/veo-3.1-lite)  [经中转站 video-bridge 转发 OpenRouter /videos]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'google/veo-3.1-lite', '/v1/videos', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.337500000000000000000000, "billing_type": "per_second", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 0.0000, "source_unit_cost": 0.05, "output_cost_per_m": 0.0000, "unit_cost_by_size": {}, "unit_cost_by_variant": {}, "cache_read_cost_per_m": 0.0000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.0000, "audio_output_cost_per_m": 0.0000, "unit_cost_by_resolution": {"720p": 0.337500000000000000000000, "1080p": 0.540000000000000000000000}}'::jsonb,
  120, 100, 1800, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'veo-3-1-lite'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- gen-4-5 += 新加坡中转 (runway/gen-4.5)  [经中转站 video-bridge 转发 OpenRouter /videos]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'runway/gen-4.5', '/v1/videos', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.810000000000000000000000, "billing_type": "per_second", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 0.0000, "source_unit_cost": 0.12, "output_cost_per_m": 0.0000, "unit_cost_by_size": {}, "unit_cost_by_variant": {}, "cache_read_cost_per_m": 0.0000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.0000, "audio_output_cost_per_m": 0.0000, "unit_cost_by_resolution": {"720p": 0.810000000000000000000000}}'::jsonb,
  120, 100, 1800, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'gen-4-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- grok-imagine-video += 新加坡中转 (x-ai/grok-imagine-video)  [经中转站 video-bridge 转发 OpenRouter /videos]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'x-ai/grok-imagine-video', '/v1/videos', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.472500000000000000000000, "billing_type": "per_second", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 0.0000, "source_unit_cost": 0.07, "output_cost_per_m": 0.0000, "unit_cost_by_size": {}, "unit_cost_by_variant": {}, "cache_read_cost_per_m": 0.0000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.0000, "audio_output_cost_per_m": 0.0000, "unit_cost_by_resolution": {"480p": 0.337500000000000000000000, "720p": 0.472500000000000000000000}}'::jsonb,
  120, 100, 1800, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'grok-imagine-video'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- grok-imagine-video-1-5 += 新加坡中转 (x-ai/grok-imagine-video-1.5)  [经中转站 video-bridge 转发 OpenRouter /videos]
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled, health_status, consecutive_failures
) SELECT m.id, '新加坡中转 · yardai', 'new_api', 'openai', 'x-ai/grok-imagine-video-1.5', '/v1/videos', 'https://hub.yardai.cc',
  '', 'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"currency": "CREDITS", "unit_cost": 0.94500000000000000000, "billing_type": "per_second", "usd_to_compute": 6.75, "source_currency": "USD", "input_cost_per_m": 0.0000, "source_unit_cost": 0.14, "output_cost_per_m": 0.0000, "unit_cost_by_size": {}, "unit_cost_by_variant": {}, "cache_read_cost_per_m": 0.0000, "price_unit_normalized": true, "audio_input_cost_per_m": 0.0000, "cache_write_cost_per_m": 0.0000, "audio_output_cost_per_m": 0.0000, "unit_cost_by_resolution": {"480p": 0.540000000000000000000000, "720p": 0.94500000000000000000, "1080p": 1.68750000000000000000}}'::jsonb,
  120, 100, 1800, 0, false, 'healthy', 0
FROM models m WHERE m.code = 'grok-imagine-video-1-5'
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol, upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint, base_url = EXCLUDED.base_url, api_key = EXCLUDED.api_key,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header, headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params, runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight, timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries, is_enabled = EXCLUDED.is_enabled, health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- ============ 2. 删除海外模型的 OpenRouter 直连线路 ============

DROP TABLE IF EXISTS model_routes_backup_202;
CREATE TABLE model_routes_backup_202 AS
SELECT r.* FROM model_routes r
WHERE r.provider = 'openrouter'
  AND r.upstream_model ~ '^(anthropic|openai|google|x-ai|runway)/';

DELETE FROM model_routes r USING model_routes_backup_202 b WHERE r.id = b.id;

-- ============ 3. 价格监控：new_api 源改指新加坡中转站 ============

INSERT INTO upstream_providers (provider, display_name, source_type, price_endpoint, currency, recharge_rate, usd_cny_rate, is_enabled, config)
VALUES ('new_api', '新加坡中转 · yardai', 'new_api', 'https://hub.yardai.cc', 'CNY', 1, 6.75, true, '{"days":7,"group":"default"}'::jsonb)
ON CONFLICT (provider) DO UPDATE SET
  display_name = EXCLUDED.display_name, source_type = EXCLUDED.source_type,
  price_endpoint = EXCLUDED.price_endpoint, currency = EXCLUDED.currency,
  recharge_rate = EXCLUDED.recharge_rate, usd_cny_rate = EXCLUDED.usd_cny_rate,
  config = EXCLUDED.config, is_enabled = EXCLUDED.is_enabled, updated_at = now();

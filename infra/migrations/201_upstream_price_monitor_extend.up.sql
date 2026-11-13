-- 201_upstream_price_monitor_extend: 上游价格监控补齐七牛云 / aiboli 两个同步源及模型映射。
-- 依赖 200_align_five_upstreams 与 Go 侧新增的 qiniu_market 抓取器（fetchQiniuPrices）。
-- aiboli 走 new_api 源类型，需在 config.api_key 填入其 access token 后同步才会成功（否则 401）。

-- 1) 同步源配置
INSERT INTO upstream_providers (provider, display_name, source_type, price_endpoint, currency, recharge_rate, usd_cny_rate, is_enabled, config)
VALUES
  ('qiniu', '七牛云 Modelink', 'qiniu_market', 'https://api.qnaigc.com/v1', 'CNY', 1, 6.75, true, '{}'::jsonb),
  ('aiboli', 'aiboli', 'new_api', 'https://api.aiboli.xyz', 'CNY', 1, 6.75, true, '{"api_key":""}'::jsonb)
ON CONFLICT (provider) DO UPDATE SET
  display_name = EXCLUDED.display_name, source_type = EXCLUDED.source_type,
  price_endpoint = EXCLUDED.price_endpoint, currency = EXCLUDED.currency,
  config = EXCLUDED.config, is_enabled = EXCLUDED.is_enabled, updated_at = now();

-- 2) 模型映射（按 model.code 关联，跨环境安全）
INSERT INTO upstream_model_mappings (provider, upstream_model, model_id, match_method, confidence, is_confirmed)
SELECT v.provider, v.upstream_model, m.id, 'alias', 0.8, false
FROM (VALUES
  ('qiniu', 'moonshotai/kimi-k2.6', 'kimi-k2-6'),
  ('qiniu', 'deepseek/deepseek-v4-flash-20260731', 'deepseek-v4-1-flash-1017'),
  ('qiniu', 'qwen/qwen3.8-max', 'qwen3-8-max-0902'),
  ('qiniu', 'minimax/minimax-h3', 'hailuo-3'),
  ('qiniu', 'deepseek/deepseek-v4-pro-0813', 'deepseek-v4-pro-0813'),
  ('qiniu', 'qwen/qwen3.8-flash-next', 'qwen3-8-flash'),
  ('qiniu', 'z-ai/glm-5.3-flash', 'glm-5-3-flash'),
  ('qiniu', 'minimax/minimax-h3-max', 'hailuo-3-max'),
  ('qiniu', 'deepseek/deepseek-v4.1-flash', 'deepseek-v4-1-flash-1017'),
  ('qiniu', 'bytedance/doubao-seedream-5.0-pro', 'seedream-5-0-pro'),
  ('qiniu', 'qwen/qwen3.8-omni-flash', 'qwen3-8-omni-flash'),
  ('qiniu', 'deepseek/deepseek-v4.1-flash-1017', 'deepseek-v4-1-flash-1017'),
  ('qiniu', 'kling/kling-video-4.0', 'kling-video-4-0'),
  ('qiniu', 'kling-v2-6', 'kling-v2-6'),
  ('qiniu', 'kling-video-o1', 'kling-video-o1'),
  ('qiniu', 'kling-v3', 'kling-v3'),
  ('qiniu', 'qwen/qwen3.6-plus', 'qwen3-6-plus'),
  ('qiniu', 'z-ai/glm-5', 'glm-5'),
  ('qiniu', 'vidu/viduq3', 'viduq3'),
  ('qiniu', 'viduq3-pro', 'viduq3-pro'),
  ('qiniu', 'viduq3-turbo', 'viduq3-turbo'),
  ('aiboli', 'glm-5.3-flashx', 'glm-5-3-flashx'),
  ('aiboli', 'deepseek-flash', 'deepseek-v4-1-flash-1017'),
  ('aiboli', 'deepseek-v4.1-flash', 'deepseek-v4-1-flash-1017'),
  ('aiboli', 'deepseek-v4.1-flash-1017', 'deepseek-v4-1-flash-1017'),
  ('aiboli', 'gpt-6-luna', 'gpt-6-luna'),
  ('aiboli', 'luna-6', 'gpt-6-luna'),
  ('aiboli', 'sol-6', 'gpt-6-sol'),
  ('aiboli', 'claude-opus-5-5', 'claude-opus-5-5'),
  ('aiboli', 'astra-6', 'gpt-6-astra'),
  ('aiboli', 'gpt-6-astra', 'gpt-6-astra'),
  ('aiboli', 'gemini-3.8-flash-high', 'gemini-3-8-flash'),
  ('aiboli', 'jiuyue-3.8-flash-high', 'gemini-3-8-flash'),
  ('aiboli', 'glm-5.3-flash', 'glm-5-3-flash'),
  ('aiboli', 'gpt-5.6-luna', 'gpt-5-6-luna'),
  ('aiboli', 'gpt-5.6-sol', 'gpt-5-6-sol'),
  ('aiboli', 'sol-5.6', 'gpt-5-6-sol'),
  ('aiboli', 'claude-sonnet-5', 'claude-sonnet-5'),
  ('aiboli', 'gemini-3-pro-image-2k', 'gemini-3-pro-image'),
  ('aiboli', 'gemini-3-pro-image-4k', 'gemini-3-pro-image'),
  ('aiboli', 'gemini-3-pro-image-preview', 'gemini-3-pro-image'),
  ('aiboli', 'claude-fable-5', 'claude-fable-5'),
  ('aiboli', 'claude-opus-4-8', 'claude-opus-4-8'),
  ('aiboli', 'gpt-image-2.5-flare', 'gpt-image-2-5-flare'),
  ('aiboli', 'gpt-image-2', 'gpt-image-2'),
  ('aiboli', 'gpt-image-2.5-sunburst', 'gpt-image-2-5-sunburst'),
  ('aiboli', 'sd2.0-933-pro', 'seedance-2-0'),
  ('aiboli', 'seedance2.0', 'seedance-2-0'),
  ('aiboli', 'sd2.5-720p', 'seedance-2-5'),
  ('aiboli', 'seedance2.5', 'seedance-2-5'),
  ('aiboli', 'wan3.0', 'wan-3-0')
) AS v(provider, upstream_model, code)
JOIN models m ON m.code = v.code
ON CONFLICT (provider, upstream_model) DO NOTHING;
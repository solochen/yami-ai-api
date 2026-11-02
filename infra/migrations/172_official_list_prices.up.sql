-- 全站官方挂牌价：所有模型的用户售价统一锁定为厂商官方价（海外=官方美元牌价×6.7，国产=厂商人民币牌价）。
-- 平台佣金率已设 0，故页面展示价 = 本规则数值 = 扣费价。上游成本与本规则解耦，
-- 平台利润 = 官方价 − 实际线路成本，后续更换更便宜上游时用户价不变。
-- 国产牌价来源：百炼价格总表(qwen/wan/happyhorse/glm/deepseek)、火山方舟价格页(seedance/seedream)、
-- 可灵 API 定价(kling, 1积分≈1元)、MiniMax(海螺H3=百炼同步价；2.3/H3-Max 按美元牌价×6.7, 待核)。
-- qwen3.8-max-prime 来自第三方价格追踪(llmabacus)，建议上线前在百炼控制台复核。
-- 回滚：见 172_official_list_prices.down.sql（从备份表恢复原 user_price_rule）。

CREATE TABLE IF NOT EXISTS official_list_price_backup_172 AS
SELECT r.id AS route_id, m.code AS model_code, r.user_price_rule AS old_rule, now() AS backed_up_at
FROM model_routes r JOIN models m ON m.id = r.model_id
WHERE m.code = ANY(ARRAY['audio_openrouter_gpt_audio'::text,'audio_openrouter_gpt_audio_mini'::text,'audio_openrouter_lyria_3_clip'::text,'audio_openrouter_lyria_3_pro'::text,'claude-fable-5'::text,'claude-fable-5-1'::text,'claude-opus-4-8'::text,'claude-opus-5-5'::text,'claude-sonnet-5'::text,'gemini-2-5-flash-image'::text,'gemini-3-7-flash'::text,'gemini-3-8-flash'::text,'gemini-3-pro-image'::text,'gen-4-5'::text,'glm-5-3-flash'::text,'glm-5-3-flashx'::text,'glm-5-3-prime'::text,'gpt-5-6-luna'::text,'gpt-5-6-sol'::text,'gpt-6-astra'::text,'gpt-6-luna'::text,'gpt-6-sol'::text,'gpt-image-2'::text,'gpt-image-2-5-flare'::text,'gpt-image-2-5-sunburst'::text,'grok-imagine-image-2-0'::text,'grok-imagine-video'::text,'grok-imagine-video-1-5'::text,'hailuo-2-3'::text,'hailuo-3'::text,'hailuo-3-max'::text,'happyhorse-1-0'::text,'happyhorse-1-1'::text,'kling-v3-0-pro'::text,'kling-v3-0-std'::text,'kling-video-o1'::text,'qwen-image-3'::text,'qwen-image-3-pro'::text,'qwen3-8-27b'::text,'qwen3-8-flash'::text,'qwen3-8-max-0902'::text,'qwen3-8-max-prime'::text,'qwen3-8-omni-flash'::text,'seedance-1-5-pro'::text,'seedance-2-0'::text,'seedance-2-0-fast'::text,'seedance-2-0-mini'::text,'seedance-2-5'::text,'seedream-4-5'::text,'seedream-5-0-lite'::text,'seedream-5-0-pro'::text,'veo-3-1'::text,'veo-3-1-fast'::text,'veo-3-1-lite'::text,'wan-2-6'::text,'wan-2-7'::text,'wan-3-0'::text,'wan-3-0-prime'::text]);

UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 16.75, "output_cost_per_m": 67.0, "audio_input_cost_per_m": 214.4, "audio_output_cost_per_m": 428.8}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'audio_openrouter_gpt_audio';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 4.02, "output_cost_per_m": 16.08, "audio_input_cost_per_m": 4.02, "audio_output_cost_per_m": 16.08}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'audio_openrouter_gpt_audio_mini';
UPDATE model_routes r SET user_price_rule = $json${"unit_cost": 0.268, "billing_type": "per_request"}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'audio_openrouter_lyria_3_clip';
UPDATE model_routes r SET user_price_rule = $json${"unit_cost": 0.536, "billing_type": "per_request"}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'audio_openrouter_lyria_3_pro';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 67.0, "output_cost_per_m": 335.0, "cache_read_cost_per_m": 6.7, "cache_write_cost_per_m": 83.75}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'claude-fable-5';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 67.0, "output_cost_per_m": 335.0, "cache_read_cost_per_m": 1.675, "cache_write_cost_per_m": 83.75}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'claude-fable-5-1';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 33.5, "output_cost_per_m": 167.5, "cache_read_cost_per_m": 3.35, "cache_write_cost_per_m": 41.875}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'claude-opus-4-8';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 26.8, "output_cost_per_m": 134.0, "cache_read_cost_per_m": 1.34, "cache_write_cost_per_m": 33.5}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'claude-opus-5-5';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 13.4, "output_cost_per_m": 67.0, "cache_read_cost_per_m": 1.34, "cache_write_cost_per_m": 16.75}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'claude-sonnet-5';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 2.01, "output_cost_per_m": 201.0}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gemini-2-5-flash-image';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 5.025, "output_cost_per_m": 25.125, "cache_read_cost_per_m": 0.5025, "cache_write_cost_per_m": 0.2792}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gemini-3-7-flash';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 5.5275, "output_cost_per_m": 27.6375, "cache_read_cost_per_m": 0.5527}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gemini-3-8-flash';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 13.4, "output_cost_per_m": 804.0}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gemini-3-pro-image';
UPDATE model_routes r SET user_price_rule = $json${"unit_cost": 0.804, "billing_type": "per_second", "unit_cost_by_resolution": {"720p": 0.804}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gen-4-5';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 0.8, "output_cost_per_m": 2.8, "cache_read_cost_per_m": 0.23}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'glm-5-3-flash';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 2, "output_cost_per_m": 7, "cache_read_cost_per_m": 0.57}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'glm-5-3-flashx';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 16, "output_cost_per_m": 56, "cache_read_cost_per_m": 3.75}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'glm-5-3-prime';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 1.34, "output_cost_per_m": 8.04, "cache_read_cost_per_m": 0.134, "cache_write_cost_per_m": 1.675}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gpt-5-6-luna';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 26.8, "output_cost_per_m": 134.0, "cache_read_cost_per_m": 2.68}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gpt-5-6-sol';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 67.0, "output_cost_per_m": 335.0, "cache_read_cost_per_m": 6.7, "cache_write_cost_per_m": 83.75}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gpt-6-astra';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 0.67, "output_cost_per_m": 3.35, "cache_read_cost_per_m": 0.067, "cache_write_cost_per_m": 0.8375}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gpt-6-luna';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 13.4, "output_cost_per_m": 67.0, "cache_read_cost_per_m": 1.34, "cache_write_cost_per_m": 16.75}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gpt-6-sol';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 53.6, "output_cost_per_m": 201.0}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gpt-image-2';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 53.6, "output_cost_per_m": 201.0}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gpt-image-2-5-flare';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 53.6, "output_cost_per_m": 201.0}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'gpt-image-2-5-sunburst';
UPDATE model_routes r SET user_price_rule = $json${"unit_cost": 0.268, "billing_type": "per_image", "unit_cost_by_variant": {"low_1k": 0.268, "low_2k": 0.402, "medium_1k": 0.402, "medium_2k": 0.536}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'grok-imagine-image-2-0';
UPDATE model_routes r SET user_price_rule = $json${"unit_cost": 0.469, "billing_type": "per_second", "unit_cost_by_resolution": {"480p": 0.335, "720p": 0.469}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'grok-imagine-video';
UPDATE model_routes r SET user_price_rule = $json${"unit_cost": 0.938, "billing_type": "per_second", "unit_cost_by_resolution": {"480p": 0.536, "720p": 0.938, "1080p": 1.675}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'grok-imagine-video-1-5';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.547, "unit_cost_by_resolution": {"1080p": 0.547}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'hailuo-2-3';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.5, "unit_cost_by_resolution": {"768p": 0.5, "2K": 0.8}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'hailuo-3';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.536, "unit_cost_by_resolution": {"480p": 0.336, "768p": 0.536}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'hailuo-3-max';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.9, "unit_cost_by_resolution": {"720p": 0.9, "1080p": 1.6}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'happyhorse-1-0';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.9, "unit_cost_by_resolution": {"480p": 0.45, "720p": 0.9, "1080p": 1.2}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'happyhorse-1-1';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.75, "unit_cost_by_resolution": {"720p": 0.75}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'kling-v3-0-pro';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.6, "unit_cost_by_resolution": {"720p": 0.6, "1080p": 0.8}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'kling-v3-0-std';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.75, "unit_cost_by_resolution": {"720p": 0.75}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'kling-video-o1';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_image", "unit_cost": 0.2, "unit_cost_by_size": {"1K": 0.2, "2K": 0.2}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'qwen-image-3';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_image", "unit_cost": 0.27, "unit_cost_by_size": {"1K": 0.27, "2K": 0.52}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'qwen-image-3-pro';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 3, "output_cost_per_m": 12, "cache_read_cost_per_m": 0.3}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'qwen3-8-27b';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 0.8, "output_cost_per_m": 2.7, "cache_read_cost_per_m": 0.1}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'qwen3-8-flash';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 12, "output_cost_per_m": 36, "cache_read_cost_per_m": 1.87}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'qwen3-8-max-0902';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 24, "output_cost_per_m": 72, "cache_read_cost_per_m": 3.74}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'qwen3-8-max-prime';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_token", "input_cost_per_m": 0.8, "output_cost_per_m": 2.7, "cache_read_cost_per_m": 0.1}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'qwen3-8-omni-flash';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.347, "unit_cost_by_resolution": {"480p": 0.155, "720p": 0.347, "1080p": 0.782}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'seedance-1-5-pro';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.99, "unit_cost_by_resolution": {"480p": 0.46, "720p": 0.99, "1080p": 2.48}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'seedance-2-0';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.8, "unit_cost_by_resolution": {"480p": 0.37, "720p": 0.8}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'seedance-2-0-fast';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.5, "unit_cost_by_resolution": {"480p": 0.23, "720p": 0.5}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'seedance-2-0-mini';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 1.51, "unit_cost_by_resolution": {"480p": 0.67, "720p": 1.51, "1080p": 3.74}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'seedance-2-5';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_image", "unit_cost": 0.25, "unit_cost_by_size": {"1K": 0.25, "2K": 0.25}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'seedream-4-5';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_image", "unit_cost": 0.22, "unit_cost_by_size": {"2K": 0.22, "4K": 0.22}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'seedream-5-0-lite';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_image", "unit_cost": 0.3, "unit_cost_by_size": {"1K": 0.3, "2K": 0.6}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'seedream-5-0-pro';
UPDATE model_routes r SET user_price_rule = $json${"unit_cost": 2.68, "billing_type": "per_second", "unit_cost_by_resolution": {"4K": 4.02, "720p": 2.68, "1080p": 2.68}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'veo-3-1';
UPDATE model_routes r SET user_price_rule = $json${"unit_cost": 0.67, "billing_type": "per_second", "unit_cost_by_resolution": {"4K": 2.01, "720p": 0.67, "1080p": 0.804}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'veo-3-1-fast';
UPDATE model_routes r SET user_price_rule = $json${"unit_cost": 0.335, "billing_type": "per_second", "unit_cost_by_resolution": {"720p": 0.335, "1080p": 0.536}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'veo-3-1-lite';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.6, "unit_cost_by_resolution": {"720p": 0.6, "1080p": 1.0}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'wan-2-6';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.6, "unit_cost_by_resolution": {"720p": 0.6, "1080p": 1.0}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'wan-2-7';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.6, "unit_cost_by_resolution": {"480p": 0.3, "720p": 0.6, "1080p": 1.2}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'wan-3-0';
UPDATE model_routes r SET user_price_rule = $json${"billing_type": "per_second", "unit_cost": 0.9, "unit_cost_by_resolution": {"480p": 0.45, "720p": 0.9, "1080p": 1.8}}$json$::jsonb, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'wan-3-0-prime';

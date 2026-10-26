-- 开放音频模型：OpenRouter Lyria 3 Pro + 腾讯云 TokenHub 语音合成/音乐生成。
--
-- 背景：这 3 个音频模型此前没有可计费线路，被前台模型列表自动跳过（用户看不到）。
-- 现在按「只用已有账号」的原则接上：
--   Lyria 3 Pro            → OpenRouter（线路已存在，只需启用 + 复用 OR 密钥）
--   MiniMax Speech 2.8 HD  → 腾讯云 TokenHub（¥3.5/万字符）
--   MiniMax Music 3.0      → 腾讯云 TokenHub（¥1/首）
--
-- 计费口径：
--   语音合成按字符计费（TokenHub ¥3.5/万字符 = ¥350/百万字符），任务创建时会注入
--   精确字符数到 _estimated_input_tokens，worker 结算时按同一数值折算；
--   音乐按次计费（¥1/首）。售价 = 成本 ÷ (1 − 8% 佣金率)。
--   TokenHub 音频接口无公开文档，请求体按 MiniMax 原生结构构造（见 worker 的
--   tencent_audio 适配器），密钥需在后台替换为真实 TokenHub 密钥。

-- ========= 1. 启用 OpenRouter Lyria 3 Pro（复用同 provider 的密钥）=========
UPDATE model_routes r
SET is_enabled = true,
    api_key = COALESCE(NULLIF(r.api_key, ''), (
      SELECT clip.api_key FROM model_routes clip
      JOIN models cm ON cm.id = clip.model_id
      WHERE cm.code = 'audio_openrouter_lyria_3_clip' AND clip.provider = 'openrouter'
      LIMIT 1
    )),
    updated_at = now()
FROM models m
WHERE m.id = r.model_id AND m.code = 'audio_openrouter_lyria_3_pro';

-- ========= 2. 停用演示模型 image_fast_v1（上游仍是 dall-e-3 占位价）=========
UPDATE models SET is_enabled = false, updated_at = now() WHERE code = 'image_fast_v1';

-- ========= 3. TokenHub 语音合成线路（按字符计费）=========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, '腾讯云 TokenHub', 'tokenhub', 'openai', 'minimax-speech-2.8-hd', '/v1/t2a_v2',
       'https://tokenhub.tencentmaas.com/v1',
       'REPLACE_ME_TENCENT_TOKENHUB_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb,
       '{"upstream":{"adapter":"tencent_audio"}}'::jsonb,
       '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":350,"source_input_cost_per_m":350,"input_unit":"character"}'::jsonb,
       20, 100, 600, 0, true
FROM models m
WHERE m.code = 'audio_minimax_speech_28_hd'
  AND NOT EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.provider = 'tokenhub');

-- ========= 4. TokenHub 音乐生成线路（按次计费）=========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, '腾讯云 TokenHub', 'tokenhub', 'openai', 'minimax-music-v3.0', '/v1/music_generation',
       'https://tokenhub.tencentmaas.com/v1',
       'REPLACE_ME_TENCENT_TOKENHUB_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb,
       '{"upstream":{"adapter":"tencent_audio"}}'::jsonb,
       '{"currency":"CREDITS","billing_type":"per_request","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","unit_cost":1,"source_unit_cost":1}'::jsonb,
       20, 100, 600, 0, true
FROM models m
WHERE m.code = 'audio_minimax_music_26'
  AND NOT EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.provider = 'tokenhub');

-- ========= 5. 音乐模型补「歌词」输入框 =========
UPDATE models
SET input_schema = jsonb_set(
      input_schema,
      '{properties,lyrics}',
      '{"type":"string","title":"歌词","description":"留空则由上游按提示词生成（可开启歌词优化）","x-order":0,"x-widget":"textarea","x-placement":"top"}'::jsonb,
      true
    ),
    updated_at = now()
WHERE code = 'audio_minimax_music_26'
  AND NOT (input_schema->'properties' ? 'lyrics');

-- ========= 6. 模型售价展示值同步（成本 ÷ 0.92）=========
UPDATE models
SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":380.434783}'::jsonb,
    updated_at = now()
WHERE code = 'audio_minimax_speech_28_hd';

UPDATE models
SET price_rule = '{"currency":"CREDITS","billing_type":"per_request","price_unit_normalized":true,"unit_price":1.086957}'::jsonb,
    updated_at = now()
WHERE code = 'audio_minimax_music_26';

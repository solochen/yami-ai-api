-- OpenRouter official audio models. API keys are intentionally left blank and
-- routes are disabled by default; administrators can fill the key and enable
-- each route after validating their OpenRouter account.
WITH billing AS (
  SELECT COALESCE(
    (SELECT NULLIF((value #>> '{}')::numeric, 0)
       FROM system_configs
      WHERE key = 'payment_compute_rate'),
    7.2::numeric
  ) AS usd_to_compute
), specs AS (
  SELECT * FROM (VALUES
    (
      'audio_openrouter_lyria_3_pro',
      'OpenRouter · Lyria 3 Pro',
      'google/lyria-3-pro-preview',
      'Google Lyria 3 Pro Preview，OpenRouter 官方音频输出模型。适合生成完整歌曲，支持文本提示词和图片参考。官方价格 $0.08 / song。',
      jsonb_build_array('音频','音乐','OpenRouter','Lyria','Google'),
      '{"type":"object","properties":{"format":{"type":"string","title":"输出格式","enum":["mp3","wav"],"enumLabels":{"mp3":"MP3","wav":"WAV"},"default":"mp3","x-order":1,"x-widget":"option_menu","x-icon":"format","x-placement":"top","x-highlight":true},"temperature":{"type":"number","title":"创造性","enum":[0.7,0.9,1,1.2],"enumLabels":{"0.7":"稳妥","0.9":"均衡","1":"标准","1.2":"更发散"},"default":1,"x-order":2,"x-widget":"option_menu","x-icon":"sparkles"},"top_p":{"type":"number","title":"Top P","enum":[0.8,0.9,1],"default":1,"x-order":3,"x-widget":"option_menu","x-icon":"compass"},"seed":{"type":"integer","title":"Seed","description":"可选，固定后更便于复现风格","x-order":4}}}'::jsonb,
      '{"format":"mp3","temperature":1,"top_p":1}'::jsonb,
      '{"audio":{"input_layout":"single","prompt_hint":"描述歌曲主题、曲风、情绪、乐器、节奏和歌词方向；可上传一张图片作为音乐灵感参考。","prompt_required":true,"billing_hint":"estimated","show_channel":false,"show_web_search":false,"show_upload":false,"show_image_upload":true,"max_reference_images":1,"count_options":[1],"count_allow_custom":false,"count_max":1},"upstream":{"adapter":"openrouter_audio","include":["format","temperature","top_p","seed","reference_images"],"map":{},"request_timeout_sec":900},"capabilities":{"web_search":false,"deep_think":false}}'::jsonb,
      0.08::numeric,
      10,
      'per_request'
    ),
    (
      'audio_openrouter_lyria_3_clip',
      'OpenRouter · Lyria 3 Clip',
      'google/lyria-3-clip-preview',
      'Google Lyria 3 Clip Preview，OpenRouter 官方音频输出模型。适合 30 秒音乐片段、广告钩子和短视频 BGM，支持文本提示词和图片参考。官方价格 $0.04 / 30 秒 clip。',
      jsonb_build_array('音频','音乐','OpenRouter','Lyria','Google','Clip'),
      '{"type":"object","properties":{"format":{"type":"string","title":"输出格式","enum":["mp3","wav"],"enumLabels":{"mp3":"MP3","wav":"WAV"},"default":"mp3","x-order":1,"x-widget":"option_menu","x-icon":"format","x-placement":"top","x-highlight":true},"temperature":{"type":"number","title":"创造性","enum":[0.7,0.9,1,1.2],"enumLabels":{"0.7":"稳妥","0.9":"均衡","1":"标准","1.2":"更发散"},"default":1,"x-order":2,"x-widget":"option_menu","x-icon":"sparkles"},"top_p":{"type":"number","title":"Top P","enum":[0.8,0.9,1],"default":1,"x-order":3,"x-widget":"option_menu","x-icon":"compass"},"seed":{"type":"integer","title":"Seed","description":"可选，固定后更便于复现风格","x-order":4}}}'::jsonb,
      '{"format":"mp3","temperature":1,"top_p":1}'::jsonb,
      '{"audio":{"input_layout":"single","prompt_hint":"描述 30 秒音乐片段的风格、氛围、速度、乐器和使用场景；可上传一张图片作为灵感参考。","prompt_required":true,"billing_hint":"estimated","show_channel":false,"show_web_search":false,"show_upload":false,"show_image_upload":true,"max_reference_images":1,"count_options":[1],"count_allow_custom":false,"count_max":1},"upstream":{"adapter":"openrouter_audio","include":["format","temperature","top_p","seed","reference_images"],"map":{},"request_timeout_sec":900},"capabilities":{"web_search":false,"deep_think":false}}'::jsonb,
      0.04::numeric,
      11,
      'per_request'
    ),
    (
      'audio_openrouter_gpt_audio',
      'OpenRouter · GPT Audio',
      'openai/gpt-audio',
      'OpenAI GPT Audio through OpenRouter。支持文本与参考音频输入，同时返回文本和音频；适合对话配音、声音改写、音频问答和多模态语音生成。',
      jsonb_build_array('音频','TTS','OpenRouter','OpenAI','GPT Audio'),
      '{"type":"object","properties":{"voice":{"type":"string","title":"音色","enum":["alloy","ash","ballad","coral","echo","fable","nova","onyx","sage","shimmer","verse"],"enumLabels":{"alloy":"Alloy","ash":"Ash","ballad":"Ballad","coral":"Coral","echo":"Echo","fable":"Fable","nova":"Nova","onyx":"Onyx","sage":"Sage","shimmer":"Shimmer","verse":"Verse"},"default":"alloy","x-order":1,"x-widget":"option_menu","x-icon":"voice","x-placement":"top","x-highlight":true},"format":{"type":"string","title":"输出格式","enum":["mp3","wav","flac","opus","pcm16"],"enumLabels":{"mp3":"MP3","wav":"WAV","flac":"FLAC","opus":"Opus","pcm16":"PCM16"},"default":"mp3","x-order":2,"x-widget":"option_menu","x-icon":"format","x-placement":"top","x-highlight":true},"temperature":{"type":"number","title":"创造性","enum":[0.2,0.5,0.8,1],"enumLabels":{"0.2":"稳定","0.5":"均衡","0.8":"自然","1":"更自由"},"default":0.8,"x-order":3,"x-widget":"option_menu","x-icon":"sparkles"},"top_p":{"type":"number","title":"Top P","enum":[0.8,0.9,1],"default":1,"x-order":4,"x-widget":"option_menu","x-icon":"compass"},"max_tokens":{"type":"integer","title":"最大文本 Token","enum":[256,512,1024,2048],"default":1024,"x-order":5,"x-widget":"option_menu","x-icon":"text"}}}'::jsonb,
      '{"voice":"alloy","format":"mp3","temperature":0.8,"top_p":1,"max_tokens":1024}'::jsonb,
      '{"audio":{"input_layout":"single","prompt_hint":"输入要生成或处理的语音内容；可上传参考音频，让模型结合音频内容和文本指令输出语音。","prompt_required":true,"billing_hint":"per_token","show_channel":false,"show_web_search":false,"show_upload":true,"show_image_upload":false,"count_options":[1],"count_allow_custom":false,"count_max":1},"upstream":{"adapter":"openrouter_audio","include":["voice","format","temperature","top_p","max_tokens","reference_audio"],"map":{},"request_timeout_sec":600},"capabilities":{"web_search":false,"deep_think":false}}'::jsonb,
      NULL::numeric,
      12,
      'per_token'
    ),
    (
      'audio_openrouter_gpt_audio_mini',
      'OpenRouter · GPT Audio Mini',
      'openai/gpt-audio-mini',
      'OpenAI GPT Audio Mini through OpenRouter。成本更低，支持文本与参考音频输入、文本与音频输出，适合批量语音生成和轻量音频理解。',
      jsonb_build_array('音频','TTS','OpenRouter','OpenAI','GPT Audio Mini'),
      '{"type":"object","properties":{"voice":{"type":"string","title":"音色","enum":["alloy","ash","ballad","coral","echo","fable","nova","onyx","sage","shimmer","verse"],"enumLabels":{"alloy":"Alloy","ash":"Ash","ballad":"Ballad","coral":"Coral","echo":"Echo","fable":"Fable","nova":"Nova","onyx":"Onyx","sage":"Sage","shimmer":"Shimmer","verse":"Verse"},"default":"alloy","x-order":1,"x-widget":"option_menu","x-icon":"voice","x-placement":"top","x-highlight":true},"format":{"type":"string","title":"输出格式","enum":["mp3","wav","flac","opus","pcm16"],"enumLabels":{"mp3":"MP3","wav":"WAV","flac":"FLAC","opus":"Opus","pcm16":"PCM16"},"default":"mp3","x-order":2,"x-widget":"option_menu","x-icon":"format","x-placement":"top","x-highlight":true},"temperature":{"type":"number","title":"创造性","enum":[0.2,0.5,0.8,1],"enumLabels":{"0.2":"稳定","0.5":"均衡","0.8":"自然","1":"更自由"},"default":0.8,"x-order":3,"x-widget":"option_menu","x-icon":"sparkles"},"top_p":{"type":"number","title":"Top P","enum":[0.8,0.9,1],"default":1,"x-order":4,"x-widget":"option_menu","x-icon":"compass"},"max_tokens":{"type":"integer","title":"最大文本 Token","enum":[256,512,1024,2048],"default":1024,"x-order":5,"x-widget":"option_menu","x-icon":"text"}}}'::jsonb,
      '{"voice":"alloy","format":"mp3","temperature":0.8,"top_p":1,"max_tokens":1024}'::jsonb,
      '{"audio":{"input_layout":"single","prompt_hint":"输入要生成或处理的语音内容；可上传参考音频，让模型结合音频内容和文本指令输出语音。","prompt_required":true,"billing_hint":"per_token","show_channel":false,"show_web_search":false,"show_upload":true,"show_image_upload":false,"count_options":[1],"count_allow_custom":false,"count_max":1},"upstream":{"adapter":"openrouter_audio","include":["voice","format","temperature","top_p","max_tokens","reference_audio"],"map":{},"request_timeout_sec":600},"capabilities":{"web_search":false,"deep_think":false}}'::jsonb,
      NULL::numeric,
      13,
      'per_token'
    )
  ) AS t(code, display_name, upstream_model, description, tags, input_schema, default_params, runtime_rule, unit_usd, sort_order, billing_type)
)
INSERT INTO models (
  code, display_name, new_api_model, new_api_endpoint, request_mode, category,
  description, tags, input_schema, default_params, new_api_extra_params,
  price_rule, runtime_rule, is_enabled, sort_order
)
SELECT
  specs.code,
  specs.display_name,
  specs.upstream_model,
  '/chat/completions',
  'audio',
  'audio',
  specs.description,
  specs.tags,
  specs.input_schema,
  specs.default_params,
  jsonb_build_object('connection', jsonb_build_object(
    'provider', 'openrouter',
    'protocol', 'openai_compatible',
    'base_url', 'https://openrouter.ai/api/v1',
    'api_key', '',
    'auth_type', 'none',
    'api_key_header', 'Authorization',
    'models_endpoint', '/models'
  )),
  CASE specs.code
    WHEN 'audio_openrouter_lyria_3_pro' THEN jsonb_build_object(
      'billing_type', 'per_request', 'unit_price', specs.unit_usd * billing.usd_to_compute,
      'usd_to_compute', billing.usd_to_compute, 'source_currency', 'USD', 'source_unit_usd', specs.unit_usd,
      'source_price_note', '$0.08 / song'
    )
    WHEN 'audio_openrouter_lyria_3_clip' THEN jsonb_build_object(
      'billing_type', 'per_request', 'unit_price', specs.unit_usd * billing.usd_to_compute,
      'usd_to_compute', billing.usd_to_compute, 'source_currency', 'USD', 'source_unit_usd', specs.unit_usd,
      'source_price_note', '$0.04 / 30s clip'
    )
    WHEN 'audio_openrouter_gpt_audio' THEN jsonb_build_object(
      'billing_type', 'per_token',
      'input_price_per_m', 2.5 * billing.usd_to_compute,
      'output_price_per_m', 10 * billing.usd_to_compute,
      'audio_input_price_per_m', 32 * billing.usd_to_compute,
      'audio_output_price_per_m', 64 * billing.usd_to_compute,
      'estimated_input_tokens', 300,
      'estimated_output_tokens', 1500,
      'estimated_audio_output_tokens', 1500,
      'usd_to_compute', billing.usd_to_compute,
      'source_currency', 'USD',
      'source_input_price_per_m', 2.5,
      'source_output_price_per_m', 10,
      'source_audio_input_price_per_m', 32,
      'source_audio_output_price_per_m', 64
    )
    ELSE jsonb_build_object(
      'billing_type', 'per_token',
      'input_price_per_m', 0.6 * billing.usd_to_compute,
      'output_price_per_m', 2.4 * billing.usd_to_compute,
      'audio_input_price_per_m', 0.6 * billing.usd_to_compute,
      'audio_output_price_per_m', 2.4 * billing.usd_to_compute,
      'estimated_input_tokens', 300,
      'estimated_output_tokens', 1500,
      'estimated_audio_output_tokens', 1500,
      'usd_to_compute', billing.usd_to_compute,
      'source_currency', 'USD',
      'source_input_price_per_m', 0.6,
      'source_output_price_per_m', 2.4,
      'source_audio_input_price_per_m', 0.6,
      'source_audio_output_price_per_m', 2.4
    )
  END,
  specs.runtime_rule,
  true,
  specs.sort_order
FROM specs
CROSS JOIN billing
ON CONFLICT (code) DO UPDATE SET
  display_name = EXCLUDED.display_name,
  new_api_model = EXCLUDED.new_api_model,
  new_api_endpoint = EXCLUDED.new_api_endpoint,
  request_mode = EXCLUDED.request_mode,
  category = EXCLUDED.category,
  description = EXCLUDED.description,
  tags = EXCLUDED.tags,
  input_schema = EXCLUDED.input_schema,
  default_params = EXCLUDED.default_params,
  new_api_extra_params = EXCLUDED.new_api_extra_params,
  price_rule = EXCLUDED.price_rule,
  runtime_rule = EXCLUDED.runtime_rule,
  is_enabled = models.is_enabled,
  sort_order = EXCLUDED.sort_order,
  updated_at = now();

WITH billing AS (
  SELECT COALESCE(
    (SELECT NULLIF((value #>> '{}')::numeric, 0)
       FROM system_configs
      WHERE key = 'payment_compute_rate'),
    7.2::numeric
  ) AS usd_to_compute
)
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule,
  cost_rule, priority, weight, timeout_seconds, max_retries, is_enabled,
  health_status, consecutive_failures, cooldown_until, updated_at
)
SELECT
  m.id,
  'OpenRouter 官方',
  'openrouter',
  'openai',
  m.new_api_model,
  '/chat/completions',
  'https://openrouter.ai/api/v1',
  '',
  'bearer',
  'Authorization',
  '{}'::jsonb,
  '{}'::jsonb,
  jsonb_build_object('upstream', jsonb_build_object('adapter', 'openrouter_audio')),
  CASE m.code
    WHEN 'audio_openrouter_lyria_3_pro' THEN jsonb_build_object(
      'billing_type', 'per_request', 'unit_cost', 0.08 * billing.usd_to_compute,
      'source_currency', 'USD', 'source_unit_cost', 0.08, 'usd_to_compute', billing.usd_to_compute
    )
    WHEN 'audio_openrouter_lyria_3_clip' THEN jsonb_build_object(
      'billing_type', 'per_request', 'unit_cost', 0.04 * billing.usd_to_compute,
      'source_currency', 'USD', 'source_unit_cost', 0.04, 'usd_to_compute', billing.usd_to_compute
    )
    WHEN 'audio_openrouter_gpt_audio' THEN jsonb_build_object(
      'billing_type', 'per_token',
      'input_cost_per_m', 2.5 * billing.usd_to_compute,
      'output_cost_per_m', 10 * billing.usd_to_compute,
      'audio_input_cost_per_m', 32 * billing.usd_to_compute,
      'audio_output_cost_per_m', 64 * billing.usd_to_compute,
      'source_currency', 'USD', 'source_input_cost_per_m', 2.5, 'source_output_cost_per_m', 10,
      'source_audio_input_cost_per_m', 32, 'source_audio_output_cost_per_m', 64,
      'usd_to_compute', billing.usd_to_compute
    )
    ELSE jsonb_build_object(
      'billing_type', 'per_token',
      'input_cost_per_m', 0.6 * billing.usd_to_compute,
      'output_cost_per_m', 2.4 * billing.usd_to_compute,
      'audio_input_cost_per_m', 0.6 * billing.usd_to_compute,
      'audio_output_cost_per_m', 2.4 * billing.usd_to_compute,
      'source_currency', 'USD', 'source_input_cost_per_m', 0.6, 'source_output_cost_per_m', 2.4,
      'source_audio_input_cost_per_m', 0.6, 'source_audio_output_cost_per_m', 2.4,
      'usd_to_compute', billing.usd_to_compute
    )
  END,
  100,
  100,
  CASE WHEN m.code LIKE 'audio_openrouter_lyria_%' THEN 900 ELSE 600 END,
  0,
  false,
  'healthy',
  0,
  NULL,
  now()
FROM models AS m
CROSS JOIN billing
WHERE m.code IN (
  'audio_openrouter_lyria_3_pro',
  'audio_openrouter_lyria_3_clip',
  'audio_openrouter_gpt_audio',
  'audio_openrouter_gpt_audio_mini'
)
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider,
  protocol = EXCLUDED.protocol,
  upstream_model = EXCLUDED.upstream_model,
  endpoint = EXCLUDED.endpoint,
  base_url = EXCLUDED.base_url,
  api_key = CASE WHEN model_routes.api_key = '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type,
  api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers,
  extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule,
  cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority,
  weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds,
  max_retries = EXCLUDED.max_retries,
  is_enabled = model_routes.is_enabled,
  updated_at = now();

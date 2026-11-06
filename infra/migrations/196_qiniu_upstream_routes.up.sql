-- 接入七牛云 AI 推理（Modelink，api.qnaigc.com）为上游。
--
-- 覆盖范围：
--   1) 与我方现有模型重合的部分（复用既有 model，仅新增七牛线路）
--      deepseek-v4-1-flash-1017 / deepseek-v4-pro-0813 / glm-5-3-flash /
--      qwen3-8-flash / qwen3-8-max-0902 / kling-video-o1 / seedream-5-0-pro /
--      hailuo-3 / hailuo-3-max
--   2) 七牛有、我方尚无的模型（新建 model + 七牛线路）
--      glm-5 / qwen3-6-plus / kling-v2-6 / kling-v3 /
--      kling-video-4-0 / viduq3 / viduq3-pro / viduq3-turbo / kimi-k2-6
--
-- 下架替换说明（七牛市场接口 retirement_at 字段）：
--   kimi-k2  已于 2026-05-27 下架 → 接入同族在售的 moonshotai/kimi-k2.6
--   kling-v2 已于 2026-09-30 下架（连同 kling-v1/v1.5/v2-new，官方建议迁 kling-v3）
--            → 按 Kling V2 世代接入在售的 kling-v2-6
--
-- 价格口径：全部取自七牛云官方模型市场接口
--   GET https://api.qnaigc.com/v1/market/models
-- （即 portal.qiniu.com/ai-inference/model 模型广场页面展示的官方价），
-- 单位为 CNY。本库 payment_compute_rate = 1，即 1 CNY = 1 CREDITS，
-- 因此 cost_rule 与 price_rule 同值（平价直通，不加价）。
-- 单价换算：市场接口 unit_price 为「每 unit_size 个 token」的价，
-- 换算到每百万 token = unit_price * 1000。
--
-- 线路形态：
--   对话   base_url https://api.qnaigc.com/v1   endpoint /v1/chat/completions   protocol=openai
--   生图   base_url https://api.qnaigc.com/v1   endpoint /v1/images/generations protocol=openai
--          （七牛生图接口即 OpenAI images 形态，适配器复用 openai_images）
--   生视频 base_url https://api.qnaigc.com      endpoint /queue/...             Fal 队列协议
--          七牛视频走 Fal 格式队列：创建 POST /queue/<vendor>/<model>/<op>，
--          查询 GET  /queue/<vendor>/<model>/requests/{id}/status，
--          响应含 status_url，Worker 会自动采用。
--          端点随「文生视频 / 图生视频 / 参考生视频」变化，故由新增的
--          Worker 适配器 qiniu_video 依据 generation_mode 选择，配置在
--          runtime_rule.upstream.endpoints。
--
-- 鉴权：
--   七牛非 Fal 接口用 Authorization: Bearer {key}（auth_type=bearer）。
--   七牛 Fal 队列接口要求 Authorization: Key {key}，平台 auth_type 无此前缀能力，
--   故用 headers 固定 "Key " 前缀 + auth_type=none。
--
-- 密钥通过可选的 PostgreSQL 会话参数 app.qiniu_api_key 注入，不写入仓库。
-- 未提供密钥时，新线路默认禁用；可在后台「模型管理 → 线路」配置后启用。
-- 未注入密钥时保留已有密钥、视频鉴权头和启用状态，不修改现有可用连接。
--
-- 回滚：见 196_qiniu_upstream_routes.down.sql

-- ============================================================================
-- 1. 新建模型（七牛有、我方尚无）
-- ============================================================================
INSERT INTO models (
  code, display_name, new_api_model, new_api_endpoint, request_mode, category,
  icon_url, description, tags, capabilities, input_schema, default_params,
  new_api_extra_params, price_rule, runtime_rule, retention_days, is_enabled, sort_order
)
SELECT
  v.code, v.display_name, v.new_api_model, v.endpoint, v.request_mode, v.category,
  v.icon_url, v.description, v.tags::jsonb, '{}'::jsonb,
  t.input_schema, t.default_params,
  jsonb_build_object('connection', jsonb_build_object(
    'provider', 'qiniu',
    'protocol', 'openai_compatible',
    'base_url', v.base_url,
    'api_key', '',
    'auth_type', 'bearer',
    'api_key_header', 'Authorization',
    'models_endpoint', '/v1/models'
  )),
  v.price_rule::jsonb,
  -- 继承同类模板的 runtime_rule，并把上游适配器改写成七牛形态
  jsonb_set(t.runtime_rule, '{upstream,adapter}', to_jsonb(v.adapter::text), true),
  t.retention_days, true, v.sort_order
FROM (VALUES
  -- ---- 对话 ----
  -- 注：七牛已下架 kimi-k2（retirement_at 2026-05-27），改用同族在售的 K2.6。
  ('kimi-k2-6', 'Kimi K2.6', 'moonshotai/kimi-k2.6', '/v1/chat/completions', 'chat_completions', 'chat',
   NULL::text, '月之暗面 Kimi K2.6。七牛原 kimi-k2 已于 2026-05-27 下架，此处接入同族在售版本。',
   '["对话","moonshot"]', 'https://api.qnaigc.com/v1', 'openai_images',
   '{"currency":"CREDITS","billing_type":"per_token","input_price_per_m":6.5,"output_price_per_m":27,"price_unit_normalized":true,"source_currency":"CNY","source_input_price_per_m":6.5,"source_output_price_per_m":27}',
   300, 'glm-5-3-flash'),
  ('glm-5', 'GLM 5', 'z-ai/glm-5', '/v1/chat/completions', 'chat_completions', 'chat',
   (SELECT icon_url FROM models WHERE code = 'glm-5-3-flash'),
   '智谱 GLM-5，200K 上下文；按输入长度分档计价（≤32K 与 >32K 两档）。',
   '["对话","z-ai"]', 'https://api.qnaigc.com/v1', 'openai_images',
   '{"currency":"CREDITS","billing_type":"per_token","input_price_per_m":4,"output_price_per_m":18,"price_unit_normalized":true,"source_currency":"CNY","source_input_price_per_m":4,"source_output_price_per_m":18,"source_pricing_note":"七牛官方为分档价：输入 ≤32K 为 4/18，>32K 为 6/22；此处取低档"}',
   301, 'glm-5-3-flash'),
  ('qwen3-6-plus', 'Qwen3.6 Plus', 'qwen/qwen3.6-plus', '/v1/chat/completions', 'chat_completions', 'chat',
   (SELECT icon_url FROM models WHERE code = 'qwen3-8-flash'),
   '阿里通义千问 3.6 Plus，1M 上下文，支持文本/图片/视频输入。',
   '["对话","qwen"]', 'https://api.qnaigc.com/v1', 'openai_images',
   '{"currency":"CREDITS","billing_type":"per_token","input_price_per_m":2,"output_price_per_m":12,"cache_read_price_per_m":0.4,"cache_write_price_per_m":2.5,"price_unit_normalized":true,"source_currency":"CNY","source_input_price_per_m":2,"source_output_price_per_m":12,"source_cache_read_price_per_m":0.4,"source_cache_write_price_per_m":2.5,"source_pricing_note":"七牛官方按输入长度分档：≤256K 为 2/12，>256K 为 4/48；此处取低档"}',
   302, 'qwen3-8-flash'),
  -- ---- 生图 ----
  -- ---- 生视频 ----
  -- 注：七牛已下架 kling-v1/v1.5/v2/v2-new（retirement_at 2026-09-30），
  -- 官方建议迁移到 kling-v3；此处按「Kling V2 世代」接入在售的 V2.6。
  ('kling-v2-6', '可灵AI V2.6', 'kling-v2-6', '/videos', 'video', 'video',
   (SELECT icon_url FROM models WHERE code = 'kling-v3-0-pro'),
   '可灵 V2.6 视频生成，支持文生视频、图生视频与原生音频。七牛原 kling-v2 已于 2026-09-30 下架。',
   '["视频","kling","Kling"]', 'https://api.qnaigc.com', 'qiniu_video',
   '{"currency":"CREDITS","billing_type":"per_second","unit_price":0.5,"price_unit_normalized":true,"source_currency":"CNY","source_unit_price":0.5,"source_pricing_note":"七牛官方：std/pro 无参考视频无声均 0.5/秒；有声 1.0/秒，指定音色 1.2/秒"}',
   303, 'kling-v3-0-pro'),
  ('kling-v3', '可灵AI V3', 'kling-v3', '/videos', 'video', 'video',
   (SELECT icon_url FROM models WHERE code = 'kling-v3-0-pro'),
   '可灵 V3 视频生成，支持文生视频、图生视频与原生音频，按时长计费。',
   '["视频","kling","Kling"]', 'https://api.qnaigc.com', 'qiniu_video',
   '{"currency":"CREDITS","billing_type":"per_second","unit_price":0.8,"price_unit_normalized":true,"source_currency":"CNY","source_unit_price":0.8,"source_pricing_note":"七牛官方：std 无参考视频 0.6、pro 无参考视频 0.8；有声 std 0.9 / pro 1.2；4K 3.0。此处取 pro 无参考视频档"}',
   304, 'kling-v3-0-pro'),
  ('kling-video-4-0', '可灵AI Video 4.0', 'kling/kling-video-4.0', '/videos', 'video', 'video',
   (SELECT icon_url FROM models WHERE code = 'kling-v3-0-pro'),
   '可灵 Video 4.0，支持 720p/1080p 文生视频与图生视频，按时长计费。',
   '["视频","kling","Kling"]', 'https://api.qnaigc.com', 'qiniu_video',
   '{"currency":"CREDITS","billing_type":"per_second","unit_price":1.5,"unit_price_by_resolution":{"720p":1.0,"1080p":1.5},"price_unit_normalized":true,"source_currency":"CNY","source_unit_price":1.5,"source_unit_price_by_resolution":{"720p":1.0,"1080p":1.5}}',
   305, 'kling-v3-0-pro'),
  ('viduq3', 'Vidu Q3', 'vidu/viduq3', '/videos', 'video', 'video',
   NULL::text, '生数 Vidu Q3 视频生成，支持参考主体生视频，按分辨率与时长计费。',
   '["视频","vidu","Vidu"]', 'https://api.qnaigc.com', 'qiniu_video',
   '{"currency":"CREDITS","billing_type":"per_second","unit_price":0.375,"unit_price_by_resolution":{"540p":0.21875,"720p":0.375,"1080p":0.46875},"price_unit_normalized":true,"source_currency":"CNY","source_unit_price":0.375,"source_unit_price_by_resolution":{"540p":0.21875,"720p":0.375,"1080p":0.46875}}',
   306, 'wan-3-0'),
  ('viduq3-pro', 'Vidu Q3 Pro', 'viduq3-pro', '/videos', 'video', 'video',
   NULL::text, '生数 Vidu Q3 Pro 视频生成，文生/图生/首尾帧，按分辨率与时长计费。',
   '["视频","vidu","Vidu"]', 'https://api.qnaigc.com', 'qiniu_video',
   '{"currency":"CREDITS","billing_type":"per_second","unit_price":0.9375,"unit_price_by_resolution":{"540p":0.4375,"720p":0.9375,"1080p":1.0},"price_unit_normalized":true,"source_currency":"CNY","source_unit_price":0.9375,"source_unit_price_by_resolution":{"540p":0.4375,"720p":0.9375,"1080p":1.0}}',
   307, 'wan-3-0'),
  ('viduq3-turbo', 'Vidu Q3 Turbo', 'viduq3-turbo', '/videos', 'video', 'video',
   NULL::text, '生数 Vidu Q3 Turbo 视频生成，速度优先，按分辨率与时长计费。',
   '["视频","vidu","Vidu"]', 'https://api.qnaigc.com', 'qiniu_video',
   '{"currency":"CREDITS","billing_type":"per_second","unit_price":0.375,"unit_price_by_resolution":{"540p":0.25,"720p":0.375,"1080p":0.5},"price_unit_normalized":true,"source_currency":"CNY","source_unit_price":0.375,"source_unit_price_by_resolution":{"540p":0.25,"720p":0.375,"1080p":0.5}}',
   308, 'wan-3-0')
) AS v(code, display_name, new_api_model, endpoint, request_mode, category, icon_url,
       description, tags, base_url, adapter, price_rule, sort_order, template)
JOIN models t ON t.code = v.template
ON CONFLICT (code) DO NOTHING;

-- 1b. 推理参数修正
-- 模板继承来的 reasoning_effort=max 并非七牛全渠道支持：
--   z-ai/glm-5            仅接受 none/minimal/low/medium/high/xhigh
--   moonshotai/kimi-k2.6  部分渠道仅接受 low/medium/high/xhigh/none（同一模型多渠道路由，
--                         实测 reasoning_effort=max 会间歇性 400）
-- 统一改为 high（两模型各渠道均接受），关闭档位用 none。
UPDATE models
SET runtime_rule = jsonb_set(
      runtime_rule,
      '{reasoning}',
      '{"mode":"reasoning_effort","on_effort":"high","off_effort":"none","default_enabled":true}'::jsonb,
      true
    ),
    updated_at = now()
WHERE code IN ('glm-5', 'kimi-k2-6');

-- 1c. 新视频模型的可选档位对齐七牛实际支持的取值，避免前端给出上游会拒绝的参数
--     （模板继承自 wan-3-0 / kling-v3-0-pro，档位与七牛官方不一致）
--   Vidu Q3 系列：分辨率 540p/720p/1080p，时长 3~16 秒
UPDATE models
SET input_schema = jsonb_set(
      jsonb_set(input_schema, '{properties,resolution,enum}', '["540p","720p","1080p"]'::jsonb, true),
      '{properties,duration,enum}',
      '[3,4,5,6,7,8,9,10,11,12,13,14,15,16]'::jsonb,
      true
    ),
    updated_at = now()
WHERE code IN ('viduq3', 'viduq3-pro', 'viduq3-turbo');

--   可灵 Video 4.0：分辨率 720p/1080p，时长 3~30 秒
UPDATE models
SET input_schema = jsonb_set(
      jsonb_set(input_schema, '{properties,resolution,enum}', '["720p","1080p"]'::jsonb, true),
      '{properties,duration,enum}',
      '[3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30]'::jsonb,
      true
    ),
    updated_at = now()
WHERE code = 'kling-video-4-0';

--   可灵 V2.6：时长仅支持 5 / 10 秒
UPDATE models
SET input_schema = jsonb_set(input_schema, '{properties,duration,enum}', '[5,10]'::jsonb, true),
    updated_at = now()
WHERE code = 'kling-v2-6';

-- 1d. 品牌图标：Vidu / Kimi(Moonshot)
-- 与既有 imported-icons 约定一致：640×640 透明 PNG；静态资源随仓库部署，
-- 不依赖对象存储，避免生产环境缺图。源图由业务方提供，黑色底已去除，
-- Kimi 的白色 K 改为深色以适配浅色图标底。
UPDATE models
SET icon_url = '/assets/brand/vidu.png', updated_at = now()
WHERE code IN ('viduq3', 'viduq3-pro', 'viduq3-turbo');

UPDATE models
SET icon_url = '/assets/brand/moonshot.png', updated_at = now()
WHERE code = 'kimi-k2-6';

-- ============================================================================
-- 2. 对话线路（8 个模型）
-- ============================================================================
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT
  m.id, '七牛云 Modelink', 'qiniu', 'openai', v.upstream_model, '/v1/chat/completions',
  'https://api.qnaigc.com/v1',
  COALESCE(current_setting('app.qiniu_api_key', true), ''),
  'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, v.cost_rule::jsonb,
  v.priority, 100, 600, 0, COALESCE(current_setting('app.qiniu_api_key', true), '') <> ''
FROM (VALUES
  ('deepseek-v4-1-flash-1017', 'deepseek/deepseek-v4.1-flash-1017', 200,
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"peak_multiplier":2,"peak_ranges":["09:00-12:00","14:00-18:00"],"peak_weekdays_only":true,"source_currency":"CNY","input_cost_per_m":1,"output_cost_per_m":4,"cache_read_cost_per_m":0.02,"price_unit_normalized":true,"source_input_cost_per_m":1,"source_output_cost_per_m":4,"source_cache_read_cost_per_m":0.02,"source_pricing_note":"七牛官方分高峰/空闲：空闲 1/4，高峰 2/8，缓存空闲 0.02、高峰 0.04；此处以空闲为基准 ×2 高峰倍率"}'),
  ('deepseek-v4-pro-0813', 'deepseek/deepseek-v4-pro-0813', 200,
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"peak_multiplier":2,"peak_ranges":["09:00-12:00","14:00-18:00"],"peak_weekdays_only":true,"source_currency":"CNY","input_cost_per_m":4.5,"output_cost_per_m":13.5,"cache_read_cost_per_m":0.15,"price_unit_normalized":true,"source_input_cost_per_m":4.5,"source_output_cost_per_m":13.5,"source_cache_read_cost_per_m":0.15,"source_pricing_note":"七牛官方分高峰/空闲：空闲 4.5/13.5，高峰 9/27"}'),
  ('glm-5-3-flash', 'z-ai/glm-5.3-flash', 200,
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY","input_cost_per_m":0.8,"output_cost_per_m":2.8,"cache_read_cost_per_m":0.23,"price_unit_normalized":true,"source_input_cost_per_m":0.8,"source_output_cost_per_m":2.8,"source_cache_read_cost_per_m":0.23}'),
  ('qwen3-8-flash', 'qwen/qwen3.8-flash-next', 200,
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY","input_cost_per_m":1,"output_cost_per_m":3,"cache_read_cost_per_m":0.1,"cache_write_cost_per_m":1.25,"price_unit_normalized":true,"source_input_cost_per_m":1,"source_output_cost_per_m":3,"source_cache_read_cost_per_m":0.1,"source_cache_write_cost_per_m":1.25}'),
  ('qwen3-8-max-0902', 'qwen/qwen3.8-max', 200,
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY","input_cost_per_m":12,"output_cost_per_m":36,"cache_read_cost_per_m":1.5,"cache_write_cost_per_m":15,"price_unit_normalized":true,"source_input_cost_per_m":12,"source_output_cost_per_m":36,"source_cache_read_cost_per_m":1.5,"source_cache_write_cost_per_m":15}'),
  ('kimi-k2-6', 'moonshotai/kimi-k2.6', 10,
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY","input_cost_per_m":6.5,"output_cost_per_m":27,"price_unit_normalized":true,"source_input_cost_per_m":6.5,"source_output_cost_per_m":27,"source_pricing_note":"七牛原 kimi-k2 已于 2026-05-27 下架，改用同族在售的 K2.6"}'),
  ('glm-5', 'z-ai/glm-5', 10,
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY","input_cost_per_m":4,"output_cost_per_m":18,"price_unit_normalized":true,"source_input_cost_per_m":4,"source_output_cost_per_m":18,"source_pricing_note":"七牛官方分档：输入 ≤32K 为 4/18，>32K 为 6/22；此处取低档"}'),
  ('qwen3-6-plus', 'qwen/qwen3.6-plus', 10,
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY","input_cost_per_m":2,"output_cost_per_m":12,"cache_read_cost_per_m":0.4,"cache_write_cost_per_m":2.5,"price_unit_normalized":true,"source_input_cost_per_m":2,"source_output_cost_per_m":12,"source_cache_read_cost_per_m":0.4,"source_cache_write_cost_per_m":2.5,"source_pricing_note":"七牛官方分档：输入 ≤256K 为 2/12，>256K 为 4/48；此处取低档"}')
) AS v(code, upstream_model, priority, cost_rule)
JOIN models m ON m.code = v.code
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol,
  upstream_model = EXCLUDED.upstream_model, endpoint = EXCLUDED.endpoint,
  base_url = EXCLUDED.base_url,
  api_key = CASE WHEN model_routes.api_key = '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.is_enabled ELSE model_routes.is_enabled END,
  health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- ============================================================================
-- 3. 生图线路（2 个模型）
-- ============================================================================
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT
  m.id, '七牛云 Modelink', 'qiniu', 'openai', v.upstream_model, '/v1/images/generations',
  'https://api.qnaigc.com/v1',
  COALESCE(current_setting('app.qiniu_api_key', true), ''),
  'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb,
  '{"upstream":{"adapter":"openai_images"}}'::jsonb, v.cost_rule::jsonb,
  v.priority, 100, 300, 0, COALESCE(current_setting('app.qiniu_api_key', true), '') <> ''
FROM (VALUES
  ('seedream-5-0-pro', 'bytedance/doubao-seedream-5.0-pro', 200,
   '{"currency":"CREDITS","billing_type":"per_image","compute_per_cny":1,"source_currency":"CNY","unit_cost":0.3,"unit_cost_by_size":{"1K":0.3,"2K":0.6},"source_unit_cost":0.3,"source_unit_cost_by_size":{"1K":0.3,"2K":0.6},"price_unit_normalized":true,"source_pricing_note":"七牛官方：单图 ≤1.5K 0.3，>1.5K 0.6；图片输入 0.02/张（首张免费）"}')
) AS v(code, upstream_model, priority, cost_rule)
JOIN models m ON m.code = v.code
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol,
  upstream_model = EXCLUDED.upstream_model, endpoint = EXCLUDED.endpoint,
  base_url = EXCLUDED.base_url,
  api_key = CASE WHEN model_routes.api_key = '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = EXCLUDED.headers, extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.is_enabled ELSE model_routes.is_enabled END,
  health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

-- ============================================================================
-- 4. 生视频线路（8 个模型，Fal 队列协议）
-- ============================================================================
-- Fal 鉴权为 Authorization: Key {api_key}，用 headers 固定前缀 + auth_type=none。
-- 端点随生成模式变化，交由 Worker 的 qiniu_video 适配器按 generation_mode 选择。
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT
  m.id, '七牛云 Modelink', 'qiniu', 'openai', v.upstream_model, v.endpoint,
  'https://api.qnaigc.com',
  '',
  'none', 'Authorization',
  CASE WHEN COALESCE(current_setting('app.qiniu_api_key', true), '') <> ''
    THEN jsonb_build_object('Authorization', 'Key ' || COALESCE(current_setting('app.qiniu_api_key', true), ''))
    ELSE '{}'::jsonb END,
  '{}'::jsonb,
  jsonb_build_object('upstream', v.upstream::jsonb),
  v.cost_rule::jsonb,
  v.priority, 100, 1800, 0, COALESCE(current_setting('app.qiniu_api_key', true), '') <> ''
FROM (VALUES
  ('kling-video-o1', 'kling-video-o1',
   '/queue/fal-ai/kling-video/o1/pro/text-to-video', 200,
   '{"adapter":"qiniu_video","async":true,"family":"kling","mode":"pro","endpoints":{"text":"/queue/fal-ai/kling-video/o1/pro/text-to-video","first_frame":"/queue/fal-ai/kling-video/o1/pro/image-to-video","first_last":"/queue/fal-ai/kling-video/o1/pro/image-to-video"},"poll_path":"/queue/fal-ai/kling-video/requests/{id}/status","poll_interval_sec":10,"poll_timeout_sec":1800}',
   '{"currency":"CREDITS","billing_type":"per_second","compute_per_cny":1,"source_currency":"CNY","unit_cost":0.8,"source_unit_cost":0.8,"price_unit_normalized":true,"source_pricing_note":"七牛官方：std 无参考视频 0.6 / 有参考视频 0.9；pro 无参考视频 0.8 / 有参考视频 1.2；此处取 pro 无参考视频档"}'),
  ('hailuo-3', 'minimax/minimax-h3',
   '/queue/minimax/h3/text-to-video', 200,
   '{"adapter":"qiniu_video","async":true,"family":"minimax","endpoints":{"text":"/queue/minimax/h3/text-to-video","first_frame":"/queue/minimax/h3/image-to-video","reference":"/queue/minimax/h3/reference-to-video"},"poll_path":"/queue/minimax/h3/requests/{id}/status","poll_interval_sec":10,"poll_timeout_sec":1800}',
   '{"currency":"CREDITS","billing_type":"per_second","compute_per_cny":1,"source_currency":"CNY","unit_cost":0.5,"source_unit_cost":0.5,"unit_cost_by_resolution":{"768P":0.5,"2K":0.8},"source_unit_cost_by_resolution":{"768P":0.5,"2K":0.8},"price_unit_normalized":true,"source_pricing_note":"七牛官方：768P 0.5/秒，2K 0.8/秒；图片输入 0.2/张（前 5 张免费）"}'),
  ('hailuo-3-max', 'minimax/minimax-h3-max',
   '/queue/minimax/h3-max/text-to-video', 200,
   '{"adapter":"qiniu_video","async":true,"family":"minimax","endpoints":{"text":"/queue/minimax/h3-max/text-to-video","first_frame":"/queue/minimax/h3-max/image-to-video","reference":"/queue/minimax/h3-max/reference-to-video"},"poll_path":"/queue/minimax/h3-max/requests/{id}/status","poll_interval_sec":10,"poll_timeout_sec":1800}',
   '{"currency":"CREDITS","billing_type":"per_second","compute_per_cny":1,"source_currency":"CNY","unit_cost":0.5,"source_unit_cost":0.5,"unit_cost_by_resolution":{"480P":0.33,"768P":0.5},"source_unit_cost_by_resolution":{"480P":0.33,"768P":0.5},"price_unit_normalized":true,"source_pricing_note":"七牛官方：480P 0.33/秒，768P 0.5/秒；图片输入 0.5/张（前 2 张免费）"}'),
  ('kling-v2-6', 'kling-v2-6',
   '/queue/fal-ai/kling-video/v2.6/pro/text-to-video', 10,
   '{"adapter":"qiniu_video","async":true,"family":"kling","mode":"pro","endpoints":{"text":"/queue/fal-ai/kling-video/v2.6/pro/text-to-video","first_frame":"/queue/fal-ai/kling-video/v2.6/pro/image-to-video","first_last":"/queue/fal-ai/kling-video/v2.6/pro/image-to-video"},"poll_path":"/queue/fal-ai/kling-video/requests/{id}/status","poll_interval_sec":10,"poll_timeout_sec":1800}',
   '{"currency":"CREDITS","billing_type":"per_second","compute_per_cny":1,"source_currency":"CNY","unit_cost":0.5,"source_unit_cost":0.5,"price_unit_normalized":true,"source_pricing_note":"七牛官方：std/pro 无参考视频无声均 0.5/秒；有声 1.0/秒，指定音色 1.2/秒；此处取无参考视频无声档"}'),
  ('kling-v3', 'kling-v3',
   '/queue/fal-ai/kling-video/v3/pro/text-to-video', 10,
   '{"adapter":"qiniu_video","async":true,"family":"kling","mode":"pro","endpoints":{"text":"/queue/fal-ai/kling-video/v3/pro/text-to-video","first_frame":"/queue/fal-ai/kling-video/v3/pro/image-to-video","first_last":"/queue/fal-ai/kling-video/v3/pro/image-to-video"},"poll_path":"/queue/fal-ai/kling-video/requests/{id}/status","poll_interval_sec":10,"poll_timeout_sec":1800}',
   '{"currency":"CREDITS","billing_type":"per_second","compute_per_cny":1,"source_currency":"CNY","unit_cost":0.8,"source_unit_cost":0.8,"price_unit_normalized":true,"source_pricing_note":"七牛官方：std 无参考视频 0.6 / pro 无参考视频 0.8；有声 std 0.9 / pro 1.2；4K 3.0；此处取 pro 无参考视频档"}'),
  ('kling-video-4-0', 'kling/kling-video-4.0',
   '/queue/kling/v4/text-to-video', 10,
   '{"adapter":"qiniu_video","async":true,"family":"kling4","endpoints":{"text":"/queue/kling/v4/text-to-video","first_frame":"/queue/kling/v4/image-to-video","reference":"/queue/kling/v4/reference-to-video"},"poll_path":"/queue/kling/v4/requests/{id}/status","poll_interval_sec":10,"poll_timeout_sec":1800}',
   '{"currency":"CREDITS","billing_type":"per_second","compute_per_cny":1,"source_currency":"CNY","unit_cost":1.5,"source_unit_cost":1.5,"unit_cost_by_resolution":{"720p":1.0,"1080p":1.5},"source_unit_cost_by_resolution":{"720p":1.0,"1080p":1.5},"price_unit_normalized":true,"source_pricing_note":"七牛官方：720p 无输入视频 1.0/秒、有输入视频 0.6/秒；1080p 无输入视频 1.5/秒、有输入视频 0.9/秒"}'),
  ('viduq3', 'vidu/viduq3',
   '/queue/fal-ai/vidu/q3/reference-to-video', 10,
   '{"adapter":"qiniu_video","async":true,"family":"vidu","tier":"","endpoints":{"reference":"/queue/fal-ai/vidu/q3/reference-to-video"},"poll_path":"/queue/fal-ai/vidu/requests/{id}/status","poll_interval_sec":10,"poll_timeout_sec":1800}',
   '{"currency":"CREDITS","billing_type":"per_second","compute_per_cny":1,"source_currency":"CNY","unit_cost":0.375,"source_unit_cost":0.375,"unit_cost_by_resolution":{"540p":0.21875,"720p":0.375,"1080p":0.46875},"source_unit_cost_by_resolution":{"540p":0.21875,"720p":0.375,"1080p":0.46875},"price_unit_normalized":true,"source_pricing_note":"七牛官方参考主体生视频：540p 0.21875、720p 0.375、1080p 0.46875 元/秒"}'),
  ('viduq3-pro', 'viduq3-pro',
   '/queue/fal-ai/vidu/q3/text-to-video/pro', 10,
   '{"adapter":"qiniu_video","async":true,"family":"vidu","tier":"pro","endpoints":{"text":"/queue/fal-ai/vidu/q3/text-to-video/pro","first_frame":"/queue/fal-ai/vidu/q3/image-to-video/pro","first_last":"/queue/fal-ai/vidu/q3/start-end-to-video/pro"},"poll_path":"/queue/fal-ai/vidu/requests/{id}/status","poll_interval_sec":10,"poll_timeout_sec":1800}',
   '{"currency":"CREDITS","billing_type":"per_second","compute_per_cny":1,"source_currency":"CNY","unit_cost":0.9375,"source_unit_cost":0.9375,"unit_cost_by_resolution":{"540p":0.4375,"720p":0.9375,"1080p":1.0},"source_unit_cost_by_resolution":{"540p":0.4375,"720p":0.9375,"1080p":1.0},"price_unit_normalized":true,"source_pricing_note":"七牛官方文生/图生视频：540p 0.4375、720p 0.9375、1080p 1.0 元/秒"}'),
  ('viduq3-turbo', 'viduq3-turbo',
   '/queue/fal-ai/vidu/q3/text-to-video/turbo', 10,
   '{"adapter":"qiniu_video","async":true,"family":"vidu","tier":"turbo","endpoints":{"text":"/queue/fal-ai/vidu/q3/text-to-video/turbo","first_frame":"/queue/fal-ai/vidu/q3/image-to-video/turbo","first_last":"/queue/fal-ai/vidu/q3/start-end-to-video/turbo","reference":"/queue/fal-ai/vidu/q3/reference-to-video/turbo"},"poll_path":"/queue/fal-ai/vidu/requests/{id}/status","poll_interval_sec":10,"poll_timeout_sec":1800}',
   '{"currency":"CREDITS","billing_type":"per_second","compute_per_cny":1,"source_currency":"CNY","unit_cost":0.375,"source_unit_cost":0.375,"unit_cost_by_resolution":{"540p":0.25,"720p":0.375,"1080p":0.5},"source_unit_cost_by_resolution":{"540p":0.25,"720p":0.375,"1080p":0.5},"price_unit_normalized":true,"source_pricing_note":"七牛官方文生/图生视频：540p 0.25、720p 0.375、1080p 0.5 元/秒；参考主体 540p 0.125、720p 0.3125、1080p 0.40625"}')
) AS v(code, upstream_model, endpoint, priority, upstream, cost_rule)
JOIN models m ON m.code = v.code
ON CONFLICT (model_id, route_name) DO UPDATE SET
  provider = EXCLUDED.provider, protocol = EXCLUDED.protocol,
  upstream_model = EXCLUDED.upstream_model, endpoint = EXCLUDED.endpoint,
  base_url = EXCLUDED.base_url,
  api_key = CASE WHEN model_routes.api_key = '' THEN EXCLUDED.api_key ELSE model_routes.api_key END,
  auth_type = EXCLUDED.auth_type, api_key_header = EXCLUDED.api_key_header,
  headers = CASE WHEN EXCLUDED.headers ? 'Authorization' THEN EXCLUDED.headers ELSE model_routes.headers END,
  extra_params = EXCLUDED.extra_params,
  runtime_rule = EXCLUDED.runtime_rule, cost_rule = EXCLUDED.cost_rule,
  priority = EXCLUDED.priority, weight = EXCLUDED.weight,
  timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries,
  is_enabled = CASE WHEN EXCLUDED.headers ? 'Authorization' THEN EXCLUDED.is_enabled ELSE model_routes.is_enabled END,
  health_status = 'healthy',
  consecutive_failures = 0, cooldown_until = NULL, updated_at = now();

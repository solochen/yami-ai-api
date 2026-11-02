-- 国产上游直连：阿里云百炼（文本）+ 火山方舟（图像），并下架百炼不卖的 qwen3.8-max-prime。
--
-- 背景：迁移 172 把用户售价统一锁定为厂商官方挂牌价后，走 OpenRouter 的国产模型普遍亏损
-- （qwen 系 -20~50%、seedream/seedance -8~15%）。本迁移为这些模型补上原厂直连线路，
-- 优先级 5（排在 OpenRouter 的 20 之前），让流量优先走原厂。
--
-- 定价口径：成本 = 厂商官方价，售价 = 官方价（迁移 172 已锁），因此这批线路毛利≈0，
-- 属于「止血」而非盈利；真正的利润来自 Batch 批量半价、免费额度与企业协议折扣。
--
-- 技术边界：
--   * 文本：百炼提供 OpenAI 兼容端点（compatible-mode/v1），可直接复用现有 openai 适配器。
--   * 图像：火山方舟 /api/v3/images/generations 为同步接口，请求体与 OpenAI 图像协议兼容。
--   * 视频：火山 Seedance 走异步任务接口（contents/generations/tasks + 轮询），
--     与现有 /videos 同步协议不兼容，需另写适配器，故本次不接入（seedance 仍在 OR 上）。
--
-- 密钥均为占位符且线路停用，需在后台填入真实 Key 后启用。
-- 回滚：见 173_bailian_volcano_routes.down.sql

-- ========= 1. 阿里云百炼：文本模型 =========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, '阿里云百炼 · 原厂直连', 'bailian', 'openai',
       v.upstream_model, '/chat/completions', 'https://dashscope.aliyuncs.com/compatible-mode/v1',
       'REPLACE_ME_BAILIAN_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, v.cost_rule::jsonb,
       5, 100, 120, 1, false
FROM models m
JOIN (VALUES
  ('qwen3-8-max-0902',  'qwen3.8-max',
   '{"billing_type":"per_token","input_cost_per_m":12,"output_cost_per_m":36,"cache_read_cost_per_m":1.87}'),
  ('qwen3-8-flash',     'qwen3.8-flash',
   '{"billing_type":"per_token","input_cost_per_m":0.8,"output_cost_per_m":2.7,"cache_read_cost_per_m":0.1}'),
  ('qwen3-8-27b',       'qwen3.8-27b',
   '{"billing_type":"per_token","input_cost_per_m":3,"output_cost_per_m":12,"cache_read_cost_per_m":0.3}'),
  ('qwen3-8-omni-flash','qwen3.8-omni-flash',
   '{"billing_type":"per_token","input_cost_per_m":0.8,"output_cost_per_m":2.7,"cache_read_cost_per_m":0.1}'),
  ('glm-5-3-prime',     'glm-5.3-prime',
   '{"billing_type":"per_token","input_cost_per_m":16,"output_cost_per_m":56,"cache_read_cost_per_m":3.75}')
) AS v(code, upstream_model, cost_rule) ON v.code = m.code;

-- ========= 2. 火山方舟：Seedream 图像（同步接口，OpenAI 兼容）=========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, '火山方舟 · 原厂直连', 'volcano', 'openai',
       v.upstream_model, '/images/generations', 'https://ark.cn-beijing.volces.com/api/v3',
       'REPLACE_ME_VOLCANO_ARK_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, v.cost_rule::jsonb,
       5, 100, 180, 1, false
FROM models m
JOIN (VALUES
  ('seedream-4-5',      'doubao-seedream-4-5',
   '{"billing_type":"per_image","unit_cost":0.25,"unit_cost_by_size":{"1K":0.25,"2K":0.25}}'),
  ('seedream-5-0-lite', 'doubao-seedream-5-0-lite',
   '{"billing_type":"per_image","unit_cost":0.22,"unit_cost_by_size":{"2K":0.22,"4K":0.22}}'),
  ('seedream-5-0-pro',  'doubao-seedream-5-0-pro',
   '{"billing_type":"per_image","unit_cost":0.3,"unit_cost_by_size":{"1K":0.3,"2K":0.6}}')
) AS v(code, upstream_model, cost_rule) ON v.code = m.code;

-- ========= 3. 下架 qwen3.8-max-prime =========
-- 百炼不售卖该档位（只有 max / flash / 27b / omni-flash），第三方追踪价 ¥24/72 无法核实，
-- 保留会让用户点到无法稳定供货、且按不可靠官方价亏损的模型。
UPDATE models SET is_enabled = false, updated_at = now() WHERE code = 'qwen3-8-max-prime';

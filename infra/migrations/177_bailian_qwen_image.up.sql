-- 阿里云百炼图像模型直连：Qwen-Image 系列走 DashScope 原生协议。
--
-- 官方文档明确：千问-文生图（Qwen-Image）等图像生成模型通过 DashScope 原生接口调用，
-- **不支持 OpenAI 兼容（compatible-mode）模式**。因此不能复用 /images/generations，
-- 必须使用原生端点，且请求/响应结构与 OpenAI 图像协议不同：
--   请求 { model, input:{messages:[{role,content:[{text}]}]}, parameters:{size,n,watermark,...} }
--   响应 output.choices[].message.content[].image（同步返回，24 小时后 URL 失效，
--        由 worker 的 persistGeneratedMedia 转存到自己的对象存储）
-- 响应解析复用 worker 里 extractMediaItems 的递归逻辑（output → choices → message → content → image），
-- 仅请求体需要新写的 dashscope_image 适配器。
--
-- Endpoint 约定（worker 的 requiresAliyunWorkspaceEndpoint 强校验）：
--   base_url 必须是「裸工作空间域名」（无路径），完整路径写进 endpoint。
--
-- 成本 = 百炼官方牌价，与迁移 172 锁定的售价一致 → 毛利 0（止血线，走 Batch 半价才有利润）。
-- 密钥占位、线路停用；填 Key 后启用即可，优先级 5 排在 OpenRouter 的 100 之前。
-- 回滚：见 177_bailian_qwen_image.down.sql

INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, '阿里云百炼 · Qwen-Image 原生', 'bailian', 'openai',
       v.upstream_model, '/api/v1/services/aigc/multimodal-generation/generation',
       'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com',
       'REPLACE_ME_BAILIAN_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{"upstream":{"adapter":"dashscope_image"}}'::jsonb,
       v.cost_rule::jsonb,
       5, 100, 180, 1, false
FROM models m
JOIN (VALUES
  ('qwen-image-3', 'qwen-image-3.0',
   '{"billing_type":"per_image","unit_cost":0.2,"unit_cost_by_size":{"1K":0.2,"2K":0.2}}'),
  ('qwen-image-3-pro', 'qwen-image-3.0-pro',
   '{"billing_type":"per_image","unit_cost":0.27,"unit_cost_by_size":{"1K":0.27,"2K":0.52}}')
) AS v(code, upstream_model, cost_rule) ON v.code = m.code;

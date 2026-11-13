-- CodexAuv 国产文本模型补充接入：优先复用已配置的 codexauv 密钥，priority=3 排在全站最前。
--
-- 交集范围（CodexAuv「国产模型聚合」分组只卖文本，无图像/视频）：
--   qwen3.8-max    → qwen3-8-max-0902
--   qwen3.8-flash  → qwen3-8-flash
--   glm-5.3-flash  → glm-5-3-flash
--   glm-5.3-flashx → glm-5-3-flashx
--   glm-5.3-prime  → glm-5-3-prime
-- 未覆盖：qwen3.8-27b / qwen3.8-omni-flash（CodexAuv 无此型号）、全部图像与视频模型。
--
-- 价格（CodexAuv 模型广场公布价，人民币/百万 token）：
--   qwen3.8-max 12/36、qwen3.8-flash 0.8/2.7、glm-5.3-flash 0.8/2.8 —— 与官方价一致，毛利≈0
--   **glm-5.3-flashx 0.15/0.5、glm-5.3-prime 2.8/8.8** —— 显著低于官方（2/7、16/56），
--   若线上按此计价，按官方售价可获 75~82% 毛利；但这两个数字异常便宜，**必须用首笔账单核对**，
--   故 cache 单价仍按官方上限记录，避免高估利润。
--
-- 售价：沿用迁移 172 锁定的官方挂牌价（user_price_rule 不变），本迁移只加成本侧线路。
-- 回滚：见 174_codexauv_domestic_text.down.sql

INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, 'CodexAuv 国产聚合 · 主线路', 'codexauv', 'openai',
       v.upstream_model, '/v1/chat/completions', 'https://codexauv.com',
       COALESCE((SELECT r2.api_key FROM model_routes r2
        WHERE r2.provider = 'codexauv' AND r2.api_key NOT LIKE 'REPLACE_ME%'
        ORDER BY r2.id LIMIT 1), ''),
       'bearer', 'Authorization', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, v.cost_rule::jsonb,
       3, 100, 120, 1, true
FROM models m
JOIN (VALUES
  ('qwen3-8-max-0902', 'qwen3.8-max',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":12,"output_cost_per_m":36,"cache_read_cost_per_m":1.87,
     "price_unit_normalized":true,"source_input_cost_per_m":12,"source_output_cost_per_m":36}'),
  ('qwen3-8-flash', 'qwen3.8-flash',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":0.8,"output_cost_per_m":2.7,"cache_read_cost_per_m":0.1,
     "price_unit_normalized":true,"source_input_cost_per_m":0.8,"source_output_cost_per_m":2.7}'),
  ('glm-5-3-flash', 'glm-5.3-flash',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":0.8,"output_cost_per_m":2.8,"cache_read_cost_per_m":0.23,
     "price_unit_normalized":true,"source_input_cost_per_m":0.8,"source_output_cost_per_m":2.8}'),
  ('glm-5-3-flashx', 'glm-5.3-flashx',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":0.15,"output_cost_per_m":0.5,"cache_read_cost_per_m":0.57,
     "price_unit_normalized":true,"source_input_cost_per_m":0.15,"source_output_cost_per_m":0.5}'),
  ('glm-5-3-prime', 'glm-5.3-prime',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":2.8,"output_cost_per_m":8.8,"cache_read_cost_per_m":3.75,
     "price_unit_normalized":true,"source_input_cost_per_m":2.8,"source_output_cost_per_m":8.8}')
) AS v(code, upstream_model, cost_rule) ON v.code = m.code;

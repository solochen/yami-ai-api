-- 修正 174 引入的售价回落问题：新增的 codexauv 线路没有 user_price_rule，
-- publicPriceRule 会选中优先级最高的可计费线路并回落到 cost_rule，
-- 导致 glm-5.3-flashx 展示价从官方 ¥2/¥7 骤降到 CodexAuv 成本 ¥0.15/¥0.5、
-- glm-5.3-prime 从 ¥16/¥56 降到 ¥2.8/¥8.8。
--
-- 处置：给这 5 条 codexauv 线路补上与官方挂牌价一致的 user_price_rule，
-- 使展示价/扣费价与线路无关（迁移 162「售价与成本解耦」原则）。
-- 补完之后 CodexAuv 的进货差价直接体现为平台毛利：
--   glm-5.3-flashx 成本 0.15/0.5 vs 售价 2/7    → 毛利约 92%
--   glm-5.3-prime  成本 2.8/8.8  vs 售价 16/56  → 毛利约 82%
--   qwen3.8-max / flash / glm-5.3-flash         → 成本=官方价，毛利 0
-- 回滚：见 175_codexauv_domestic_user_price.down.sql

UPDATE model_routes r
SET user_price_rule = v.rule::jsonb, updated_at = now()
FROM models m
JOIN (VALUES
  ('qwen3-8-max-0902', '{"billing_type":"per_token","input_cost_per_m":12,"output_cost_per_m":36,"cache_read_cost_per_m":1.87}'),
  ('qwen3-8-flash',    '{"billing_type":"per_token","input_cost_per_m":0.8,"output_cost_per_m":2.7,"cache_read_cost_per_m":0.1}'),
  ('glm-5-3-flash',    '{"billing_type":"per_token","input_cost_per_m":0.8,"output_cost_per_m":2.8,"cache_read_cost_per_m":0.23}'),
  ('glm-5-3-flashx',   '{"billing_type":"per_token","input_cost_per_m":2,"output_cost_per_m":7,"cache_read_cost_per_m":0.57}'),
  ('glm-5-3-prime',    '{"billing_type":"per_token","input_cost_per_m":16,"output_cost_per_m":56,"cache_read_cost_per_m":3.75}')
) AS v(code, rule) ON v.code = m.code
WHERE r.model_id = m.id AND r.provider = 'codexauv';

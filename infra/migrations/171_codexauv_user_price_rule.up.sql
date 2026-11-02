-- 给 CodexAuv 两条 DeepSeek 线路补 user_price_rule，与 OR/TokenHub 线路保持同一售价口径。
--
-- 背景：迁移 170 新增的 codexauv 线路未配 user_price_rule，publicPriceRule 回落到
-- cost_rule（成本口径），导致切到该线路后用户可见价直接减半（flash 2.17→1.087），
-- 且线路降级时价格来回跳。售价应与线路解耦（迁移 162 原则），三条线路统一锁定：
--   deepseek-v4-1-flash-1017   售价基准 2 / 8 / 缓存读 0.04（每百万 token，算力）
--   deepseek-v4-pro-0813       售价基准 9 / 27 / 缓存读 0.3
-- 页面展示价 = 售价基准 ÷ (1 − 8%)。codexauv 成本约为售价一半，毛利率升至约 50%。
-- 回滚：置回 '{}' 即回落成本口径。

UPDATE model_routes r
SET user_price_rule = '{
  "billing_type": "per_token",
  "input_cost_per_m": 2,
  "output_cost_per_m": 8,
  "cache_read_cost_per_m": 0.04
}'::jsonb,
  updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND m.code = 'deepseek-v4-1-flash-1017'
  AND r.provider = 'codexauv';

UPDATE model_routes r
SET user_price_rule = '{
  "billing_type": "per_token",
  "input_cost_per_m": 9,
  "output_cost_per_m": 27,
  "cache_read_cost_per_m": 0.3
}'::jsonb,
  updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND m.code = 'deepseek-v4-pro-0813'
  AND r.provider = 'codexauv';

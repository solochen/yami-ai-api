-- 回滚 175：清空 codexauv 国产文本线路的售价规则（回落成本口径）。

UPDATE model_routes r
SET user_price_rule = '{}'::jsonb, updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND r.provider = 'codexauv'
  AND m.code IN ('qwen3-8-max-0902','qwen3-8-flash','glm-5-3-flash','glm-5-3-flashx','glm-5-3-prime');

-- 回滚 171：清空 codexauv 线路的售价规则，回落成本口径。

UPDATE model_routes r
SET user_price_rule = '{}'::jsonb, updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND m.code IN ('deepseek-v4-1-flash-1017', 'deepseek-v4-pro-0813')
  AND r.provider = 'codexauv';

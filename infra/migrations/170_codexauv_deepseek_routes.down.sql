-- 回滚 170：移除 CodexAuv 的两条 DeepSeek 线路。

DELETE FROM model_routes
WHERE provider = 'codexauv'
  AND model_id IN (SELECT id FROM models WHERE code IN ('deepseek-v4-pro-0813', 'deepseek-v4-1-flash-1017'));

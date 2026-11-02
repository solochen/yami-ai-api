-- 回滚 173：移除百炼/火山线路并恢复 qwen3.8-max-prime 上架。

DELETE FROM model_routes
WHERE provider IN ('bailian', 'volcano')
  AND model_id IN (SELECT id FROM models WHERE code IN (
    'qwen3-8-max-0902','qwen3-8-flash','qwen3-8-27b','qwen3-8-omni-flash','glm-5-3-prime',
    'seedream-4-5','seedream-5-0-lite','seedream-5-0-pro'
  ));

UPDATE models SET is_enabled = true, updated_at = now() WHERE code = 'qwen3-8-max-prime';

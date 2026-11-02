-- 回滚 177：移除百炼 Qwen-Image 原生线路。

DELETE FROM model_routes r USING models m
WHERE m.id = r.model_id
  AND r.provider = 'bailian'
  AND m.code IN ('qwen-image-3', 'qwen-image-3-pro');

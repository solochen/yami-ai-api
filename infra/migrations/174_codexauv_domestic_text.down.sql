-- 回滚 174：移除 CodexAuv 国产文本线路（DeepSeek 的两条由迁移 170 管理，不在本迁移范围）。

DELETE FROM model_routes r
USING models m
WHERE m.id = r.model_id
  AND r.provider = 'codexauv'
  AND m.code IN ('qwen3-8-max-0902','qwen3-8-flash','glm-5-3-flash','glm-5-3-flashx','glm-5-3-prime');

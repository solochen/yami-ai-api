-- 回滚 PackyCode 线路，并恢复 GPT 三条本地 new_api 线路的 priority=0。
-- 回滚后 PackyCode 路由全部删除（含已填 Key 的线路），如需保留 Key 请先备份 model_routes。

DELETE FROM model_routes r
USING models m
WHERE r.model_id = m.id
  AND r.provider = 'packycode'
  AND m.code IN (
    'qwen3-8-flash','qwen3-8-max-0902','deepseek-v4-1-flash-1017','deepseek-v4-pro-0813',
    'gpt-5-6-sol','gpt-6-astra','gpt-6-sol','gemini-3-8-flash','gemini-3-7-flash',
    'glm-5-3-flash','claude-sonnet-5','claude-opus-5-5','claude-opus-4-8',
    'claude-fable-5','claude-fable-5-1','grok-imagine-image-2-0'
  );

UPDATE model_routes r
SET priority = 0, updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND r.provider = 'new_api'
  AND m.code IN ('gpt-5-6-sol', 'gpt-6-astra', 'gpt-6-sol')
  AND r.priority = 90;

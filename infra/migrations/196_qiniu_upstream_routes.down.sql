-- 回滚 196_qiniu_upstream_routes.up.sql
--
-- 只删除本次新增的七牛线路；对于本次新建的模型，仅当其已无任何线路时才删除，
-- 避免误删后台手工补充的其他线路。

DELETE FROM model_routes r
USING models m
WHERE r.model_id = m.id
  AND r.provider = 'qiniu'
  AND r.route_name = '七牛云 Modelink';

DELETE FROM models m
WHERE m.code IN (
  'kimi-k2-6', 'glm-5', 'qwen3-6-plus',
  'kling-v2-6', 'kling-v3', 'kling-video-4-0', 'viduq3', 'viduq3-pro', 'viduq3-turbo'
)
AND NOT EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id);

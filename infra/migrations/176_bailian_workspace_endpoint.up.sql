-- 百炼线路切换到业务空间专属域名（官方推荐，性能与稳定性更好）。
--
-- 官方文档：北京地域 Base URL 为 https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com/compatible-mode/v1，
-- 旧域名 https://dashscope.aliyuncs.com/compatible-mode/v1 仍可用但不推荐。
-- 本站业务空间 ID：ws-bzvsw2wexhbe3hpf。
--
-- 注意：endpoint 保持 /chat/completions 不变，base_url 只到 /compatible-mode/v1；
-- API Key 必须在北京地域创建（Key 按地域绑定，跨地域调用返回 401 invalid_api_key）。
-- 回滚：见 176_bailian_workspace_endpoint.down.sql

UPDATE model_routes
SET base_url = 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1',
    updated_at = now()
WHERE provider = 'bailian'
  AND base_url = 'https://dashscope.aliyuncs.com/compatible-mode/v1';

-- 把自建 New API（hub.yardai.cc）的价格源切到上游检查器 sidecar。
-- 本地开发默认 host.docker.internal:3104；生产改为 https://hub.yardai.cc 即可。
UPDATE upstream_providers
SET source_type='new_api_hub',
    price_endpoint='http://host.docker.internal:3104',
    config = COALESCE(config,'{}'::jsonb) || '{"group":"default","days":7}'::jsonb,
    is_enabled=true,
    updated_at=now()
WHERE provider='new_api';

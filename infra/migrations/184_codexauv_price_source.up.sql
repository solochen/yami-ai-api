-- CodexAuv 价格源：改用公开 model-plaza JSON 抓取；本地通过 host 代理，生产可去掉 proxy_url。
-- BigModel 上游渠道已删除，停用其监控来源与线路。
UPDATE upstream_providers
SET source_type='codexauv_page',
    price_endpoint='https://codexauv.com',
    config = COALESCE(config,'{}'::jsonb) || '{"proxy_url":"http://host.docker.internal:7897"}'::jsonb,
    is_enabled=true,
    updated_at=now()
WHERE provider='codexauv';

UPDATE upstream_providers
SET is_enabled=false, source_type='manual', updated_at=now()
WHERE provider='bigmodel';

UPDATE model_routes r SET is_enabled=false, updated_at=now()
FROM models m
WHERE r.model_id=m.id AND r.provider='bigmodel';

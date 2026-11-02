UPDATE upstream_providers
SET source_type='manual', price_endpoint='',
    config = COALESCE(config,'{}'::jsonb) - 'proxy_url',
    updated_at=now()
WHERE provider='codexauv';

UPDATE upstream_providers
SET is_enabled=true, source_type='official_page', updated_at=now()
WHERE provider='bigmodel';

UPDATE model_routes r SET is_enabled=true, updated_at=now()
FROM models m
WHERE r.model_id=m.id AND r.provider='bigmodel';

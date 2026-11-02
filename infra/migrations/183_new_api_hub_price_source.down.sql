UPDATE upstream_providers
SET source_type='manual',
    price_endpoint='',
    config = COALESCE(config,'{}'::jsonb) - 'group' - 'days',
    updated_at=now()
WHERE provider='new_api';

UPDATE system_configs
SET value = '3'::jsonb, updated_at = now()
WHERE key = 'platform_gross_margin_percent';

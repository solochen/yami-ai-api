-- Restore the empty defaults installed by 020_legal_favicon_config.

UPDATE system_configs SET value = '""'::jsonb
WHERE key IN ('privacy_content', 'terms_content');

UPDATE system_configs SET value = '"隐私政策"'::jsonb WHERE key = 'privacy_title';
UPDATE system_configs SET value = '"服务协议"'::jsonb WHERE key = 'terms_title';

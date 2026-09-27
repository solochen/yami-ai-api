UPDATE system_configs
SET value = '"api.vidu.com"'::jsonb, updated_at = now()
WHERE key = 'vidu_api_host'
  AND value #>> '{}' IN ('api.vidu.cn', 'https://api.vidu.cn', 'http://api.vidu.cn');

UPDATE system_configs
SET value = '"api.vidu.cn"'::jsonb, updated_at = now()
WHERE key = 'vidu_api_host'
  AND value #>> '{}' IN ('api.vidu.com', 'https://api.vidu.com', 'http://api.vidu.com');

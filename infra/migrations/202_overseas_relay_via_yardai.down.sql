-- 202 回滚：恢复海外模型 OpenRouter 线路、移除中转线路、还原价格监控源。

INSERT INTO model_routes SELECT * FROM model_routes_backup_202 ON CONFLICT (id) DO NOTHING;
DROP TABLE model_routes_backup_202;

DELETE FROM model_routes r USING models m
WHERE r.model_id = m.id AND r.route_name = '新加坡中转 · yardai';

UPDATE upstream_providers SET
  source_type = 'new_api_hub', price_endpoint = 'http://host.docker.internal:3104',
  currency = 'CNY', recharge_rate = 1, is_enabled = true, updated_at = now()
WHERE provider = 'new_api';
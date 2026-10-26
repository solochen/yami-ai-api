-- 回滚 162：清空售价规则、还原优先级与模型售价、删除告警表与配置。
UPDATE model_routes SET user_price_rule = '{}'::jsonb, updated_at = now();

UPDATE model_routes r SET priority = 20, updated_at = now()
FROM models m
WHERE m.id = r.model_id AND m.code IN ('deepseek-v4-pro-0813','deepseek-v4-1-flash-1017') AND r.provider = 'tokenhub';

UPDATE model_routes r SET priority = 100, updated_at = now()
FROM models m
WHERE m.id = r.model_id AND m.code IN ('deepseek-v4-pro-0813','deepseek-v4-1-flash-1017') AND r.provider = 'openrouter';

UPDATE models SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":4.5,"output_price_per_m":13.5,"cache_read_price_per_m":0.15}'::jsonb, updated_at = now()
WHERE code = 'deepseek-v4-pro-0813';

UPDATE models SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":1,"output_price_per_m":4,"cache_read_price_per_m":0.02}'::jsonb, updated_at = now()
WHERE code = 'deepseek-v4-1-flash-1017';

DROP TABLE IF EXISTS price_drift_alerts;
DELETE FROM system_configs WHERE key IN ('price_drift_threshold_percent','price_drift_alert_email');

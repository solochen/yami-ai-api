-- 回滚 179：删除价格监控相关表、price_group 列与配置项。
-- 注意：会丢失已同步的价格快照、切换日志和用量日汇总；model_routes / models 本身不受影响。

DROP TABLE IF EXISTS model_route_switch_logs;
DROP TABLE IF EXISTS model_usage_daily;
DROP TABLE IF EXISTS price_sync_provider_runs;
DROP TABLE IF EXISTS price_sync_runs;
DROP TABLE IF EXISTS upstream_price_current;
DROP TABLE IF EXISTS upstream_price_snapshots;
DROP TABLE IF EXISTS upstream_model_mappings;
DROP TABLE IF EXISTS upstream_providers;

ALTER TABLE model_routes DROP COLUMN IF EXISTS price_group;

DELETE FROM system_configs WHERE key IN (
  'usd_cny_rate',
  'price_sync_concurrency',
  'price_sync_timeout_seconds',
  'price_sync_retries',
  'price_sync_stale_hours',
  'price_gap_alert_threshold_percent',
  'price_loss_margin_threshold_percent',
  'price_snapshot_retention_days',
  'price_sync_alert_email'
);

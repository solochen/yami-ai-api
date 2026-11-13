-- 209_alert_manual_migrate：现有告警通道补订阅「手动批量迁移」事件。
UPDATE alert_channels
SET notify_events = CASE
  WHEN COALESCE(notify_events,'') = '' THEN 'route_failover,account_down,all_routes_down,manual_migrate'
  WHEN POSITION('manual_migrate' IN notify_events) > 0 THEN notify_events
  ELSE notify_events || ',manual_migrate'
END,
updated_at = now();

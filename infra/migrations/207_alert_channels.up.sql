-- 207_alert_channels：钉钉/飞书告警通道配置。
CREATE TABLE IF NOT EXISTS alert_channels (
  id            bigserial PRIMARY KEY,
  name          varchar(128) NOT NULL,
  type          varchar(16)  NOT NULL,
  webhook_url   text         NOT NULL,
  secret        text         NOT NULL DEFAULT '',
  is_enabled    boolean      NOT NULL DEFAULT true,
  notify_events varchar(255) NOT NULL DEFAULT 'route_failover,account_down,all_routes_down',
  created_at    timestamptz  NOT NULL DEFAULT now(),
  updated_at    timestamptz  NOT NULL DEFAULT now()
);

-- 206_upstream_events：记录上游线路切换 / 账号不可用 / 恢复等事件，供告警通知与排查使用。
CREATE TABLE IF NOT EXISTS upstream_events (
  id            bigserial PRIMARY KEY,
  event_type    varchar(32)  NOT NULL, -- route_failover / account_down / account_recovered / all_routes_down
  provider      varchar(64)  NOT NULL DEFAULT '',
  account_id    bigint,
  model_id      bigint,
  model_code    varchar(64)  NOT NULL DEFAULT '',
  from_route_id bigint,
  to_route_id   bigint,
  reason        text         NOT NULL DEFAULT '',
  status_code   int,
  notified      boolean      NOT NULL DEFAULT false,
  notified_at   timestamptz,
  created_at    timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_upstream_events_pending ON upstream_events (notified, created_at);
CREATE INDEX IF NOT EXISTS idx_upstream_events_model ON upstream_events (model_id, created_at);

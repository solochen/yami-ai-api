-- 212_upstream_sync：上游分组账号一键同步（同步源 / 运行记录 / 明细）。

CREATE TABLE IF NOT EXISTS upstream_sync_sources (
  id              bigserial PRIMARY KEY,
  provider        varchar(64)  NOT NULL,
  adapter         varchar(32)  NOT NULL,
  name            varchar(128) NOT NULL,
  site_base_url   text         NOT NULL,
  credential      text         NOT NULL DEFAULT '',
  credential_hint varchar(64)  NOT NULL DEFAULT '',
  login_account   varchar(190) NOT NULL DEFAULT '',
  config          jsonb        NOT NULL DEFAULT '{}'::jsonb,
  is_enabled      boolean      NOT NULL DEFAULT true,
  last_sync_at    timestamptz,
  last_status     varchar(16),
  last_error      text,
  created_at      timestamptz  NOT NULL DEFAULT now(),
  updated_at      timestamptz  NOT NULL DEFAULT now(),
  UNIQUE (provider, site_base_url)
);

CREATE TABLE IF NOT EXISTS upstream_sync_runs (
  id               bigserial PRIMARY KEY,
  source_id        bigint NOT NULL REFERENCES upstream_sync_sources(id) ON DELETE CASCADE,
  status           varchar(16) NOT NULL,
  groups_total     int NOT NULL DEFAULT 0,
  keys_created     int NOT NULL DEFAULT 0,
  keys_reused      int NOT NULL DEFAULT 0,
  accounts_created int NOT NULL DEFAULT 0,
  accounts_updated int NOT NULL DEFAULT 0,
  routes_bound     int NOT NULL DEFAULT 0,
  verify_passed    int NOT NULL DEFAULT 0,
  verify_failed    int NOT NULL DEFAULT 0,
  started_at       timestamptz NOT NULL DEFAULT now(),
  finished_at      timestamptz,
  summary          text
);

CREATE TABLE IF NOT EXISTS upstream_sync_run_items (
  id         bigserial PRIMARY KEY,
  run_id     bigint NOT NULL REFERENCES upstream_sync_runs(id) ON DELETE CASCADE,
  group_name varchar(128) NOT NULL,
  rate       numeric(12,6) NOT NULL DEFAULT 0,
  action     varchar(24) NOT NULL,
  message    text NOT NULL DEFAULT '',
  account_id bigint
);
CREATE INDEX IF NOT EXISTS idx_upstream_sync_items_run ON upstream_sync_run_items (run_id);

-- 上游模型价格监控与选路（P0）
--
-- 设计口径：
--   1. 只做手动同步：页面点「同步」触发，不建定时任务。
--   2. 实际成本 = 上游标价 × 该线路 Key 所属分组倍率 × 厂商充值/结算倍率；
--      美元站统一 usd_cny_rate（默认 6.75，可配置），人民币额度站用厂商 recharge_rate。
--   3. 价格快照保留 60 天，每次同步结束时清理。
--   4. 本迁移先把现有 model_routes.cost_rule 作为 source_type='config' 的初始价格，
--      保证页面第一次打开就有数据；后续由同步任务覆盖。
--
-- 回滚：见 179_upstream_price_monitor.down.sql

-- ========= 1. 厂商价格源 =========
CREATE TABLE IF NOT EXISTS upstream_providers (
  provider             varchar(64) PRIMARY KEY,
  display_name         varchar(128) NOT NULL DEFAULT '',
  source_type          varchar(32)  NOT NULL DEFAULT 'manual',
  price_endpoint       text         NOT NULL DEFAULT '',
  currency             varchar(8)   NOT NULL DEFAULT 'CNY',
  recharge_rate        numeric(12,4) NOT NULL DEFAULT 1,
  usd_cny_rate         numeric(12,4) NOT NULL DEFAULT 6.75,
  is_enabled           boolean      NOT NULL DEFAULT true,
  config               jsonb        NOT NULL DEFAULT '{}'::jsonb,
  last_sync_at         timestamptz,
  last_status          varchar(32)  NOT NULL DEFAULT 'never',
  consecutive_failures integer      NOT NULL DEFAULT 0,
  created_at           timestamptz  NOT NULL DEFAULT now(),
  updated_at           timestamptz  NOT NULL DEFAULT now()
);

INSERT INTO upstream_providers (provider, display_name)
SELECT DISTINCT provider, provider FROM model_routes WHERE provider <> ''
ON CONFLICT (provider) DO NOTHING;

UPDATE upstream_providers SET
  display_name = 'PackyCode', source_type = 'new_api',
  price_endpoint = 'https://www.packyapi.ai', currency = 'CNY',
  recharge_rate = 1, is_enabled = true, updated_at = now()
WHERE provider = 'packycode';

UPDATE upstream_providers SET
  display_name = 'OpenRouter', source_type = 'openrouter',
  price_endpoint = 'https://openrouter.ai/api/v1', currency = 'USD',
  recharge_rate = 1, usd_cny_rate = 6.75, is_enabled = true, updated_at = now()
WHERE provider = 'openrouter';

UPDATE upstream_providers SET display_name = 'CodexAuv', source_type = 'manual', currency = 'CNY', updated_at = now() WHERE provider = 'codexauv';
UPDATE upstream_providers SET display_name = '阿里云百炼', source_type = 'official_page', currency = 'CNY', is_enabled = true, updated_at = now() WHERE provider = 'bailian';
UPDATE upstream_providers SET display_name = '火山方舟', source_type = 'official_page', currency = 'CNY', is_enabled = true, updated_at = now() WHERE provider = 'volcano';
UPDATE upstream_providers SET display_name = '智谱 BigModel', source_type = 'official_page', currency = 'CNY', is_enabled = true, updated_at = now() WHERE provider = 'bigmodel';
UPDATE upstream_providers SET display_name = '腾讯云 TokenHub', source_type = 'official_page', currency = 'CNY', is_enabled = true, updated_at = now() WHERE provider = 'tokenhub';
UPDATE upstream_providers SET display_name = '本地 New API', source_type = 'manual', currency = 'CNY', updated_at = now() WHERE provider = 'new_api';
UPDATE upstream_providers SET display_name = 'Sync', source_type = 'manual', currency = 'CNY', updated_at = now() WHERE provider = 'sync';

-- ========= 2. 价格快照（不可变历史） =========
CREATE TABLE IF NOT EXISTS upstream_price_snapshots (
  id                    bigserial PRIMARY KEY,
  sync_run_id           bigint,
  provider              varchar(64)  NOT NULL,
  upstream_model        varchar(128) NOT NULL,
  group_name            varchar(128) NOT NULL DEFAULT '',
  billing_type          varchar(32)  NOT NULL,
  source_currency       varchar(8)   NOT NULL DEFAULT 'CNY',
  source_unit           varchar(32)  NOT NULL DEFAULT 'per_1m_tokens',
  input_price           numeric(18,8),
  output_price          numeric(18,8),
  cache_read_price      numeric(18,8),
  cache_write_price     numeric(18,8),
  unit_price            numeric(18,8),
  billing_multiplier    numeric(12,6) NOT NULL DEFAULT 1,
  fx_rate               numeric(12,6) NOT NULL DEFAULT 1,
  cny_input_per_m       numeric(18,8),
  cny_output_per_m      numeric(18,8),
  cny_cache_read_per_m  numeric(18,8),
  cny_cache_write_per_m numeric(18,8),
  cny_unit_price        numeric(18,8),
  source_url            text,
  source_type           varchar(32)  NOT NULL DEFAULT 'sync',
  is_trusted            boolean      NOT NULL DEFAULT true,
  observed_at           timestamptz  NOT NULL DEFAULT now(),
  raw                   jsonb        NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_upstream_price_snapshots_lookup
  ON upstream_price_snapshots (provider, upstream_model, observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_upstream_price_snapshots_run
  ON upstream_price_snapshots (sync_run_id);
CREATE INDEX IF NOT EXISTS idx_upstream_price_snapshots_time
  ON upstream_price_snapshots (observed_at);

-- ========= 3. 最新价（查询表） =========
CREATE TABLE IF NOT EXISTS upstream_price_current (
  provider              varchar(64)  NOT NULL,
  upstream_model        varchar(128) NOT NULL,
  group_name            varchar(128) NOT NULL DEFAULT '',
  snapshot_id           bigint,
  billing_type          varchar(32)  NOT NULL,
  source_currency       varchar(8)   NOT NULL DEFAULT 'CNY',
  source_unit           varchar(32)  NOT NULL DEFAULT 'per_1m_tokens',
  input_price           numeric(18,8),
  output_price          numeric(18,8),
  cache_read_price      numeric(18,8),
  cache_write_price     numeric(18,8),
  unit_price            numeric(18,8),
  billing_multiplier    numeric(12,6) NOT NULL DEFAULT 1,
  fx_rate               numeric(12,6) NOT NULL DEFAULT 1,
  cny_input_per_m       numeric(18,8),
  cny_output_per_m      numeric(18,8),
  cny_cache_read_per_m  numeric(18,8),
  cny_cache_write_per_m numeric(18,8),
  cny_unit_price        numeric(18,8),
  source_url            text,
  source_type           varchar(32)  NOT NULL DEFAULT 'sync',
  is_trusted            boolean      NOT NULL DEFAULT true,
  observed_at           timestamptz  NOT NULL DEFAULT now(),
  is_stale              boolean      NOT NULL DEFAULT false,
  PRIMARY KEY (provider, upstream_model, group_name)
);

CREATE INDEX IF NOT EXISTS idx_upstream_price_current_model
  ON upstream_price_current (upstream_model);

-- ========= 4. 上游名 → 内部模型映射 =========
CREATE TABLE IF NOT EXISTS upstream_model_mappings (
  id             bigserial PRIMARY KEY,
  provider       varchar(64)  NOT NULL,
  upstream_model varchar(128) NOT NULL,
  model_id       bigint REFERENCES models(id) ON DELETE CASCADE,
  match_method   varchar(32)  NOT NULL DEFAULT 'alias',
  confidence     numeric(4,3) NOT NULL DEFAULT 1,
  is_confirmed   boolean      NOT NULL DEFAULT false,
  created_at     timestamptz  NOT NULL DEFAULT now(),
  updated_at     timestamptz  NOT NULL DEFAULT now(),
  UNIQUE (provider, upstream_model)
);

-- 现有线路本身就是精确映射，直接回填为已确认。
INSERT INTO upstream_model_mappings (provider, upstream_model, model_id, match_method, confidence, is_confirmed)
SELECT DISTINCT ON (r.provider, r.upstream_model)
       r.provider, r.upstream_model, r.model_id, 'alias', 1, true
FROM model_routes r
WHERE r.provider <> '' AND r.upstream_model <> ''
ORDER BY r.provider, r.upstream_model, r.model_id
ON CONFLICT (provider, upstream_model) DO UPDATE
SET model_id = EXCLUDED.model_id,
    match_method = 'alias',
    confidence = 1,
    is_confirmed = true,
    updated_at = now();

-- ========= 5. 同步批次与厂商子结果 =========
CREATE TABLE IF NOT EXISTS price_sync_runs (
  id                 bigserial PRIMARY KEY,
  trigger            varchar(16) NOT NULL DEFAULT 'manual',
  status             varchar(16) NOT NULL DEFAULT 'running',
  provider_total     integer NOT NULL DEFAULT 0,
  provider_succeeded integer NOT NULL DEFAULT 0,
  provider_failed    integer NOT NULL DEFAULT 0,
  items_total        integer NOT NULL DEFAULT 0,
  items_matched      integer NOT NULL DEFAULT 0,
  error_summary      text,
  created_by         bigint,
  started_at         timestamptz NOT NULL DEFAULT now(),
  finished_at        timestamptz
);

CREATE TABLE IF NOT EXISTS price_sync_provider_runs (
  id          bigserial PRIMARY KEY,
  sync_run_id bigint NOT NULL REFERENCES price_sync_runs(id) ON DELETE CASCADE,
  provider    varchar(64) NOT NULL,
  status      varchar(16) NOT NULL DEFAULT 'running',
  http_status integer,
  items       integer NOT NULL DEFAULT 0,
  latency_ms  integer NOT NULL DEFAULT 0,
  attempts    integer NOT NULL DEFAULT 0,
  error       text,
  started_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_price_sync_provider_runs_run
  ON price_sync_provider_runs (sync_run_id);
CREATE INDEX IF NOT EXISTS idx_price_sync_runs_started
  ON price_sync_runs (started_at DESC);

-- ========= 6. 模型用量日汇总 =========
CREATE TABLE IF NOT EXISTS model_usage_daily (
  model_id          bigint NOT NULL REFERENCES models(id) ON DELETE CASCADE,
  day               date   NOT NULL,
  input_tokens      bigint NOT NULL DEFAULT 0,
  output_tokens     bigint NOT NULL DEFAULT 0,
  cache_read_tokens bigint NOT NULL DEFAULT 0,
  cache_write_tokens bigint NOT NULL DEFAULT 0,
  image_count       bigint NOT NULL DEFAULT 0,
  video_seconds     numeric(18,3) NOT NULL DEFAULT 0,
  request_count     bigint NOT NULL DEFAULT 0,
  provider_cost     numeric(18,8) NOT NULL DEFAULT 0,
  PRIMARY KEY (model_id, day)
);

INSERT INTO model_usage_daily (model_id, day, input_tokens, output_tokens, request_count, provider_cost)
SELECT model_id, date(created_at), COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0),
       COUNT(*), COALESCE(SUM(provider_cost),0)
FROM ai_call_logs
WHERE model_id IS NOT NULL AND status = 'success' AND created_at >= now() - interval '60 days'
GROUP BY model_id, date(created_at)
ON CONFLICT (model_id, day) DO UPDATE
SET input_tokens = EXCLUDED.input_tokens,
    output_tokens = EXCLUDED.output_tokens,
    request_count = EXCLUDED.request_count,
    provider_cost = EXCLUDED.provider_cost;

INSERT INTO model_usage_daily (model_id, day, image_count, video_seconds, request_count, provider_cost)
SELECT model_id, date(created_at),
       COUNT(*) FILTER (WHERE type = 'image'),
       COALESCE(SUM(CASE WHEN type = 'video' THEN COALESCE((input->>'target_duration_sec')::numeric, (input->>'duration')::numeric, 0) ELSE 0 END),0),
       COUNT(*), COALESCE(SUM(provider_cost),0)
FROM tasks
WHERE model_id IS NOT NULL AND status = 'succeeded' AND created_at >= now() - interval '60 days'
GROUP BY model_id, date(created_at)
ON CONFLICT (model_id, day) DO UPDATE
SET image_count = EXCLUDED.image_count,
    video_seconds = EXCLUDED.video_seconds,
    request_count = model_usage_daily.request_count + EXCLUDED.request_count,
    provider_cost = model_usage_daily.provider_cost + EXCLUDED.provider_cost;

-- ========= 7. 切主线路日志 =========
CREATE TABLE IF NOT EXISTS model_route_switch_logs (
  id                     bigserial PRIMARY KEY,
  model_id               bigint NOT NULL REFERENCES models(id) ON DELETE CASCADE,
  from_route_id          bigint,
  to_route_id            bigint NOT NULL,
  before_snapshot        jsonb NOT NULL DEFAULT '[]'::jsonb,
  after_snapshot         jsonb NOT NULL DEFAULT '[]'::jsonb,
  reason                 varchar(64) NOT NULL DEFAULT 'manual',
  estimated_daily_saving numeric(18,8) NOT NULL DEFAULT 0,
  estimated_monthly_saving numeric(18,8) NOT NULL DEFAULT 0,
  admin_id               bigint,
  created_at             timestamptz NOT NULL DEFAULT now(),
  rolled_back_at         timestamptz,
  rollback_of_id         bigint
);

CREATE INDEX IF NOT EXISTS idx_model_route_switch_logs_model
  ON model_route_switch_logs (model_id, created_at DESC);

-- ========= 8. 线路价格分组 =========
ALTER TABLE model_routes ADD COLUMN IF NOT EXISTS price_group varchar(128) NOT NULL DEFAULT '';

UPDATE model_routes r SET price_group = v.grp, updated_at = now()
FROM models m, (VALUES
  ('qwen3-8-flash','bailian'),
  ('qwen3-8-max-0902','bailian'),
  ('deepseek-v4-1-flash-1017','deepseek-sale'),
  ('deepseek-v4-pro-0813','deepseek-sale'),
  ('gpt-5-6-sol','azure-officially'),
  ('gpt-6-astra','azure-officially'),
  ('gpt-6-sol','azure-officially'),
  ('claude-sonnet-5','claude-officially'),
  ('claude-opus-5-5','claude-officially'),
  ('claude-opus-4-8','claude-officially'),
  ('claude-fable-5','claude-officially'),
  ('claude-fable-5-1','claude-officially'),
  ('glm-5-3-flash','glm-sale'),
  ('grok-imagine-image-2-0','grok-officially')
) AS v(code, grp)
WHERE r.model_id = m.id AND r.provider = 'packycode' AND m.code = v.code;

UPDATE model_routes r SET price_group = 'gemini-slb', updated_at = now()
FROM models m
WHERE r.model_id = m.id AND r.provider = 'packycode' AND m.code = 'gemini-3-8-flash' AND r.priority = 2;

UPDATE model_routes r SET price_group = 'gemini-officially', updated_at = now()
FROM models m
WHERE r.model_id = m.id AND r.provider = 'packycode' AND m.code IN ('gemini-3-8-flash','gemini-3-7-flash') AND r.priority <> 2;

-- ========= 9. 现有 cost_rule 作为初始价格 =========
WITH src AS (
  SELECT DISTINCT ON (r.provider, r.upstream_model, COALESCE(NULLIF(r.price_group,''), ''))
         r.provider, r.upstream_model, COALESCE(NULLIF(r.price_group,''), '') AS group_name,
         r.cost_rule, r.model_id, r.priority, r.id
  FROM model_routes r
  WHERE r.cost_rule <> '{}'::jsonb
  ORDER BY r.provider, r.upstream_model, COALESCE(NULLIF(r.price_group,''), ''), r.priority, r.id
), parsed AS (
  SELECT provider, upstream_model, group_name, model_id,
         COALESCE(NULLIF(cost_rule->>'billing_type',''), 'per_token') AS billing_type,
         COALESCE(NULLIF(cost_rule->>'source_currency',''), 'CNY') AS source_currency,
         CASE
           WHEN COALESCE(cost_rule->>'billing_type','per_token') = 'per_token' THEN 'per_1m_tokens'
           WHEN cost_rule->>'billing_type' = 'per_image' THEN 'per_image'
           WHEN cost_rule->>'billing_type' = 'per_second' THEN 'per_second'
           ELSE 'per_request'
         END AS source_unit,
         NULLIF(cost_rule->>'input_cost_per_m','')::numeric AS input_price,
         NULLIF(cost_rule->>'output_cost_per_m','')::numeric AS output_price,
         NULLIF(cost_rule->>'cache_read_cost_per_m','')::numeric AS cache_read_price,
         NULLIF(cost_rule->>'cache_write_cost_per_m','')::numeric AS cache_write_price,
         NULLIF(cost_rule->>'unit_cost','')::numeric AS unit_price,
         cost_rule
  FROM src
), ins AS (
  INSERT INTO upstream_price_snapshots
    (provider, upstream_model, group_name, billing_type, source_currency, source_unit,
     input_price, output_price, cache_read_price, cache_write_price, unit_price,
     billing_multiplier, fx_rate,
     cny_input_per_m, cny_output_per_m, cny_cache_read_per_m, cny_cache_write_per_m, cny_unit_price,
     source_url, source_type, is_trusted, observed_at, raw)
  SELECT provider, upstream_model, group_name, billing_type, source_currency, source_unit,
         input_price, output_price, cache_read_price, cache_write_price, unit_price,
         1, 1,
         input_price, output_price, cache_read_price, cache_write_price, unit_price,
         '', 'config', true, now(), cost_rule
  FROM parsed
  RETURNING id, provider, upstream_model, group_name, billing_type, source_currency, source_unit,
            input_price, output_price, cache_read_price, cache_write_price, unit_price,
            billing_multiplier, fx_rate,
            cny_input_per_m, cny_output_per_m, cny_cache_read_per_m, cny_cache_write_per_m, cny_unit_price,
            source_url, source_type, is_trusted, observed_at
)
INSERT INTO upstream_price_current
  (provider, upstream_model, group_name, snapshot_id, billing_type, source_currency, source_unit,
   input_price, output_price, cache_read_price, cache_write_price, unit_price,
   billing_multiplier, fx_rate,
   cny_input_per_m, cny_output_per_m, cny_cache_read_per_m, cny_cache_write_per_m, cny_unit_price,
   source_url, source_type, is_trusted, observed_at, is_stale)
SELECT provider, upstream_model, group_name, id, billing_type, source_currency, source_unit,
       input_price, output_price, cache_read_price, cache_write_price, unit_price,
       billing_multiplier, fx_rate,
       cny_input_per_m, cny_output_per_m, cny_cache_read_per_m, cny_cache_write_per_m, cny_unit_price,
       source_url, source_type, is_trusted, observed_at, false
FROM ins
ON CONFLICT (provider, upstream_model, group_name) DO UPDATE
SET snapshot_id = EXCLUDED.snapshot_id,
    billing_type = EXCLUDED.billing_type,
    source_currency = EXCLUDED.source_currency,
    source_unit = EXCLUDED.source_unit,
    input_price = EXCLUDED.input_price,
    output_price = EXCLUDED.output_price,
    cache_read_price = EXCLUDED.cache_read_price,
    cache_write_price = EXCLUDED.cache_write_price,
    unit_price = EXCLUDED.unit_price,
    billing_multiplier = EXCLUDED.billing_multiplier,
    fx_rate = EXCLUDED.fx_rate,
    cny_input_per_m = EXCLUDED.cny_input_per_m,
    cny_output_per_m = EXCLUDED.cny_output_per_m,
    cny_cache_read_per_m = EXCLUDED.cny_cache_read_per_m,
    cny_cache_write_per_m = EXCLUDED.cny_cache_write_per_m,
    cny_unit_price = EXCLUDED.cny_unit_price,
    source_url = EXCLUDED.source_url,
    source_type = EXCLUDED.source_type,
    is_trusted = EXCLUDED.is_trusted,
    observed_at = EXCLUDED.observed_at,
    is_stale = false;

-- ========= 10. 配置项 =========
INSERT INTO system_configs (key, value, updated_at) VALUES
  ('usd_cny_rate', '6.75'::jsonb, now()),
  ('price_sync_concurrency', '5'::jsonb, now()),
  ('price_sync_timeout_seconds', '60'::jsonb, now()),
  ('price_sync_retries', '2'::jsonb, now()),
  ('price_sync_stale_hours', '24'::jsonb, now()),
  ('price_gap_alert_threshold_percent', '20'::jsonb, now()),
  ('price_loss_margin_threshold_percent', '5'::jsonb, now()),
  ('price_snapshot_retention_days', '60'::jsonb, now()),
  ('price_sync_alert_email', '""'::jsonb, now())
ON CONFLICT (key) DO NOTHING;

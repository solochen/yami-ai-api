-- 用户售价规则（与上游成本解耦）+ DeepSeek 定价策略 + 价格漂移告警表。
--
-- 背景：DeepSeek 在腾讯云 TokenHub 是「官方价 + 峰时翻倍」，而 OpenRouter 报的
-- 是官方闲时价且全天不变（$0.66/$1.98 = ¥4.752/14.256）。因此上游统一走
-- OpenRouter（成本更低、无峰谷波动），用户售价按腾讯云峰时价计（不区分时段），
-- 毛利约 48.8%。售价与成本分离后，成本仍按上游真实价记账，毛利报表保持准确。

-- ========= 1. 新增 user_price_rule 列 =========
ALTER TABLE model_routes
  ADD COLUMN IF NOT EXISTS user_price_rule JSONB NOT NULL DEFAULT '{}'::jsonb;

-- ========= 2. DeepSeek 两条线路配置用户售价规则（峰时价）=========
UPDATE model_routes r
SET user_price_rule = '{"billing_type":"per_token","input_cost_per_m":9,"output_cost_per_m":27,"cache_read_cost_per_m":0.3}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id AND m.code = 'deepseek-v4-pro-0813';

UPDATE model_routes r
SET user_price_rule = '{"billing_type":"per_token","input_cost_per_m":2,"output_cost_per_m":8,"cache_read_cost_per_m":0.04}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id AND m.code = 'deepseek-v4-1-flash-1017';

-- ========= 3. 优先级：OpenRouter 优先（成本更低），腾讯云降为备份 =========
UPDATE model_routes r
SET priority = 20, updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND m.code IN ('deepseek-v4-pro-0813', 'deepseek-v4-1-flash-1017')
  AND r.provider = 'openrouter';

UPDATE model_routes r
SET priority = 100, updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND m.code IN ('deepseek-v4-pro-0813', 'deepseek-v4-1-flash-1017')
  AND r.provider = 'tokenhub';

-- ========= 4. 模型售价展示值同步为用户售价 =========
UPDATE models
SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":9,"output_price_per_m":27,"cache_read_price_per_m":0.3}'::jsonb,
    updated_at = now()
WHERE code = 'deepseek-v4-pro-0813';

UPDATE models
SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":2,"output_price_per_m":8,"cache_read_price_per_m":0.04}'::jsonb,
    updated_at = now()
WHERE code = 'deepseek-v4-1-flash-1017';

-- ========= 5. 价格漂移告警 =========
-- 售价与成本解耦后，上游涨价不会再自动反映到用户售价上，必须靠巡检发现。
CREATE TABLE IF NOT EXISTS price_drift_alerts (
  id                 BIGSERIAL PRIMARY KEY,
  model_code         VARCHAR(128)  NOT NULL,
  route_id           BIGINT,
  provider           VARCHAR(64)   NOT NULL,
  upstream_model     VARCHAR(128)  NOT NULL,
  field              VARCHAR(64)   NOT NULL,
  configured         NUMERIC(18,6) NOT NULL,
  observed           NUMERIC(18,6) NOT NULL,
  deviation_percent  NUMERIC(10,2) NOT NULL,
  observed_currency  VARCHAR(8)    NOT NULL DEFAULT 'USD',
  source_url         TEXT,
  created_at         TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS price_drift_alerts_created_at_idx ON price_drift_alerts (created_at DESC);

INSERT INTO system_configs (key, value, updated_at) VALUES
  ('price_drift_threshold_percent', '10'::jsonb, now()),
  ('price_drift_alert_email', '""'::jsonb, now())
ON CONFLICT (key) DO NOTHING;

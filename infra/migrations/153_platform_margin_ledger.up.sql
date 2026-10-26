-- Platform gross-margin billing.
--
-- User charges are derived from the actual upstream route cost:
--   user_charge = provider_cost / (1 - gross_margin)
-- The ledger stores the exact inputs and rounded amounts used by each
-- settlement so finance can audit margin independently from mutable route
-- configuration.

INSERT INTO system_configs (key, value, updated_at)
VALUES
  ('platform_gross_margin_percent', '3'::jsonb, now()),
  ('missing_cost_policy', '"block"'::jsonb, now()),
  ('billing_rounding_mode', '"ceil_6dp"'::jsonb, now())
ON CONFLICT (key) DO NOTHING;

ALTER TABLE model_route_attempts
  ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS billing_ref_type VARCHAR(32),
  ADD COLUMN IF NOT EXISTS billing_ref_id VARCHAR(64),
  ADD COLUMN IF NOT EXISTS workflow_project_id BIGINT REFERENCES workflow_projects(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS provider_cost_source VARCHAR(32) NOT NULL DEFAULT 'estimated',
  ADD COLUMN IF NOT EXISTS provider_cost_complete BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE workflow_projects
  ADD COLUMN IF NOT EXISTS provider_cost NUMERIC(18,6) NOT NULL DEFAULT 0;

ALTER TABLE tasks
  ADD COLUMN IF NOT EXISTS workflow_project_id BIGINT REFERENCES workflow_projects(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS provider_cost_source VARCHAR(32) NOT NULL DEFAULT 'missing_route_cost',
  ADD COLUMN IF NOT EXISTS provider_cost_complete BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_model_route_attempts_billing_ref
  ON model_route_attempts (billing_ref_type, billing_ref_id, id);
CREATE INDEX IF NOT EXISTS idx_model_route_attempts_workflow_project
  ON model_route_attempts (workflow_project_id, id);
CREATE INDEX IF NOT EXISTS idx_tasks_workflow_project
  ON tasks (workflow_project_id, id);
CREATE INDEX IF NOT EXISTS idx_tasks_provider_cost_state
  ON tasks (workflow_project_id, provider_cost_complete, id);

CREATE TABLE IF NOT EXISTS platform_billing_accounts (
  id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  gross_margin_percent NUMERIC(9,6) NOT NULL CHECK (gross_margin_percent >= 0 AND gross_margin_percent < 100),
  user_charge_compute NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (user_charge_compute >= 0),
  provider_cost_compute NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (provider_cost_compute >= 0),
  commission_compute NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (commission_compute >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT platform_billing_accounts_balanced
    CHECK (user_charge_compute >= provider_cost_compute)
);

INSERT INTO platform_billing_accounts (id, gross_margin_percent)
VALUES (
  1,
  COALESCE((
    SELECT CASE
      WHEN jsonb_typeof(value) = 'number'
       AND (value #>> '{}')::numeric >= 0
       AND (value #>> '{}')::numeric < 100
      THEN (value #>> '{}')::numeric
      ELSE 3
    END
    FROM system_configs WHERE key='platform_gross_margin_percent'
  ), 3)
)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS billing_settlements (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  model_id BIGINT REFERENCES models(id) ON DELETE SET NULL,
  route_id BIGINT REFERENCES model_routes(id) ON DELETE SET NULL,
  ref_type VARCHAR(32) NOT NULL,
  ref_id VARCHAR(64) NOT NULL,
  wallet_transaction_id BIGINT REFERENCES wallet_transactions(id),
  provider_cost_compute NUMERIC(18,6) NOT NULL CHECK (provider_cost_compute >= 0),
  user_charge_compute NUMERIC(18,6) NOT NULL CHECK (user_charge_compute >= 0),
  commission_compute NUMERIC(18,6) NOT NULL CHECK (commission_compute >= 0),
  gross_margin_percent NUMERIC(9,6) NOT NULL CHECK (gross_margin_percent >= 0 AND gross_margin_percent < 100),
  payment_compute_rate NUMERIC(18,6) NOT NULL CHECK (payment_compute_rate > 0),
  usd_to_compute_rate NUMERIC(18,6) NOT NULL CHECK (usd_to_compute_rate > 0),
  rounding_mode VARCHAR(32) NOT NULL DEFAULT 'ceil_6dp',
  cost_source VARCHAR(32) NOT NULL,
  cost_complete BOOLEAN NOT NULL DEFAULT false,
  settlement_key VARCHAR(128) NOT NULL UNIQUE,
  workflow_sequence INTEGER NOT NULL DEFAULT 0 CHECK (workflow_sequence >= 0),
  status VARCHAR(20) NOT NULL DEFAULT 'settled' CHECK (status IN ('settled','superseded','refunded')),
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT billing_settlements_balanced
    CHECK (user_charge_compute = provider_cost_compute + commission_compute),
  CONSTRAINT billing_settlements_charge_covers_cost
    CHECK (user_charge_compute >= provider_cost_compute)
);

CREATE INDEX IF NOT EXISTS idx_billing_settlements_user_time
  ON billing_settlements (user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_billing_settlements_ref
  ON billing_settlements (ref_type, ref_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_settlements_workflow_sequence
  ON billing_settlements (ref_type, ref_id, workflow_sequence)
  WHERE ref_type = 'workflow' AND status = 'settled';
CREATE INDEX IF NOT EXISTS idx_billing_settlements_model_time
  ON billing_settlements (model_id, created_at DESC);

CREATE TABLE IF NOT EXISTS billing_settlement_items (
  id BIGSERIAL PRIMARY KEY,
  settlement_id BIGINT NOT NULL REFERENCES billing_settlements(id) ON DELETE CASCADE,
  model_id BIGINT REFERENCES models(id) ON DELETE SET NULL,
  route_id BIGINT REFERENCES model_routes(id) ON DELETE SET NULL,
  source VARCHAR(32) NOT NULL,
  ref_type VARCHAR(32) NOT NULL DEFAULT 'settlement',
  ref_id VARCHAR(128) NOT NULL DEFAULT '',
  provider_cost_compute NUMERIC(18,6) NOT NULL CHECK (provider_cost_compute >= 0),
  user_charge_compute NUMERIC(18,6) NOT NULL CHECK (user_charge_compute >= 0),
  commission_compute NUMERIC(18,6) NOT NULL CHECK (commission_compute >= 0),
  prompt_tokens BIGINT NOT NULL DEFAULT 0,
  completion_tokens BIGINT NOT NULL DEFAULT 0,
  cached_input_tokens BIGINT NOT NULL DEFAULT 0,
  image_count INT NOT NULL DEFAULT 0,
  video_seconds NUMERIC(18,6) NOT NULL DEFAULT 0,
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT billing_settlement_items_balanced
    CHECK (user_charge_compute = provider_cost_compute + commission_compute)
);

CREATE INDEX IF NOT EXISTS idx_billing_settlement_items_settlement
  ON billing_settlement_items (settlement_id, id);
CREATE INDEX IF NOT EXISTS idx_billing_settlement_items_ref
  ON billing_settlement_items (ref_type, ref_id, id);

CREATE OR REPLACE VIEW platform_billing_totals AS
SELECT
  COALESCE(SUM(s.provider_cost_compute), 0) AS provider_cost_compute,
  COALESCE(SUM(s.user_charge_compute), 0) AS user_charge_compute,
  COALESCE(SUM(s.commission_compute), 0) AS commission_compute,
  CASE
    WHEN COALESCE(SUM(s.user_charge_compute), 0) > 0
    THEN COALESCE(SUM(s.commission_compute), 0) / SUM(s.user_charge_compute)
    ELSE 0
  END AS realized_margin_rate,
  COUNT(*) AS settlement_count,
  COUNT(*) FILTER (WHERE NOT s.cost_complete) AS incomplete_cost_count
FROM billing_settlements s
WHERE s.status = 'settled';

-- Stage 6: controlled batch generation for gallery case specifications.
-- These tables are an internal production ledger. They never publish directly
-- to gallery_items; successful output remains pending_review until a later
-- human review stage explicitly promotes it.

CREATE TABLE gallery_generation_batches (
  id BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(64) NOT NULL UNIQUE,
  status VARCHAR(24) NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft', 'queued', 'running', 'completed', 'partial_failed', 'failed', 'cancelled')),
  requested_by BIGINT REFERENCES admin_users(id) ON DELETE SET NULL,
  spec_count INT NOT NULL DEFAULT 0 CHECK (spec_count >= 0),
  task_count INT NOT NULL DEFAULT 0 CHECK (task_count >= 0),
  completed_count INT NOT NULL DEFAULT 0 CHECK (completed_count >= 0),
  failed_count INT NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
  cancelled_count INT NOT NULL DEFAULT 0 CHECK (cancelled_count >= 0),
  active_count INT NOT NULL DEFAULT 0 CHECK (active_count >= 0),
  max_concurrency INT NOT NULL DEFAULT 2 CHECK (max_concurrency BETWEEN 1 AND 20),
  max_retries INT NOT NULL DEFAULT 2 CHECK (max_retries BETWEEN 0 AND 5),
  budget_limit NUMERIC(18,8) NOT NULL DEFAULT 0 CHECK (budget_limit >= 0),
  estimated_cost NUMERIC(18,8) NOT NULL DEFAULT 0 CHECK (estimated_cost >= 0),
  actual_cost NUMERIC(18,8) NOT NULL DEFAULT 0 CHECK (actual_cost >= 0),
  currency VARCHAR(16) NOT NULL DEFAULT 'compute',
  cost_rule_version VARCHAR(64) NOT NULL DEFAULT 'model.price_rule.v1',
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  cancelled_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_gallery_generation_batches_status_time
  ON gallery_generation_batches (status, updated_at DESC);

CREATE TABLE gallery_generation_tasks (
  id BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(64) NOT NULL UNIQUE,
  batch_id BIGINT NOT NULL REFERENCES gallery_generation_batches(id) ON DELETE CASCADE,
  spec_id BIGINT NOT NULL REFERENCES gallery_case_specs(id) ON DELETE RESTRICT,
  attempt INT NOT NULL DEFAULT 1 CHECK (attempt > 0),
  status VARCHAR(24) NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued', 'submitting', 'processing', 'pending_review', 'succeeded', 'failed', 'cancelled', 'expired', 'unknown')),
  model_id BIGINT NOT NULL REFERENCES models(id) ON DELETE RESTRICT,
  model_code VARCHAR(128) NOT NULL,
  provider_model VARCHAR(256) NOT NULL DEFAULT '',
  request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  request_id VARCHAR(128) NOT NULL,
  upstream_task_id VARCHAR(256) NOT NULL DEFAULT '',
  output JSONB NOT NULL DEFAULT '{}'::jsonb,
  output_asset_url TEXT NOT NULL DEFAULT '',
  output_thumbnail_url TEXT NOT NULL DEFAULT '',
  estimated_cost NUMERIC(18,8) NOT NULL DEFAULT 0 CHECK (estimated_cost >= 0),
  actual_cost NUMERIC(18,8) NOT NULL DEFAULT 0 CHECK (actual_cost >= 0),
  provider_cost NUMERIC(18,8) NOT NULL DEFAULT 0 CHECK (provider_cost >= 0),
  retry_count INT NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
  max_retries INT NOT NULL DEFAULT 2 CHECK (max_retries BETWEEN 0 AND 5),
  error_code VARCHAR(64),
  error_message TEXT,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (batch_id, spec_id)
);

CREATE INDEX idx_gallery_generation_tasks_batch_status
  ON gallery_generation_tasks (batch_id, status, updated_at DESC);
CREATE INDEX idx_gallery_generation_tasks_status_time
  ON gallery_generation_tasks (status, updated_at);
CREATE INDEX idx_gallery_generation_tasks_request_id
  ON gallery_generation_tasks (request_id);
CREATE UNIQUE INDEX uq_gallery_generation_active_spec
  ON gallery_generation_tasks (spec_id)
  WHERE status IN ('queued', 'submitting', 'processing', 'pending_review');

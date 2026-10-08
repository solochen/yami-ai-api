-- Stage 5: gap tasks and versioned, reviewable case specifications.
-- These records are intentionally separate from public gallery_items and
-- gallery_reference_cases. Nothing here is visible to Web until a later
-- generation and quality-review stage creates a public catalog record.

CREATE TABLE gallery_gap_tasks (
  id BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(64) NOT NULL UNIQUE,
  fingerprint CHAR(64) NOT NULL UNIQUE,
  source_report_hash CHAR(64) NOT NULL,
  source_snapshot_at TIMESTAMPTZ,
  dimension_type VARCHAR(32) NOT NULL CHECK (dimension_type IN ('category', 'style', 'scene', 'category_scene', 'category_style', 'scene_language', 'custom')),
  category VARCHAR(128) NOT NULL DEFAULT '',
  style VARCHAR(128) NOT NULL DEFAULT '',
  scene VARCHAR(128) NOT NULL DEFAULT '',
  prompt_language VARCHAR(32) NOT NULL DEFAULT '',
  target_count INT NOT NULL CHECK (target_count > 0),
  current_count INT NOT NULL DEFAULT 0 CHECK (current_count >= 0),
  gap_count INT NOT NULL CHECK (gap_count >= 0),
  priority VARCHAR(16) NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'critical')),
  title VARCHAR(256) NOT NULL,
  rationale TEXT NOT NULL DEFAULT '',
  risk_flags JSONB NOT NULL DEFAULT '[]'::jsonb,
  source_context JSONB NOT NULL DEFAULT '{}'::jsonb,
  status VARCHAR(16) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'fulfilled', 'dismissed')),
  created_by BIGINT REFERENCES admin_users(id) ON DELETE SET NULL,
  updated_by BIGINT REFERENCES admin_users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_gallery_gap_tasks_status_priority
  ON gallery_gap_tasks (status, priority DESC, gap_count DESC, updated_at DESC);

CREATE TABLE gallery_case_specs (
  id BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(64) NOT NULL UNIQUE,
  gap_task_id BIGINT NOT NULL REFERENCES gallery_gap_tasks(id) ON DELETE RESTRICT,
  version INT NOT NULL DEFAULT 1 CHECK (version > 0),
  status VARCHAR(16) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'ready', 'archived')),
  title VARCHAR(256) NOT NULL,
  purpose TEXT NOT NULL DEFAULT '',
  prompt TEXT NOT NULL,
  negative_prompt TEXT NOT NULL DEFAULT '',
  prompt_language VARCHAR(32) NOT NULL,
  image_text_languages JSONB NOT NULL DEFAULT '[]'::jsonb,
  expected_text JSONB NOT NULL DEFAULT '[]'::jsonb,
  category VARCHAR(128) NOT NULL,
  styles JSONB NOT NULL DEFAULT '[]'::jsonb,
  scenes JSONB NOT NULL DEFAULT '[]'::jsonb,
  aspect_ratio VARCHAR(16) NOT NULL,
  output_media_type VARCHAR(16) NOT NULL DEFAULT 'image' CHECK (output_media_type IN ('image')),
  candidate_models JSONB NOT NULL DEFAULT '[]'::jsonb,
  acceptance_criteria JSONB NOT NULL DEFAULT '[]'::jsonb,
  selection_reason TEXT NOT NULL DEFAULT '',
  risk_flags JSONB NOT NULL DEFAULT '[]'::jsonb,
  risk_notes TEXT NOT NULL DEFAULT '',
  risk_review_status VARCHAR(16) NOT NULL DEFAULT 'not_required' CHECK (risk_review_status IN ('not_required', 'pending', 'approved', 'rejected')),
  generator_type VARCHAR(16) NOT NULL DEFAULT 'manual' CHECK (generator_type IN ('ai', 'template', 'manual', 'codex')),
  generator_model_code VARCHAR(128) NOT NULL DEFAULT '',
  generator_prompt_version VARCHAR(64) NOT NULL DEFAULT '',
  generation_request_id VARCHAR(128) NOT NULL DEFAULT '',
  generation_input JSONB NOT NULL DEFAULT '{}'::jsonb,
  generation_output JSONB NOT NULL DEFAULT '{}'::jsonb,
  estimated_cost NUMERIC(18,8) NOT NULL DEFAULT 0 CHECK (estimated_cost >= 0),
  actual_cost NUMERIC(18,8) NOT NULL DEFAULT 0 CHECK (actual_cost >= 0),
  input_tokens INT NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
  output_tokens INT NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
  validation_errors JSONB NOT NULL DEFAULT '[]'::jsonb,
  content_hash CHAR(64) NOT NULL,
  created_by BIGINT REFERENCES admin_users(id) ON DELETE SET NULL,
  updated_by BIGINT REFERENCES admin_users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (length(btrim(title)) > 0),
  CHECK (length(btrim(prompt)) > 0),
  CHECK (length(btrim(prompt_language)) > 0),
  CHECK (length(btrim(category)) > 0),
  CHECK (length(btrim(aspect_ratio)) > 0)
);

CREATE INDEX idx_gallery_case_specs_task_status
  ON gallery_case_specs (gap_task_id, status, updated_at DESC);
CREATE INDEX idx_gallery_case_specs_status
  ON gallery_case_specs (status, updated_at DESC);
CREATE INDEX idx_gallery_case_specs_hash
  ON gallery_case_specs (content_hash);

CREATE TABLE gallery_case_spec_versions (
  spec_id BIGINT NOT NULL REFERENCES gallery_case_specs(id) ON DELETE CASCADE,
  version INT NOT NULL CHECK (version > 0),
  change_type VARCHAR(32) NOT NULL CHECK (change_type IN ('created', 'generated', 'edited', 'ready', 'archived')),
  snapshot JSONB NOT NULL,
  changed_by BIGINT REFERENCES admin_users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (spec_id, version)
);

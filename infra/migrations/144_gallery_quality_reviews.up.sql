-- Stage 7: quality review and explicit publication for generated gallery cases.
-- A review is separate from gallery_items so a generated result cannot become
-- public merely because a worker finished successfully.

ALTER TABLE gallery_items
  ADD COLUMN generation_task_id BIGINT REFERENCES gallery_generation_tasks(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX uq_gallery_items_generation_task
  ON gallery_items (generation_task_id)
  WHERE generation_task_id IS NOT NULL;

CREATE TABLE gallery_quality_reviews (
  id BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(64) NOT NULL UNIQUE,
  task_id BIGINT NOT NULL UNIQUE REFERENCES gallery_generation_tasks(id) ON DELETE CASCADE,
  spec_id BIGINT NOT NULL REFERENCES gallery_case_specs(id) ON DELETE RESTRICT,
  status VARCHAR(24) NOT NULL DEFAULT 'pending_review'
    CHECK (status IN ('pending_review', 'auto_checking', 'auto_failed', 'awaiting_manual_review', 'approved', 'rejected', 'published')),
  review_version INT NOT NULL DEFAULT 1 CHECK (review_version > 0),
  asset_check JSONB NOT NULL DEFAULT '{}'::jsonb,
  metadata_check JSONB NOT NULL DEFAULT '{}'::jsonb,
  ocr_check JSONB NOT NULL DEFAULT '{}'::jsonb,
  risk_check JSONB NOT NULL DEFAULT '{}'::jsonb,
  duplicate_check JSONB NOT NULL DEFAULT '{}'::jsonb,
  check_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
  output_content_hash CHAR(64) NOT NULL DEFAULT '',
  prompt_hash CHAR(64) NOT NULL DEFAULT '',
  reviewer_id BIGINT REFERENCES admin_users(id) ON DELETE SET NULL,
  review_notes TEXT NOT NULL DEFAULT '',
  rejection_reason TEXT NOT NULL DEFAULT '',
  reviewed_at TIMESTAMPTZ,
  gallery_item_id BIGINT REFERENCES gallery_items(id) ON DELETE SET NULL,
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_gallery_quality_reviews_status_time
  ON gallery_quality_reviews (status, updated_at DESC);
CREATE INDEX idx_gallery_quality_reviews_spec
  ON gallery_quality_reviews (spec_id, updated_at DESC);
CREATE INDEX idx_gallery_quality_reviews_content_hash
  ON gallery_quality_reviews (output_content_hash)
  WHERE output_content_hash <> '';
CREATE INDEX idx_gallery_quality_reviews_prompt_hash
  ON gallery_quality_reviews (prompt_hash)
  WHERE prompt_hash <> '';

INSERT INTO gallery_quality_reviews (public_id, task_id, spec_id)
SELECT 'g_review_' || md5(t.id::text || clock_timestamp()::text), t.id, t.spec_id
FROM gallery_generation_tasks t
WHERE t.status = 'pending_review'
ON CONFLICT (task_id) DO NOTHING;

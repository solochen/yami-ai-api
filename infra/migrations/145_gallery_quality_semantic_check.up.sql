ALTER TABLE gallery_quality_reviews
  ADD COLUMN semantic_check JSONB NOT NULL DEFAULT '{}'::jsonb;

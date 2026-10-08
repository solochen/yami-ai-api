-- Local snapshot catalog for licensed external inspiration-gallery sources.
-- This is intentionally separate from gallery_items, which backs community
-- submissions, paid reuse, and existing financial records.

CREATE TABLE gallery_reference_sources (
  id BIGSERIAL PRIMARY KEY,
  source_key VARCHAR(160) NOT NULL UNIQUE,
  repository_url TEXT NOT NULL,
  manifest_url TEXT NOT NULL,
  license_name VARCHAR(128) NOT NULL,
  license_url TEXT NOT NULL,
  attribution TEXT NOT NULL,
  sync_status VARCHAR(24) NOT NULL DEFAULT 'never' CHECK (sync_status IN ('never', 'running', 'succeeded', 'failed')),
  manifest_hash CHAR(64),
  case_count INT NOT NULL DEFAULT 0 CHECK (case_count >= 0),
  last_sync_error TEXT,
  last_synced_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE gallery_reference_cases (
  id BIGSERIAL PRIMARY KEY,
  source_id BIGINT NOT NULL REFERENCES gallery_reference_sources(id) ON DELETE CASCADE,
  source_item_id VARCHAR(128) NOT NULL,
  content_hash CHAR(64) NOT NULL,
  title VARCHAR(256) NOT NULL,
  image_source_url TEXT NOT NULL,
  image_asset_url TEXT,
  image_alt TEXT,
  source_url TEXT,
  github_url TEXT,
  prompt TEXT NOT NULL,
  prompt_preview TEXT,
  category VARCHAR(128) NOT NULL,
  styles JSONB NOT NULL DEFAULT '[]'::jsonb,
  scenes JSONB NOT NULL DEFAULT '[]'::jsonb,
  featured BOOLEAN NOT NULL DEFAULT false,
  asset_license_status VARCHAR(24) NOT NULL DEFAULT 'unverified' CHECK (asset_license_status IN ('unverified', 'verified', 'rejected')),
  status VARCHAR(24) NOT NULL DEFAULT 'pending_license' CHECK (status IN ('pending_license', 'published', 'hidden')),
  is_active BOOLEAN NOT NULL DEFAULT true,
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (source_id, source_item_id)
);

CREATE INDEX idx_gallery_reference_cases_active
  ON gallery_reference_cases (source_id, is_active, status, featured DESC, id);
CREATE INDEX idx_gallery_reference_cases_category
  ON gallery_reference_cases (category, is_active, status);
CREATE INDEX idx_gallery_reference_cases_content_hash
  ON gallery_reference_cases (content_hash);

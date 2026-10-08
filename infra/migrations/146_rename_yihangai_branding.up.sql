-- Rename the public product brand while preserving internal compatibility identifiers.
UPDATE system_configs
SET value = to_jsonb(regexp_replace(value #>> '{}', 'StarAI', 'YiHangAI', 'gi')),
    updated_at = now()
WHERE jsonb_typeof(value) = 'string'
  AND key IN (
    'site_name', 'site_copyright', 'home_meta_title',
    'home_meta_description', 'site_description', 'admin_site_description',
    'site_api_tagline', 'payment_product_name', 'paypal_brand_name',
    'customer_service_name', 'resend_from'
  )
  AND (value #>> '{}') ~* 'StarAI';

UPDATE system_configs
SET value = to_jsonb(regexp_replace(value #>> '{}', 'starai-logo-mark[.]svg', 'yihangai-logo.png', 'gi')),
    updated_at = now()
WHERE key IN ('site_logo', 'site_favicon')
  AND jsonb_typeof(value) = 'string'
  AND (value #>> '{}') ~* 'starai-logo-mark\.svg';

UPDATE api_docs
SET base_url = regexp_replace(base_url, 'starai', 'yihangai', 'gi'),
    updated_at = now()
WHERE base_url ~* 'starai';

UPDATE announcements
SET title = regexp_replace(title, 'StarAI', 'YiHangAI', 'gi'),
    content = regexp_replace(content, 'StarAI', 'YiHangAI', 'gi'),
    updated_at = now()
WHERE title ~* 'StarAI' OR content ~* 'StarAI';

UPDATE content_translation_sources
SET source_text = regexp_replace(source_text, 'StarAI', 'YiHangAI', 'gi'),
    updated_at = now()
WHERE source_text ~* 'StarAI';

UPDATE content_translations
SET value = regexp_replace(value, 'StarAI', 'YiHangAI', 'gi'),
    updated_at = now()
WHERE value ~* 'StarAI';

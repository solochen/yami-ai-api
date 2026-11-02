-- 回滚 169：把相对媒体路径还原为本地绝对地址。
-- 说明：仅还原备份表登记过的核心展示字段；其余 JSONB 历史快照按同样规则整体回滚。

UPDATE gallery_reference_cases SET image_asset_url = replace(image_asset_url, '/media/', 'http://localhost:9000/')
WHERE image_asset_url LIKE '/media/%';

UPDATE gallery_items SET cover_url = replace(cover_url, '/media/', 'http://localhost:9000/')
WHERE cover_url LIKE '/media/%';

UPDATE models SET icon_url = replace(icon_url, '/media/', 'http://localhost:9000/')
WHERE icon_url LIKE '/media/%';

UPDATE works SET thumbnail_url = replace(thumbnail_url, '/media/', 'http://localhost:9000/')
WHERE thumbnail_url LIKE '/media/%';

UPDATE gallery_generation_tasks SET
  output_asset_url = replace(output_asset_url, '/media/', 'http://localhost:9000/'),
  output_thumbnail_url = replace(output_thumbnail_url, '/media/', 'http://localhost:9000/')
WHERE output_asset_url LIKE '/media/%' OR output_thumbnail_url LIKE '/media/%';

UPDATE gallery_generation_tasks SET output = replace(output::text, '/media/', 'http://localhost:9000/')::jsonb
WHERE output::text LIKE '%/media/%';

UPDATE tasks SET input = replace(input::text, '/media/', 'http://localhost:9000/')::jsonb
WHERE input::text LIKE '%/media/%';

UPDATE tasks SET output = replace(output::text, '/media/', 'http://localhost:9000/')::jsonb
WHERE output::text LIKE '%/media/%';

UPDATE task_events SET payload = replace(payload::text, '/media/', 'http://localhost:9000/')::jsonb
WHERE payload::text LIKE '%/media/%';

UPDATE works SET metadata = replace(metadata::text, '/media/', 'http://localhost:9000/')::jsonb
WHERE metadata::text LIKE '%/media/%';

UPDATE workflow_node_runs SET input = replace(input::text, '/media/', 'http://localhost:9000/')::jsonb
WHERE input::text LIKE '%/media/%';

UPDATE workflow_node_runs SET output = replace(output::text, '/media/', 'http://localhost:9000/')::jsonb
WHERE output::text LIKE '%/media/%';

UPDATE workflow_projects SET inputs = replace(inputs::text, '/media/', 'http://localhost:9000/')::jsonb
WHERE inputs::text LIKE '%/media/%';

UPDATE workflow_projects SET outputs = replace(outputs::text, '/media/', 'http://localhost:9000/')::jsonb
WHERE outputs::text LIKE '%/media/%';

UPDATE gallery_quality_reviews SET asset_check = replace(asset_check::text, '/media/', 'http://localhost:9000/')::jsonb
WHERE asset_check::text LIKE '%/media/%';

UPDATE system_configs
SET value = to_jsonb(replace(value #>> '{}', '/media/', 'http://localhost:9000/'))
WHERE value::text LIKE '%/media/%'
  AND key IN ('site_favicon', 'site_logo', 'customer_service_avatar', 'customer_service_qr_url');

DROP TABLE IF EXISTS media_url_relative_backup_169;

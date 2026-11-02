-- 媒体地址改为同源相对路径：http://localhost:9000/<bucket>/<key>  →  /media/<bucket>/<key>
--
-- 背景：此前库里存的是浏览器侧绝对地址（MINIO_PUBLIC_URL=http://localhost:9000）。
-- Next.js 的图片优化器在服务端抓取该地址，而服务端在容器内，localhost 指向容器自己，
-- 于是 ECONNREFUSED、优化失败、图片全裂。改为相对路径后：
--   * 浏览器与图片优化器都只访问站点自身的 /media/<bucket>/<key>；
--   * 由 web/admin 的 /media 路由代理到对象存储（容器内用 minio:9000）；
--   * 对象存储无需对外暴露，生产换域名时也不必再改库。
--
-- 覆盖范围：全库 text/varchar/jsonb 列中所有 http://localhost:9000 前缀（备份表除外）。
-- 回滚：见 169_media_relative_urls.down.sql（把 /media/ 还原为原绝对前缀）。

CREATE TABLE IF NOT EXISTS media_url_relative_backup_169 AS
SELECT 'gallery_reference_cases.image_asset_url' AS target, id::text AS row_id, image_asset_url AS value
FROM gallery_reference_cases WHERE image_asset_url LIKE 'http://localhost:9000/%'
UNION ALL SELECT 'gallery_items.cover_url', id::text, cover_url FROM gallery_items WHERE cover_url LIKE 'http://localhost:9000/%'
UNION ALL SELECT 'models.icon_url', id::text, icon_url FROM models WHERE icon_url LIKE 'http://localhost:9000/%'
UNION ALL SELECT 'works.thumbnail_url', id::text, thumbnail_url FROM works WHERE thumbnail_url LIKE 'http://localhost:9000/%';

-- ========= 1. 文本列 =========
UPDATE gallery_reference_cases SET image_asset_url = replace(image_asset_url, 'http://localhost:9000/', '/media/')
WHERE image_asset_url LIKE 'http://localhost:9000/%';

UPDATE gallery_items SET cover_url = replace(cover_url, 'http://localhost:9000/', '/media/')
WHERE cover_url LIKE 'http://localhost:9000/%';

UPDATE models SET icon_url = replace(icon_url, 'http://localhost:9000/', '/media/')
WHERE icon_url LIKE 'http://localhost:9000/%';

UPDATE works SET thumbnail_url = replace(thumbnail_url, 'http://localhost:9000/', '/media/')
WHERE thumbnail_url LIKE 'http://localhost:9000/%';

UPDATE gallery_generation_tasks
SET output_asset_url = replace(output_asset_url, 'http://localhost:9000/', '/media/'),
    output_thumbnail_url = replace(output_thumbnail_url, 'http://localhost:9000/', '/media/')
WHERE output_asset_url LIKE 'http://localhost:9000/%' OR output_thumbnail_url LIKE 'http://localhost:9000/%';

-- ========= 2. JSONB 列（文本整体替换后回转 jsonb，URL 中的斜杠不会破坏 JSON 结构）=========
UPDATE gallery_generation_tasks SET output = replace(output::text, 'http://localhost:9000/', '/media/')::jsonb
WHERE output::text LIKE '%http://localhost:9000/%';

UPDATE tasks SET input = replace(input::text, 'http://localhost:9000/', '/media/')::jsonb
WHERE input::text LIKE '%http://localhost:9000/%';

UPDATE tasks SET output = replace(output::text, 'http://localhost:9000/', '/media/')::jsonb
WHERE output::text LIKE '%http://localhost:9000/%';

UPDATE task_events SET payload = replace(payload::text, 'http://localhost:9000/', '/media/')::jsonb
WHERE payload::text LIKE '%http://localhost:9000/%';

UPDATE works SET metadata = replace(metadata::text, 'http://localhost:9000/', '/media/')::jsonb
WHERE metadata::text LIKE '%http://localhost:9000/%';

UPDATE workflow_node_runs SET input = replace(input::text, 'http://localhost:9000/', '/media/')::jsonb
WHERE input::text LIKE '%http://localhost:9000/%';

UPDATE workflow_node_runs SET output = replace(output::text, 'http://localhost:9000/', '/media/')::jsonb
WHERE output::text LIKE '%http://localhost:9000/%';

UPDATE workflow_projects SET inputs = replace(inputs::text, 'http://localhost:9000/', '/media/')::jsonb
WHERE inputs::text LIKE '%http://localhost:9000/%';

UPDATE workflow_projects SET outputs = replace(outputs::text, 'http://localhost:9000/', '/media/')::jsonb
WHERE outputs::text LIKE '%http://localhost:9000/%';

UPDATE gallery_quality_reviews SET asset_check = replace(asset_check::text, 'http://localhost:9000/', '/media/')::jsonb
WHERE asset_check::text LIKE '%http://localhost:9000/%';

-- ========= 3. 站点品牌资源（Logo / 客服头像 / 二维码 / favicon）=========
UPDATE system_configs
SET value = to_jsonb(replace(value #>> '{}', 'http://localhost:9000/', '/media/'))
WHERE value::text LIKE '%http://localhost:9000/%'
  AND key IN ('site_favicon', 'site_logo', 'customer_service_avatar', 'customer_service_qr_url');

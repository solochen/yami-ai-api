-- 境外模型代号化的配套改动：同步 API 文档的标题、摘要、特性标签与请求示例。
--
-- 文档正文（content）由模型元数据派生，其中 request_example.model 会直接展示给
-- 调用方，因此必须一并切到对外别名，否则用户照抄示例仍会看到原厂模型编码。
-- 协议说明中的 "OpenAI 风格 Authorization Bearer 鉴权" 属于接口兼容性描述，保持不变。
--
-- 回滚：见 168_api_docs_vendor_branding.down.sql

CREATE TABLE IF NOT EXISTS api_docs_vendor_branding_backup_168 AS
SELECT d.id, d.slug, d.title, d.summary, d.content, now() AS backed_up_at
FROM api_docs d
JOIN models m ON m.id = d.model_id
WHERE m.public_alias IS NOT NULL;

-- ========= 1. 模型特性标签：把原厂名换成系列代称 =========
UPDATE api_docs d
SET content = jsonb_set(d.content, '{features}', m.tags, true),
    updated_at = now()
FROM models m
WHERE m.id = d.model_id
  AND m.public_alias IS NOT NULL
  AND jsonb_typeof(d.content -> 'features') = 'array';

-- ========= 2. 请求示例里的 model 字段：内部编码 → 对外别名 =========
UPDATE api_docs d
SET content = jsonb_set(d.content, '{request_example,model}', to_jsonb(m.public_alias::text), true),
    updated_at = now()
FROM models m
WHERE m.id = d.model_id
  AND m.public_alias IS NOT NULL
  AND d.content -> 'request_example' -> 'model' IS NOT NULL;

-- ========= 3. 标题与摘要：与 models 表保持同源 =========
UPDATE api_docs d
SET title = m.display_name,
    summary = m.description,
    updated_at = now()
FROM models m
WHERE m.id = d.model_id
  AND m.public_alias IS NOT NULL;

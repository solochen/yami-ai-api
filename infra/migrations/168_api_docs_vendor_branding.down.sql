-- 回滚 168：恢复 API 文档原始标题、摘要与正文。

UPDATE api_docs d
SET title = b.title,
    summary = b.summary,
    content = b.content,
    updated_at = now()
FROM api_docs_vendor_branding_backup_168 b
WHERE d.id = b.id;

DROP TABLE IF EXISTS api_docs_vendor_branding_backup_168;

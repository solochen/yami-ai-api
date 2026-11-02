-- 回滚 167：恢复境外模型原始展示字段，移除 public_alias。

-- ========= 1. 从备份表恢复展示字段 =========
UPDATE models m
SET display_name = b.display_name,
    description  = b.description,
    tags         = b.tags,
    icon_url     = b.icon_url,
    updated_at   = now()
FROM models_vendor_branding_backup_167 b
WHERE m.id = b.id;

-- ========= 2. 移除别名列与索引 =========
DROP INDEX IF EXISTS idx_models_public_alias;
ALTER TABLE models DROP COLUMN IF EXISTS public_alias;

-- ========= 3. 清理备份表 =========
DROP TABLE IF EXISTS models_vendor_branding_backup_167;

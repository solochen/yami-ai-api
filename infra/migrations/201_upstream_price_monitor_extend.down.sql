-- 201 回滚：移除七牛/aiboli 同步源与映射。
DELETE FROM upstream_model_mappings WHERE provider IN ('qiniu','aiboli');
DELETE FROM upstream_providers WHERE provider IN ('qiniu','aiboli');
-- 回滚 159：移除腾讯云 TokenHub 线路。
DELETE FROM model_routes WHERE provider = 'tokenhub';

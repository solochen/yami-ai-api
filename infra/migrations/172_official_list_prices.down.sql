-- 回滚 172：恢复各线路原 user_price_rule。
UPDATE model_routes r SET user_price_rule = b.old_rule, updated_at = now()
FROM official_list_price_backup_172 b WHERE r.id = b.route_id;
DROP TABLE IF EXISTS official_list_price_backup_172;

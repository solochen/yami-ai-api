-- 回滚 195：恢复模型与线路原价，并恢复写真馆原有固定费。
UPDATE models m
SET price_rule = b.old_price_rule,
    updated_at = now()
FROM openai_image_official_pricing_backup_195 b
WHERE m.id = b.model_id;

UPDATE model_routes r
SET user_price_rule = COALESCE(old_route.rule->'user_price_rule', '{}'::jsonb),
    updated_at = now()
FROM openai_image_official_pricing_backup_195 b
CROSS JOIN LATERAL jsonb_array_elements(b.old_route_rules) AS old_route(rule)
WHERE r.id = (old_route.rule->>'route_id')::bigint;

UPDATE workflow_definitions workflow
SET price_rule = backup.old_price_rule,
    updated_at = now()
FROM openai_image_official_workflow_backup_195 backup
WHERE workflow.code = backup.workflow_code;

DROP TABLE IF EXISTS openai_image_official_workflow_backup_195;
DROP TABLE IF EXISTS openai_image_official_pricing_backup_195;

-- Loss-capable ledger constraints remain relaxed so historical provider costs
-- and negative margin entries stay truthful and remain updateable.

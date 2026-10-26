-- Restore the pre-credits model, route, and workflow pricing snapshot.
-- Rows created after migration 152 keep their current configuration.
UPDATE models m
SET price_rule = b.price_rule
FROM model_price_credit_migration_backup_152 b
WHERE m.id = b.model_id;

UPDATE model_routes r
SET cost_rule = (b.route_costs->>r.id::text)::jsonb
FROM model_price_credit_migration_backup_152 b
WHERE r.model_id = b.model_id
  AND b.route_costs ? r.id::text;

UPDATE workflow_definitions w
SET price_rule = b.price_rule
FROM workflow_price_credit_migration_backup_152 b
WHERE w.id = b.workflow_id;

DELETE FROM system_configs current
USING system_config_price_credit_migration_backup_152 b
WHERE current.key = b.key
  AND NOT b.existed;

UPDATE system_configs current
SET value = b.value, updated_at = now()
FROM system_config_price_credit_migration_backup_152 b
WHERE current.key = b.key
  AND b.existed;

DROP TABLE IF EXISTS model_price_credit_migration_backup_152;
DROP TABLE IF EXISTS workflow_price_credit_migration_backup_152;
DROP TABLE IF EXISTS system_config_price_credit_migration_backup_152;

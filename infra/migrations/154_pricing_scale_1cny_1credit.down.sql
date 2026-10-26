-- Reverse migration 154: restore the 1 CNY = 1000 credits unit from the
-- backup table captured by the up migration.
UPDATE system_configs cfg
SET value = bak.payload, updated_at = now()
FROM pricing_scale_migration_backup_154 bak
WHERE bak.kind = 'config' AND cfg.key = bak.ref;

UPDATE models m
SET price_rule = bak.payload, updated_at = now()
FROM pricing_scale_migration_backup_154 bak
WHERE bak.kind = 'model' AND m.id::text = bak.ref;

UPDATE model_routes r
SET cost_rule = bak.payload, updated_at = now()
FROM pricing_scale_migration_backup_154 bak
WHERE bak.kind = 'route' AND r.id::text = bak.ref;

UPDATE workflow_definitions w
SET price_rule = bak.payload, updated_at = now()
FROM pricing_scale_migration_backup_154 bak
WHERE bak.kind = 'workflow' AND w.id::text = bak.ref;

UPDATE wallets wl
SET compute_balance = (bak.payload #>> '{}')::numeric
FROM pricing_scale_migration_backup_154 bak
WHERE bak.kind = 'wallet' AND wl.user_id::text = bak.ref;

UPDATE wallet_transactions t
SET amount = (bak.payload #>> '{}')::numeric
FROM pricing_scale_migration_backup_154 bak
WHERE bak.kind = 'wallet_tx' AND t.id::text = bak.ref;

UPDATE balance_freezes f
SET amount = (bak.payload #>> '{}')::numeric
FROM pricing_scale_migration_backup_154 bak
WHERE bak.kind = 'freeze' AND f.id::text = bak.ref;

DROP TABLE IF EXISTS pricing_scale_migration_backup_154;

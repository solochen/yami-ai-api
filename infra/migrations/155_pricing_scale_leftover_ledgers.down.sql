-- Reverse migration 155 from its backup table.
UPDATE tasks t
SET estimated_cost = (b.payload->>'estimated_cost')::numeric,
    actual_cost = (b.payload->>'actual_cost')::numeric,
    updated_at = now()
FROM pricing_scale_leftover_backup_155 b
WHERE b.kind = 'task' AND t.id::text = b.ref;

UPDATE workflow_projects w
SET estimated_cost = (b.payload->>'estimated_cost')::numeric,
    actual_cost = (b.payload->>'actual_cost')::numeric,
    updated_at = now()
FROM pricing_scale_leftover_backup_155 b
WHERE b.kind = 'workflow_project' AND w.id::text = b.ref;

UPDATE orders o
SET compute_credited = (b.payload->>'compute_credited')::numeric,
    updated_at = now()
FROM pricing_scale_leftover_backup_155 b
WHERE b.kind = 'order' AND o.id::text = b.ref;

UPDATE payment_packages p
SET compute_credits = NULLIF((b.payload->>'compute_credits'), '')::numeric,
    updated_at = now()
FROM pricing_scale_leftover_backup_155 b
WHERE b.kind = 'package' AND p.id::text = b.ref;

DROP TABLE IF EXISTS pricing_scale_leftover_backup_155;

-- Rescale the remaining credit-denominated historical records to the
-- "1 CNY = 1 credit" unit introduced by 154. Production carried no rows in
-- these tables, but environments with history still need their amounts divided
-- so the whole ledger reads in one unit.
--
-- provider_cost columns (model_route_attempts, tasks.provider_cost) store raw
-- upstream USD and are intentionally NOT touched. orders.amount is CNY cash.

-- ========= backup (kept for 155 down) =========
DROP TABLE IF EXISTS pricing_scale_leftover_backup_155;
CREATE TABLE pricing_scale_leftover_backup_155 AS
SELECT 'task'::text AS kind, t.id::text AS ref,
       jsonb_build_object('estimated_cost', t.estimated_cost, 'actual_cost', t.actual_cost) AS payload
FROM tasks t
WHERE t.estimated_cost <> 0 OR t.actual_cost <> 0
UNION ALL
SELECT 'workflow_project', w.id::text,
       jsonb_build_object('estimated_cost', w.estimated_cost, 'actual_cost', w.actual_cost)
FROM workflow_projects w
WHERE w.estimated_cost <> 0 OR w.actual_cost <> 0
UNION ALL
SELECT 'order', o.id::text, jsonb_build_object('compute_credited', o.compute_credited)
FROM orders o
WHERE o.compute_credited <> 0
UNION ALL
SELECT 'package', p.id::text, jsonb_build_object('compute_credits', p.compute_credits)
FROM payment_packages p
WHERE p.compute_credits IS NOT NULL AND p.compute_credits <> 0;

-- ========= historical task and workflow costs =========
UPDATE tasks
SET estimated_cost = estimated_cost / 1000,
    actual_cost = actual_cost / 1000,
    updated_at = now()
WHERE estimated_cost <> 0 OR actual_cost <> 0;

UPDATE workflow_projects
SET estimated_cost = estimated_cost / 1000,
    actual_cost = actual_cost / 1000,
    updated_at = now()
WHERE estimated_cost <> 0 OR actual_cost <> 0;

-- ========= historical credited compute on paid orders =========
UPDATE orders
SET compute_credited = compute_credited / 1000,
    updated_at = now()
WHERE compute_credited <> 0;

-- ========= recharge packages with manually pinned credits =========
-- NULL credits are computed live from payment_compute_rate, which is already
-- rescaled; only pinned values need conversion.
UPDATE payment_packages
SET compute_credits = compute_credits / 1000,
    updated_at = now()
WHERE compute_credits IS NOT NULL AND compute_credits <> 0;

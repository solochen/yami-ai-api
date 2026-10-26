DROP VIEW IF EXISTS platform_billing_totals;
DROP INDEX IF EXISTS idx_billing_settlement_items_ref;
DROP INDEX IF EXISTS idx_billing_settlements_workflow_sequence;
DROP TABLE IF EXISTS billing_settlement_items;
DROP TABLE IF EXISTS billing_settlements;
DROP TABLE IF EXISTS platform_billing_accounts;

DROP INDEX IF EXISTS idx_model_route_attempts_workflow_project;
DROP INDEX IF EXISTS idx_model_route_attempts_billing_ref;
DROP INDEX IF EXISTS idx_tasks_workflow_project;

ALTER TABLE tasks
  DROP COLUMN IF EXISTS provider_cost_complete,
  DROP COLUMN IF EXISTS provider_cost_source,
  DROP COLUMN IF EXISTS workflow_project_id;

ALTER TABLE model_route_attempts
  DROP COLUMN IF EXISTS provider_cost_complete,
  DROP COLUMN IF EXISTS provider_cost_source,
  DROP COLUMN IF EXISTS workflow_project_id,
  DROP COLUMN IF EXISTS billing_ref_id,
  DROP COLUMN IF EXISTS billing_ref_type,
  DROP COLUMN IF EXISTS user_id;

ALTER TABLE workflow_projects
  DROP COLUMN IF EXISTS provider_cost;

DELETE FROM system_configs
WHERE key IN (
  'platform_gross_margin_percent',
  'missing_cost_policy',
  'billing_rounding_mode'
);

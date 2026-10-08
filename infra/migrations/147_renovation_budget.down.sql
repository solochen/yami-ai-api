-- 回滚装修预算助理 Agent
DELETE FROM workflow_definitions WHERE code = 'renovation_budget_agent';
DROP TABLE IF EXISTS renovation_knowledge;
DROP TABLE IF EXISTS renovation_materials;

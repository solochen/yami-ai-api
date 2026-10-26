-- 回滚 157：清掉由模型价格回填出来的聊天线路成本（保留原本已配置的成本）。
UPDATE model_routes r
SET cost_rule = '{}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND r.is_enabled
  AND m.category = 'chat'
  AND r.cost_rule->>'price_unit_normalized' = 'true'
  AND r.cost_rule ? 'input_cost_per_m'
  AND NOT (r.cost_rule ? 'source_input_cost_per_m')
  AND NOT (r.cost_rule ? 'compute_per_cny')
  AND NOT (r.cost_rule ? 'usd_to_compute');

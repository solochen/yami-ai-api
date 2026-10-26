-- 回填聊天模型缺失的上游路由成本。
--
-- 新计费链路（platform_pricing.go）以「路由上游成本」为唯一计费依据，而
-- missing_cost_policy = 'block' 会让没有任何可计费线路的模型直接报
-- 「上游成本缺失」并停止计费。25 条聊天线路里只有 2 条（BigModel 直连）
-- 带成本，其余 23 条 OpenRouter 线路的 cost_rule 为空或全 0，部署新代码后
-- 这些聊天模型将无法估价与扣费。
--
-- 这些模型的 models.price_rule 保存的就是导入时的上游价（已完成 152/154
-- 归一化，单位算力、1 算力 = 1 元），因此按字段一一对应回填即可。
-- 仅回填「该模型没有任何带成本线路」的线路，避免覆盖已配置的真实成本
-- （例如 GLM 已有 BigModel 直连成本，其 OpenRouter 线路保持未配置，
-- 由 block 策略自动跳过）。

UPDATE model_routes r
SET cost_rule = jsonb_strip_nulls(jsonb_build_object(
      'billing_type', 'per_token',
      'currency', 'CREDITS',
      'price_unit_normalized', true,
      'input_cost_per_m', m.price_rule->'input_price_per_m',
      'output_cost_per_m', m.price_rule->'output_price_per_m',
      'cache_read_cost_per_m', m.price_rule->'cache_read_price_per_m',
      'cache_write_cost_per_m', m.price_rule->'cache_write_price_per_m',
      'audio_input_cost_per_m', m.price_rule->'audio_input_price_per_m',
      'audio_output_cost_per_m', m.price_rule->'audio_output_price_per_m'
    )),
    updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND r.is_enabled
  AND m.category = 'chat'
  AND m.price_rule->>'billing_type' = 'per_token'
  AND COALESCE(m.price_rule->>'input_price_per_m', '') ~ '^[0-9]+([.][0-9]+)?$'
  AND (m.price_rule->>'input_price_per_m')::numeric > 0
  AND NOT EXISTS (
    SELECT 1 FROM model_routes other
    WHERE other.model_id = r.model_id
      AND other.is_enabled
      AND (
        COALESCE(other.cost_rule->>'input_cost_per_m', '0')::numeric > 0
        OR COALESCE(other.cost_rule->>'output_cost_per_m', '0')::numeric > 0
      )
  );

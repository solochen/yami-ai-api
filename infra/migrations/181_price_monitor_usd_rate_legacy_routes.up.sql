-- 补齐 180 未覆盖的 15 条美元线路（cost_rule 缺少 source_currency/usd_to_compute 元数据）。
-- 这些线路来自 OpenRouter / 本地 new_api，历史成本按 7.2 记账；统一改为 6.75。
-- 只影响之后计费，不回溯 ai_call_logs。

UPDATE model_routes r
SET cost_rule = r.cost_rule || jsonb_build_object(
  'source_currency','USD',
  'usd_to_compute',6.75,
  'input_cost_per_m',  COALESCE((r.cost_rule->>'input_cost_per_m')::numeric,0)  * 0.9375,
  'output_cost_per_m', COALESCE((r.cost_rule->>'output_cost_per_m')::numeric,0) * 0.9375,
  'cache_read_cost_per_m',  COALESCE((r.cost_rule->>'cache_read_cost_per_m')::numeric,0)  * 0.9375,
  'cache_write_cost_per_m', COALESCE((r.cost_rule->>'cache_write_cost_per_m')::numeric,0) * 0.9375,
  'audio_input_cost_per_m',  COALESCE((r.cost_rule->>'audio_input_cost_per_m')::numeric,0)  * 0.9375,
  'audio_output_cost_per_m', COALESCE((r.cost_rule->>'audio_output_cost_per_m')::numeric,0) * 0.9375,
  'unit_cost', COALESCE((r.cost_rule->>'unit_cost')::numeric,0) * 0.9375
), updated_at=now()
FROM models m
WHERE r.model_id=m.id
  AND r.provider IN ('openrouter','new_api')
  AND m.code IN (
    'claude-fable-5','claude-fable-5-1','claude-opus-4-8','claude-opus-5-5','claude-sonnet-5',
    'gemini-3-7-flash','glm-5-3-prime','qwen3-8-flash','qwen3-8-max-0902','qwen3-8-omni-flash','qwen3-8-max-prime',
    'gpt-6-luna','gpt-5-6-luna','gpt-6-astra','gpt-6-sol'
  )
  AND COALESCE(r.cost_rule->>'source_currency','') <> 'USD';

UPDATE upstream_price_snapshots s
SET source_currency='USD', fx_rate=6.75,
    cny_input_per_m = COALESCE(s.cny_input_per_m,0) * 0.9375,
    cny_output_per_m = COALESCE(s.cny_output_per_m,0) * 0.9375,
    cny_cache_read_per_m = COALESCE(s.cny_cache_read_per_m,0) * 0.9375,
    cny_cache_write_per_m = COALESCE(s.cny_cache_write_per_m,0) * 0.9375,
    cny_unit_price = COALESCE(s.cny_unit_price,0) * 0.9375
WHERE s.source_type='config' AND s.fx_rate=1 AND s.provider IN ('openrouter','new_api')
  AND s.upstream_model IN (
    SELECT r.upstream_model FROM model_routes r JOIN models m ON m.id=r.model_id
    WHERE r.provider=s.provider AND m.code IN (
      'claude-fable-5','claude-fable-5-1','claude-opus-4-8','claude-opus-5-5','claude-sonnet-5',
      'gemini-3-7-flash','glm-5-3-prime','qwen3-8-flash','qwen3-8-max-0902','qwen3-8-omni-flash','qwen3-8-max-prime',
      'gpt-6-luna','gpt-5-6-luna','gpt-6-astra','gpt-6-sol'
    )
  );

UPDATE upstream_price_current c
SET source_currency='USD', fx_rate=6.75,
    cny_input_per_m = COALESCE(c.cny_input_per_m,0) * 0.9375,
    cny_output_per_m = COALESCE(c.cny_output_per_m,0) * 0.9375,
    cny_cache_read_per_m = COALESCE(c.cny_cache_read_per_m,0) * 0.9375,
    cny_cache_write_per_m = COALESCE(c.cny_cache_write_per_m,0) * 0.9375,
    cny_unit_price = COALESCE(c.cny_unit_price,0) * 0.9375
WHERE c.source_type='config' AND c.fx_rate=1 AND c.provider IN ('openrouter','new_api')
  AND c.upstream_model IN (
    SELECT r.upstream_model FROM model_routes r JOIN models m ON m.id=r.model_id
    WHERE r.provider=c.provider AND m.code IN (
      'claude-fable-5','claude-fable-5-1','claude-opus-4-8','claude-opus-5-5','claude-sonnet-5',
      'gemini-3-7-flash','glm-5-3-prime','qwen3-8-flash','qwen3-8-max-0902','qwen3-8-omni-flash','qwen3-8-max-prime',
      'gpt-6-luna','gpt-5-6-luna','gpt-6-astra','gpt-6-sol'
    )
  );

-- 回滚 181：15 条 legacy 美元线路恢复 7.2。
UPDATE model_routes r
SET cost_rule = r.cost_rule || jsonb_build_object(
  'usd_to_compute',7.2,
  'input_cost_per_m',  COALESCE((r.cost_rule->>'input_cost_per_m')::numeric,0)  * 1.0666666666666667,
  'output_cost_per_m', COALESCE((r.cost_rule->>'output_cost_per_m')::numeric,0) * 1.0666666666666667,
  'cache_read_cost_per_m',  COALESCE((r.cost_rule->>'cache_read_cost_per_m')::numeric,0)  * 1.0666666666666667,
  'cache_write_cost_per_m', COALESCE((r.cost_rule->>'cache_write_cost_per_m')::numeric,0) * 1.0666666666666667,
  'audio_input_cost_per_m',  COALESCE((r.cost_rule->>'audio_input_cost_per_m')::numeric,0)  * 1.0666666666666667,
  'audio_output_cost_per_m', COALESCE((r.cost_rule->>'audio_output_cost_per_m')::numeric,0) * 1.0666666666666667,
  'unit_cost', COALESCE((r.cost_rule->>'unit_cost')::numeric,0) * 1.0666666666666667
), updated_at=now()
FROM models m
WHERE r.model_id=m.id AND r.provider IN ('openrouter','new_api')
  AND m.code IN (
    'claude-fable-5','claude-fable-5-1','claude-opus-4-8','claude-opus-5-5','claude-sonnet-5',
    'gemini-3-7-flash','glm-5-3-prime','qwen3-8-flash','qwen3-8-max-0902','qwen3-8-omni-flash','qwen3-8-max-prime',
    'gpt-6-luna','gpt-5-6-luna','gpt-6-astra','gpt-6-sol'
  );

UPDATE upstream_price_snapshots s
SET fx_rate=7.2,
    cny_input_per_m = COALESCE(s.cny_input_per_m,0) * 1.0666666666666667,
    cny_output_per_m = COALESCE(s.cny_output_per_m,0) * 1.0666666666666667,
    cny_cache_read_per_m = COALESCE(s.cny_cache_read_per_m,0) * 1.0666666666666667,
    cny_cache_write_per_m = COALESCE(s.cny_cache_write_per_m,0) * 1.0666666666666667,
    cny_unit_price = COALESCE(s.cny_unit_price,0) * 1.0666666666666667
WHERE s.source_type='config' AND s.fx_rate=6.75 AND s.provider IN ('openrouter','new_api');

UPDATE upstream_price_current c
SET fx_rate=7.2,
    cny_input_per_m = COALESCE(c.cny_input_per_m,0) * 1.0666666666666667,
    cny_output_per_m = COALESCE(c.cny_output_per_m,0) * 1.0666666666666667,
    cny_cache_read_per_m = COALESCE(c.cny_cache_read_per_m,0) * 1.0666666666666667,
    cny_cache_write_per_m = COALESCE(c.cny_cache_write_per_m,0) * 1.0666666666666667,
    cny_unit_price = COALESCE(c.cny_unit_price,0) * 1.0666666666666667
WHERE c.source_type='config' AND c.fx_rate=6.75 AND c.provider IN ('openrouter','new_api');

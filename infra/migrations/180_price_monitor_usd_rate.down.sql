-- 回滚 180：美元线路记账汇率恢复 7.2，移除 PackyCode 可信分组白名单。
-- 只回滚 cost_rule 与 config 快照；真实同步产生的快照保留原观测值。

UPDATE model_routes
SET cost_rule = (
  cost_rule
  || jsonb_build_object(
    'usd_to_compute', 7.2,
    'input_cost_per_m',  COALESCE((cost_rule->>'input_cost_per_m')::numeric,0)  * 1.0666666666666667,
    'output_cost_per_m', COALESCE((cost_rule->>'output_cost_per_m')::numeric,0) * 1.0666666666666667,
    'cache_read_cost_per_m',  COALESCE((cost_rule->>'cache_read_cost_per_m')::numeric,0)  * 1.0666666666666667,
    'cache_write_cost_per_m', COALESCE((cost_rule->>'cache_write_cost_per_m')::numeric,0) * 1.0666666666666667,
    'audio_input_cost_per_m',  COALESCE((cost_rule->>'audio_input_cost_per_m')::numeric,0)  * 1.0666666666666667,
    'audio_output_cost_per_m', COALESCE((cost_rule->>'audio_output_cost_per_m')::numeric,0) * 1.0666666666666667,
    'unit_cost', COALESCE((cost_rule->>'unit_cost')::numeric,0) * 1.0666666666666667
  )
) || jsonb_build_object(
  'unit_cost_by_size', COALESCE((
    SELECT jsonb_object_agg(key, (value::text)::numeric * 1.0666666666666667)
    FROM jsonb_each_text(cost_rule->'unit_cost_by_size')
  ), '{}'::jsonb),
  'unit_cost_by_variant', COALESCE((
    SELECT jsonb_object_agg(key, (value::text)::numeric * 1.0666666666666667)
    FROM jsonb_each_text(cost_rule->'unit_cost_by_variant')
  ), '{}'::jsonb),
  'unit_cost_by_resolution', COALESCE((
    SELECT jsonb_object_agg(key, (value::text)::numeric * 1.0666666666666667)
    FROM jsonb_each_text(cost_rule->'unit_cost_by_resolution')
  ), '{}'::jsonb)
), updated_at=now()
WHERE (cost_rule->>'usd_to_compute')::numeric = 6.75;

UPDATE upstream_price_snapshots
SET fx_rate = 7.2,
    cny_input_per_m = COALESCE(cny_input_per_m,0) * 1.0666666666666667,
    cny_output_per_m = COALESCE(cny_output_per_m,0) * 1.0666666666666667,
    cny_cache_read_per_m = COALESCE(cny_cache_read_per_m,0) * 1.0666666666666667,
    cny_cache_write_per_m = COALESCE(cny_cache_write_per_m,0) * 1.0666666666666667,
    cny_unit_price = COALESCE(cny_unit_price,0) * 1.0666666666666667
WHERE source_type='config' AND source_currency='USD';

UPDATE upstream_price_current
SET fx_rate = 7.2,
    cny_input_per_m = COALESCE(cny_input_per_m,0) * 1.0666666666666667,
    cny_output_per_m = COALESCE(cny_output_per_m,0) * 1.0666666666666667,
    cny_cache_read_per_m = COALESCE(cny_cache_read_per_m,0) * 1.0666666666666667,
    cny_cache_write_per_m = COALESCE(cny_cache_write_per_m,0) * 1.0666666666666667,
    cny_unit_price = COALESCE(cny_unit_price,0) * 1.0666666666666667
WHERE source_type='config' AND source_currency='USD';

UPDATE upstream_providers SET usd_cny_rate=7.2, updated_at=now() WHERE currency='USD';

UPDATE upstream_providers
SET config = config - 'trusted_groups', updated_at=now()
WHERE provider='packycode';

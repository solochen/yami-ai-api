-- 价格监控口径修正：
--   1. PackyCode 只把官方/官转分组视为可信，cc-sale / kiro / award 等号池组不参与推荐。
--   2. 美元线路内部记账汇率从 7.2 统一为 6.75（仅影响之后计费，不回溯 ai_call_logs 历史）。
--   3. 同步修正 P0 初始化的 config 价格快照，避免页面第一次打开仍显示 7.2 口径。
--
-- 注意：这是一次财务口径变更，毛利率会因美元成本下降 6.25% 而上升。
-- 回滚：见 180_price_monitor_usd_rate.down.sql

-- ========= 1. PackyCode 可信分组 =========
UPDATE upstream_providers
SET config = COALESCE(config,'{}'::jsonb) || jsonb_build_object(
  'trusted_groups', jsonb_build_array(
    'bailian','deepseek-sale','azure-officially','gemini-slb','gemini-officially',
    'claude-officially','glm-sale','grok-officially'
  )
), updated_at=now()
WHERE provider='packycode';

-- ========= 2. 美元线路 cost_rule：7.2 → 6.75 =========
-- 6.75 / 7.2 = 0.9375
UPDATE model_routes
SET cost_rule = (
  cost_rule
  || jsonb_build_object(
    'usd_to_compute', 6.75,
    'input_cost_per_m',  COALESCE((cost_rule->>'input_cost_per_m')::numeric,0)  * 0.9375,
    'output_cost_per_m', COALESCE((cost_rule->>'output_cost_per_m')::numeric,0) * 0.9375,
    'cache_read_cost_per_m',  COALESCE((cost_rule->>'cache_read_cost_per_m')::numeric,0)  * 0.9375,
    'cache_write_cost_per_m', COALESCE((cost_rule->>'cache_write_cost_per_m')::numeric,0) * 0.9375,
    'audio_input_cost_per_m',  COALESCE((cost_rule->>'audio_input_cost_per_m')::numeric,0)  * 0.9375,
    'audio_output_cost_per_m', COALESCE((cost_rule->>'audio_output_cost_per_m')::numeric,0) * 0.9375,
    'unit_cost', COALESCE((cost_rule->>'unit_cost')::numeric,0) * 0.9375
  )
) || jsonb_build_object(
  'unit_cost_by_size', COALESCE((
    SELECT jsonb_object_agg(key, (value::text)::numeric * 0.9375)
    FROM jsonb_each_text(cost_rule->'unit_cost_by_size')
  ), '{}'::jsonb),
  'unit_cost_by_variant', COALESCE((
    SELECT jsonb_object_agg(key, (value::text)::numeric * 0.9375)
    FROM jsonb_each_text(cost_rule->'unit_cost_by_variant')
  ), '{}'::jsonb),
  'unit_cost_by_resolution', COALESCE((
    SELECT jsonb_object_agg(key, (value::text)::numeric * 0.9375)
    FROM jsonb_each_text(cost_rule->'unit_cost_by_resolution')
  ), '{}'::jsonb)
), updated_at=now()
WHERE (cost_rule->>'usd_to_compute')::numeric = 7.2;

-- ========= 3. 同步修正初始 config 价格快照与当前价 =========
UPDATE upstream_price_snapshots
SET fx_rate = 6.75,
    cny_input_per_m = COALESCE(cny_input_per_m,0) * 0.9375,
    cny_output_per_m = COALESCE(cny_output_per_m,0) * 0.9375,
    cny_cache_read_per_m = COALESCE(cny_cache_read_per_m,0) * 0.9375,
    cny_cache_write_per_m = COALESCE(cny_cache_write_per_m,0) * 0.9375,
    cny_unit_price = COALESCE(cny_unit_price,0) * 0.9375
WHERE source_type='config' AND source_currency='USD'
  AND provider IN (SELECT provider FROM upstream_providers WHERE currency='USD');

UPDATE upstream_price_current
SET fx_rate = 6.75,
    cny_input_per_m = COALESCE(cny_input_per_m,0) * 0.9375,
    cny_output_per_m = COALESCE(cny_output_per_m,0) * 0.9375,
    cny_cache_read_per_m = COALESCE(cny_cache_read_per_m,0) * 0.9375,
    cny_cache_write_per_m = COALESCE(cny_cache_write_per_m,0) * 0.9375,
    cny_unit_price = COALESCE(cny_unit_price,0) * 0.9375
WHERE source_type='config' AND source_currency='USD'
  AND provider IN (SELECT provider FROM upstream_providers WHERE currency='USD');

UPDATE upstream_providers SET usd_cny_rate=6.75, updated_at=now() WHERE currency='USD';

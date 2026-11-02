-- 把海外模型的用户售价从「官方美元价 × 6.7」对齐到「× 6.75」，与上游成本口径一致。
-- 只处理海外模型；DeepSeek / Qwen / GLM / Seedream 等国产模型的人民币售价不动。
-- 6.75 / 6.7 = 1.0074626865671642

UPDATE model_routes r
SET user_price_rule = (
  r.user_price_rule
  || jsonb_build_object(
    'input_cost_per_m',  COALESCE((r.user_price_rule->>'input_cost_per_m')::numeric,0)  * 1.0074626865671642,
    'output_cost_per_m', COALESCE((r.user_price_rule->>'output_cost_per_m')::numeric,0) * 1.0074626865671642,
    'cache_read_cost_per_m',  COALESCE((r.user_price_rule->>'cache_read_cost_per_m')::numeric,0)  * 1.0074626865671642,
    'cache_write_cost_per_m', COALESCE((r.user_price_rule->>'cache_write_cost_per_m')::numeric,0) * 1.0074626865671642,
    'audio_input_cost_per_m',  COALESCE((r.user_price_rule->>'audio_input_cost_per_m')::numeric,0)  * 1.0074626865671642,
    'audio_output_cost_per_m', COALESCE((r.user_price_rule->>'audio_output_cost_per_m')::numeric,0) * 1.0074626865671642,
    'unit_cost', COALESCE((r.user_price_rule->>'unit_cost')::numeric,0) * 1.0074626865671642
  )
) || jsonb_build_object(
  'unit_cost_by_size', COALESCE((
    SELECT jsonb_object_agg(key, (value::text)::numeric * 1.0074626865671642)
    FROM jsonb_each_text(r.user_price_rule->'unit_cost_by_size')
  ), '{}'::jsonb),
  'unit_cost_by_variant', COALESCE((
    SELECT jsonb_object_agg(key, (value::text)::numeric * 1.0074626865671642)
    FROM jsonb_each_text(r.user_price_rule->'unit_cost_by_variant')
  ), '{}'::jsonb),
  'unit_cost_by_resolution', COALESCE((
    SELECT jsonb_object_agg(key, (value::text)::numeric * 1.0074626865671642)
    FROM jsonb_each_text(r.user_price_rule->'unit_cost_by_resolution')
  ), '{}'::jsonb)
), updated_at=now()
FROM models m
WHERE r.model_id=m.id
  AND r.user_price_rule <> '{}'::jsonb
  AND m.code IN (
    'audio_openrouter_gpt_audio','audio_openrouter_gpt_audio_mini',
    'audio_openrouter_lyria_3_clip','audio_openrouter_lyria_3_pro',
    'claude-fable-5','claude-fable-5-1','claude-opus-4-8','claude-opus-5-5','claude-sonnet-5',
    'gemini-2-5-flash-image','gemini-3-7-flash','gemini-3-8-flash','gemini-3-pro-image',
    'gen-4-5','gpt-5-6-luna','gpt-5-6-sol','gpt-6-astra','gpt-6-luna','gpt-6-sol',
    'gpt-image-2','gpt-image-2-5-flare','gpt-image-2-5-sunburst',
    'grok-imagine-image-2-0','grok-imagine-video','grok-imagine-video-1-5',
    'hailuo-2-3','hailuo-3','hailuo-3-max','happyhorse-1-0','happyhorse-1-1',
    'kling-v3-0-pro','kling-v3-0-std','kling-video-o1',
    'veo-3-1','veo-3-1-fast','veo-3-1-lite'
  );

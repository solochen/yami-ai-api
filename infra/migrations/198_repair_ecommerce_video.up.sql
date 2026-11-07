-- 电商带货短视频仍绑着已下线的演示模型（chat_demo_v1 / image_fast_v1 / video_demo_v1），
-- 并且按次只冻结 0.45 积分。这里换成当前启用且有线路的模型，预估改为按节点模型用量冻结。
-- 已由管理员换成有效模型的节点保持不变。

CREATE TABLE IF NOT EXISTS ecommerce_video_repair_198_backup AS
SELECT id, nodes, runtime_config, price_rule, display_config
FROM workflow_definitions
WHERE code = 'ecommerce_video';

WITH workflow AS (
  SELECT id, nodes, runtime_config
  FROM workflow_definitions
  WHERE code = 'ecommerce_video'
), copy_model AS (
  SELECT COALESCE(
    (SELECT m.code
       FROM models m, workflow w
      WHERE m.code = NULLIF((SELECT n->>'model_code' FROM jsonb_array_elements(w.nodes) n WHERE n->>'id' = 'copy' LIMIT 1), '')
        AND m.code NOT IN ('chat_demo_v1', 'image_fast_v1', 'video_demo_v1')
        AND m.is_enabled AND m.category = 'chat'
        AND m.request_mode IN ('chat_completions', 'responses')
        AND COALESCE(m.runtime_rule->'capabilities'->>'image_input', 'false') = 'true'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.code = 'glm-5-3-flash'
        AND m.is_enabled AND m.category = 'chat'
        AND COALESCE(m.runtime_rule->'capabilities'->>'image_input', 'false') = 'true'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.is_enabled AND m.category = 'chat'
        AND m.request_mode IN ('chat_completions', 'responses')
        AND m.code <> 'multi_collab_chat'
        AND COALESCE(m.runtime_rule->'capabilities'->>'image_input', 'false') = 'true'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      ORDER BY m.sort_order, m.id
      LIMIT 1)
  ) AS code
), image_model AS (
  SELECT COALESCE(
    (SELECT m.code
       FROM models m, workflow w
      WHERE m.code = NULLIF((SELECT n->>'model_code' FROM jsonb_array_elements(w.nodes) n WHERE n->>'id' = 'poster' LIMIT 1), '')
        AND m.code NOT IN ('chat_demo_v1', 'image_fast_v1', 'video_demo_v1')
        AND m.is_enabled AND m.category = 'image' AND m.request_mode IN ('images', 'image')
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.code IN ('seedream-5-0-pro', 'seedream-4-5', 'seedream-5-0-lite')
        AND m.is_enabled AND m.category = 'image' AND m.request_mode IN ('images', 'image')
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      ORDER BY array_position(ARRAY['seedream-5-0-pro', 'seedream-4-5', 'seedream-5-0-lite'], m.code)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.is_enabled AND m.category = 'image' AND m.request_mode IN ('images', 'image')
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      ORDER BY m.sort_order, m.id
      LIMIT 1)
  ) AS code
), video_model AS (
  SELECT COALESCE(
    (SELECT m.code
       FROM models m, workflow w
      WHERE m.code = NULLIF((SELECT n->>'model_code' FROM jsonb_array_elements(w.nodes) n WHERE n->>'id' = 'video' LIMIT 1), '')
        AND m.code NOT IN ('chat_demo_v1', 'image_fast_v1', 'video_demo_v1')
        AND m.is_enabled AND m.category = 'video' AND m.request_mode = 'video'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.code IN ('seedance-2-5', 'seedance-2-0-fast', 'kling-v3-0-std', 'wan-2-6')
        AND m.is_enabled AND m.category = 'video' AND m.request_mode = 'video'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      ORDER BY array_position(ARRAY['seedance-2-5', 'seedance-2-0-fast', 'kling-v3-0-std', 'wan-2-6'], m.code)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.is_enabled AND m.category = 'video' AND m.request_mode = 'video'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      ORDER BY m.sort_order, m.id
      LIMIT 1)
  ) AS code
), enhance_model AS (
  SELECT COALESCE(
    (SELECT m.code
       FROM models m, workflow w
      WHERE m.code = NULLIF(w.runtime_config->>'prompt_enhance_model_code', '')
        AND m.is_enabled AND m.category = 'chat'
        AND m.request_mode IN ('chat_completions', 'responses')
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT code FROM copy_model)
  ) AS code
), picked AS (
  SELECT c.code AS copy_code, i.code AS image_code, v.code AS video_code, e.code AS enhance_code
  FROM copy_model c, image_model i, video_model v, enhance_model e
), rewritten AS (
  SELECT w.id,
    (
      SELECT jsonb_agg(
        CASE
          WHEN n.value->>'id' = 'copy' AND p.copy_code IS NOT NULL THEN jsonb_set(n.value, '{model_code}', to_jsonb(p.copy_code), true)
          WHEN n.value->>'id' = 'poster' AND p.image_code IS NOT NULL THEN jsonb_set(n.value, '{model_code}', to_jsonb(p.image_code), true)
          WHEN n.value->>'id' = 'video' AND p.video_code IS NOT NULL THEN jsonb_set(n.value, '{model_code}', to_jsonb(p.video_code), true)
          ELSE n.value
        END
        ORDER BY n.ordinality
      )
      FROM jsonb_array_elements(w.nodes) WITH ORDINALITY AS n(value, ordinality)
    ) AS nodes
  FROM workflow_definitions w
  JOIN picked p ON true
  WHERE w.code = 'ecommerce_video'
)
UPDATE workflow_definitions w
SET nodes = r.nodes,
    runtime_config = w.runtime_config || jsonb_strip_nulls(jsonb_build_object(
      'agent_mode', 'custom_nodes',
      'generation_type', 'video',
      'generation_model_code', p.video_code,
      'image_model_code', p.image_code,
      'prompt_enhance_model_code', p.enhance_code,
      'creative_scenes', jsonb_build_array('product_video'),
      'default_count', 1,
      'input_capabilities', jsonb_build_object(
        'allow_text_only', true,
        'support_reference_image', true,
        'support_multiple_references', true
      ),
      'flow_options', jsonb_build_object(
        'enable_step_confirm', true,
        'enable_autopilot', true,
        'allow_prompt_edit', true
      )
    )),
    price_rule = jsonb_build_object(
      'billing_type', 'model_actual',
      'unit_price', 0,
      'currency', 'CREDITS',
      'price_unit_normalized', true
    ),
    display_config = jsonb_set(
      jsonb_set(
        jsonb_set(
          COALESCE(w.display_config, '{}'::jsonb),
          '{steps}',
          '[
            {"icon":"🔍","title":"商品与卖点","subtitle":"读取商品描述和参考图，写出发布文案、海报画面和视频镜头","tags":["商品识别","卖点文案"]},
            {"icon":"✅","title":"方案确认","subtitle":"可改文案、海报和镜头；智能托管则直接继续","tags":["逐步确认","可编辑"]},
            {"icon":"🎬","title":"海报与成片","subtitle":"先生成商品海报，再以海报作为视频首帧","tags":["海报首帧","展示成片"]}
          ]'::jsonb,
          true
        ),
        '{hero_tags}',
        '["带货短视频","海报首帧","逐步确认"]'::jsonb,
        true
      ),
      '{feature_tags}',
      '["商品参考图","发布文案","海报首帧","展示短视频"]'::jsonb,
      true
    ),
    updated_at = now()
FROM rewritten r, picked p
WHERE w.id = r.id;

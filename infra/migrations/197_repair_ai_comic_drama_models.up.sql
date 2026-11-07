-- 修复 AI 漫剧 - S2.0 的模型槽位。
--
-- 032/098 建表时写入的是当时的演示模型（chat_demo_v1 / video_demo_v1 /
-- image_fast_v1）。165 停用了 image_fast_v1，演示对话/视频模型也已从模型库
-- 下线，于是前端一提交就报「所选图片模型不存在、类型不匹配或已停用：
-- image_fast_v1」；即使绕过校验，worker 也会因为 video_demo_v1 不存在而失败。
--
-- 修复策略：仍然可用的选择保持不变；失效槽位优先沿用通用创作智能体
-- (general_creative_agent) 的同槽位模型，再退到稳定的首选代号，最后选该
-- 分类下第一个启用模型（优先带启用线路）。nodes 与 system_configs 的默认值
-- 同步更新，避免后台/前台再次写回旧代号，并让重复执行保持幂等。

WITH target AS (
  SELECT id, runtime_config, nodes
  FROM workflow_definitions
  WHERE code = 'ai_comic_drama'
), general AS (
  SELECT runtime_config
  FROM workflow_definitions
  WHERE code = 'general_creative_agent'
), plan AS (
  SELECT COALESCE(
    (SELECT m.code
       FROM models m, target t
      WHERE m.code = NULLIF(t.runtime_config->>'analysis_model_code', '')
        AND m.code NOT IN ('chat_demo_v1', 'image_fast_v1', 'video_demo_v1')
        AND m.is_enabled AND m.category = 'chat'
        AND m.request_mode IN ('chat_completions', 'responses')
      LIMIT 1),
    (SELECT m.code
       FROM models m, general g
      WHERE m.code = NULLIF(g.runtime_config->>'analysis_model_code', '')
        AND m.code NOT IN ('chat_demo_v1', 'image_fast_v1', 'video_demo_v1')
        AND m.is_enabled AND m.category = 'chat'
        AND m.request_mode IN ('chat_completions', 'responses')
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.code = 'deepseek-v4-1-flash-1017'
        AND m.is_enabled AND m.category = 'chat'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.is_enabled AND m.category = 'chat'
        AND m.request_mode IN ('chat_completions', 'responses')
        AND m.code <> 'multi_collab_chat'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      ORDER BY m.sort_order, m.id
      LIMIT 1)
  ) AS code
), backup AS (
  SELECT COALESCE(
    (SELECT m.code
       FROM models m, plan p, general g
      WHERE m.code = NULLIF(g.runtime_config->>'fallback_model_code', '')
        AND m.code IS DISTINCT FROM p.code
        AND m.is_enabled AND m.category = 'chat'
        AND m.request_mode IN ('chat_completions', 'responses')
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m, plan p
      WHERE m.code = 'deepseek-v4-1-flash-1017'
        AND m.code IS DISTINCT FROM p.code
        AND m.is_enabled AND m.category = 'chat'
        AND m.request_mode IN ('chat_completions', 'responses')
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m, plan p
      WHERE m.code IS DISTINCT FROM p.code
        AND m.is_enabled AND m.category = 'chat'
        AND m.request_mode IN ('chat_completions', 'responses')
        AND m.code <> 'multi_collab_chat'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      ORDER BY m.sort_order, m.id
      LIMIT 1)
  ) AS code
), image AS (
  SELECT COALESCE(
    (SELECT m.code
       FROM models m, target t
      WHERE m.code = NULLIF(t.runtime_config->>'image_model_code', '')
        AND m.code NOT IN ('chat_demo_v1', 'image_fast_v1', 'video_demo_v1')
        AND m.is_enabled AND m.category = 'image' AND m.request_mode = 'images'
      LIMIT 1),
    (SELECT m.code
       FROM models m, general g
      WHERE m.code = NULLIF(g.runtime_config->>'image_model_code', '')
        AND m.code NOT IN ('chat_demo_v1', 'image_fast_v1', 'video_demo_v1')
        AND m.is_enabled AND m.category = 'image' AND m.request_mode = 'images'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.code IN ('seedream-5-0-pro', 'seedream-4-5', 'seedream-5-0-lite')
        AND m.is_enabled AND m.category = 'image' AND m.request_mode = 'images'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      ORDER BY array_position(ARRAY['seedream-5-0-pro', 'seedream-4-5', 'seedream-5-0-lite'], m.code)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.is_enabled AND m.category = 'image' AND m.request_mode = 'images'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      ORDER BY m.sort_order, m.id
      LIMIT 1)
  ) AS code
), video AS (
  SELECT COALESCE(
    (SELECT m.code
       FROM models m, target t
      WHERE m.code = NULLIF(t.runtime_config->>'video_model_code', '')
        AND m.code NOT IN ('chat_demo_v1', 'image_fast_v1', 'video_demo_v1')
        AND m.is_enabled AND m.category = 'video' AND m.request_mode = 'video'
      LIMIT 1),
    (SELECT m.code
       FROM models m, general g
      WHERE m.code = NULLIF(g.runtime_config->>'video_model_code', '')
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
), narration AS (
  SELECT COALESCE(
    (SELECT m.code
       FROM models m, target t
      WHERE m.code = NULLIF(t.runtime_config->>'narration_model_code', '')
        AND m.is_enabled AND m.category = 'audio' AND m.request_mode = 'audio'
      LIMIT 1),
    (SELECT m.code
       FROM models m, general g
      WHERE m.code = NULLIF(g.runtime_config->>'speech_model_code', '')
        AND m.is_enabled AND m.category = 'audio' AND m.request_mode = 'audio'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.code = 'audio_minimax_speech_28_hd'
        AND m.is_enabled AND m.category = 'audio' AND m.request_mode = 'audio'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.is_enabled AND m.category = 'audio' AND m.request_mode = 'audio'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      ORDER BY m.sort_order, m.id
      LIMIT 1)
  ) AS code
), final AS (
  SELECT
    plan.code AS plan_code,
    backup.code AS backup_code,
    image.code AS image_code,
    video.code AS video_code,
    narration.code AS narration_code,
    CASE
      WHEN narration.code IS NULL THEN 'video_native'
      WHEN COALESCE((SELECT runtime_config->>'audio_strategy' FROM target), '') IN ('video_native', 'tts_only', 'hybrid')
        THEN (SELECT runtime_config->>'audio_strategy' FROM target)
      ELSE 'hybrid'
    END AS audio_strategy
  FROM plan, backup, image, video, narration
)
UPDATE workflow_definitions w
SET runtime_config = w.runtime_config || jsonb_build_object(
      'analysis_model_code', COALESCE(f.plan_code, ''),
      'dialogue_model_codes', (
        SELECT COALESCE(jsonb_agg(v.code ORDER BY v.ord), '[]'::jsonb)
        FROM (VALUES (1, f.plan_code), (2, f.backup_code)) AS v(ord, code)
        WHERE v.code IS NOT NULL
      ),
      'image_model_code', COALESCE(f.image_code, ''),
      'video_model_code', COALESCE(f.video_code, ''),
      'generation_model_code', COALESCE(f.video_code, ''),
      'narration_model_code', COALESCE(f.narration_code, ''),
      'audio_strategy', f.audio_strategy
    ),
    nodes = (
      SELECT jsonb_agg(
        CASE n->>'id'
          WHEN 'comic_plan' THEN jsonb_set(n, '{model_code}', to_jsonb(COALESCE(f.plan_code, '')), true)
          WHEN 'keyframes' THEN jsonb_set(n, '{model_code}', to_jsonb(COALESCE(f.image_code, '')), true)
          WHEN 'video_segments' THEN jsonb_set(n, '{model_code}', to_jsonb(COALESCE(f.video_code, '')), true)
          WHEN 'narrations' THEN jsonb_set(n, '{model_code}', to_jsonb(COALESCE(f.narration_code, '')), true)
          WHEN 'compose' THEN jsonb_set(n, '{model_code}', '""'::jsonb, true)
          ELSE n
        END
        ORDER BY x.ord
      )
      FROM jsonb_array_elements(t.nodes) WITH ORDINALITY AS x(n, ord)
    ),
    updated_at = now()
FROM target t, final f
WHERE w.id = t.id;

-- 同步漫画项目默认值（033 写入的是演示模型；这些键只被本模块使用）。
WITH repaired AS (
  SELECT runtime_config
  FROM workflow_definitions
  WHERE code = 'ai_comic_drama'
)
UPDATE system_configs s
SET value = v.value, updated_at = now()
FROM (
  SELECT 'comic_drama_default_dialogue_model' AS key,
         to_jsonb(COALESCE(r.runtime_config->'dialogue_model_codes'->>0, r.runtime_config->>'analysis_model_code', '')) AS value
    FROM repaired r
  UNION ALL
  SELECT 'comic_drama_backup_dialogue_models', COALESCE(r.runtime_config->'dialogue_model_codes', '[]'::jsonb)
    FROM repaired r
  UNION ALL
  SELECT 'comic_drama_default_image_model', to_jsonb(COALESCE(r.runtime_config->>'image_model_code', ''))
    FROM repaired r
  UNION ALL
  SELECT 'comic_drama_default_video_model', to_jsonb(COALESCE(r.runtime_config->>'video_model_code', ''))
    FROM repaired r
) v
WHERE s.key = v.key AND s.value IS DISTINCT FROM v.value;

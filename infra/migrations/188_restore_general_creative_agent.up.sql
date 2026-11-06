-- Repair only general_creative_agent. Other workflow rows are not read or updated.
-- A previous admin save stored this row as an ecommerce image pipeline and
-- cleared the image, video, speech and music model slots.

WITH target AS (
  SELECT runtime_config
  FROM workflow_definitions
  WHERE code = 'general_creative_agent'
), analysis AS (
  SELECT COALESCE(
    (SELECT m.code
       FROM models m, target w
      WHERE m.code = NULLIF(w.runtime_config->>'analysis_model_code', '')
        AND m.is_enabled
        AND m.category = 'chat'
        AND m.request_mode IN ('chat_completions', 'responses')
        AND m.code <> 'multi_collab_chat'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      LIMIT 1),
    (SELECT m.code
       FROM models m
      WHERE m.is_enabled
        AND m.category = 'chat'
        AND m.request_mode IN ('chat_completions', 'responses')
        AND m.code <> 'multi_collab_chat'
        AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
      ORDER BY m.sort_order, m.id
      LIMIT 1)
  ) AS code
), picked AS (
  SELECT
    analysis.code AS analysis_code,
    COALESCE(
      (SELECT m.code
         FROM models m, target w, analysis a
        WHERE m.code = NULLIF(w.runtime_config->>'fallback_model_code', '')
          AND m.code IS DISTINCT FROM a.code
          AND m.is_enabled
          AND m.category = 'chat'
          AND m.request_mode IN ('chat_completions', 'responses')
          AND m.code <> 'multi_collab_chat'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        LIMIT 1),
      (SELECT m.code
         FROM models m, analysis a
        WHERE m.code = 'deepseek-v4-1-flash-1017'
          AND m.code IS DISTINCT FROM a.code
          AND m.is_enabled
          AND m.category = 'chat'
          AND m.request_mode IN ('chat_completions', 'responses')
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        LIMIT 1),
      (SELECT m.code
         FROM models m, analysis a
        WHERE m.code IS DISTINCT FROM a.code
          AND m.is_enabled
          AND m.category = 'chat'
          AND m.request_mode IN ('chat_completions', 'responses')
          AND m.code <> 'multi_collab_chat'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        ORDER BY m.sort_order, m.id
        LIMIT 1)
    ) AS fallback_code,
    COALESCE(
      (SELECT m.code
         FROM models m, target w
        WHERE m.code = NULLIF(w.runtime_config->>'image_model_code', '')
          AND m.is_enabled
          AND m.request_mode = 'images'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        LIMIT 1),
      (SELECT m.code
         FROM models m
        WHERE m.is_enabled
          AND m.request_mode = 'images'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        ORDER BY m.sort_order, m.id
        LIMIT 1)
    ) AS image_code,
    COALESCE(
      (SELECT m.code
         FROM models m, target w
        WHERE m.code = NULLIF(w.runtime_config->>'video_model_code', '')
          AND m.is_enabled
          AND m.request_mode = 'video'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        LIMIT 1),
      (SELECT m.code
         FROM models m
        WHERE m.is_enabled
          AND m.request_mode = 'video'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        ORDER BY m.sort_order, m.id
        LIMIT 1)
    ) AS video_code,
    COALESCE(
      (SELECT m.code
         FROM models m, target w
        WHERE m.code = NULLIF(w.runtime_config->>'speech_model_code', '')
          AND m.is_enabled
          AND m.request_mode = 'audio'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        LIMIT 1),
      (SELECT m.code
         FROM models m
        WHERE m.code = 'audio_minimax_speech_28_hd'
          AND m.is_enabled
          AND m.request_mode = 'audio'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        LIMIT 1),
      (SELECT m.code
         FROM models m
        WHERE m.is_enabled
          AND m.request_mode = 'audio'
          AND lower(m.code || ' ' || m.display_name) ~ '(speech|tts|语音|配音)'
          AND lower(m.code || ' ' || m.display_name) !~ '(music|suno|音乐|歌曲)'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        ORDER BY m.sort_order, m.id
        LIMIT 1)
    ) AS speech_code,
    COALESCE(
      (SELECT m.code
         FROM models m, target w
        WHERE m.code = NULLIF(w.runtime_config->>'music_model_code', '')
          AND m.is_enabled
          AND m.request_mode = 'audio'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        LIMIT 1),
      (SELECT m.code
         FROM models m
        WHERE m.code = 'audio_minimax_music_26'
          AND m.is_enabled
          AND m.request_mode = 'audio'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        LIMIT 1),
      (SELECT m.code
         FROM models m
        WHERE m.is_enabled
          AND m.request_mode = 'audio'
          AND lower(m.code || ' ' || m.display_name) ~ '(music|suno|音乐|歌曲)'
          AND EXISTS (SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.is_enabled)
        ORDER BY m.sort_order, m.id
        LIMIT 1)
    ) AS music_code
  FROM analysis
)
UPDATE workflow_definitions AS w
SET category = 'chat',
    description = CASE
      WHEN w.runtime_config->>'agent_mode' IS DISTINCT FROM 'creative_chat'
        THEN '通过聊天理解创作意图，自动生成图片、视频、语音或音乐任务。'
      ELSE w.description
    END,
    price_rule = jsonb_strip_nulls(COALESCE(w.price_rule, '{}'::jsonb) || jsonb_build_object(
      'billing_type', 'model_actual',
      'unit_price', 0
    )),
    display_config = CASE
      WHEN w.runtime_config->>'agent_mode' IS DISTINCT FROM 'creative_chat'
        THEN COALESCE(w.display_config, '{}'::jsonb) || '{"theme":"amber","hero_tags":["聊天创作","图片生成","视频生成","音频生成"],"help":"描述目标并添加可选参考素材，Agent 通用智能体会理解意图、选择后台默认模型并连续完成图片、视频、语音或音乐创作。"}'::jsonb
      ELSE w.display_config
    END,
    runtime_config = jsonb_strip_nulls(
      (COALESCE(w.runtime_config, '{}'::jsonb) - 'creative_scenes' - 'require_image')
      || jsonb_build_object(
        'agent_mode', 'creative_chat',
        'generation_type', 'mixed',
        'preset_code', 'general_creative_agent',
        'analysis_model_code', picked.analysis_code,
        'fallback_model_code', picked.fallback_code,
        'image_model_code', picked.image_code,
        'video_model_code', picked.video_code,
        'speech_model_code', picked.speech_code,
        'music_model_code', picked.music_code,
        'generation_model_code', picked.image_code,
        'default_count', 1,
        'candidate_count', 1,
        'input_capabilities', CASE
          WHEN w.runtime_config #>> '{input_capabilities,support_reference_video}' = 'true'
           AND w.runtime_config #>> '{input_capabilities,support_reference_audio}' = 'true'
            THEN w.runtime_config->'input_capabilities'
          ELSE '{"allow_text_only":true,"support_reference_image":true,"support_reference_video":true,"support_reference_audio":true,"support_multiple_references":true}'::jsonb
        END
      )
    ),
    updated_at = now()
FROM picked
WHERE w.code = 'general_creative_agent';

-- 回滚 197：恢复修复前的演示模型引用。
--
-- 注意：这些演示模型（chat_demo_v1 / video_demo_v1）已从模型库下线，
-- image_fast_v1 也已在 165 停用；回滚后 AI 漫剧 - S2.0 会重新变得不可用。
-- 保留此脚本只为满足迁移成对回滚的要求。

UPDATE workflow_definitions w
SET runtime_config = w.runtime_config || jsonb_build_object(
      'analysis_model_code', 'chat_demo_v1',
      'dialogue_model_codes', jsonb_build_array('chat_demo_v1'),
      'image_model_code', 'image_fast_v1',
      'video_model_code', 'video_demo_v1',
      'generation_model_code', 'video_demo_v1',
      'narration_model_code', 'audio_minimax_speech_28_hd',
      'audio_strategy', 'hybrid'
    ),
    nodes = (
      SELECT jsonb_agg(
        CASE n->>'id'
          WHEN 'comic_plan' THEN jsonb_set(n, '{model_code}', '"chat_demo_v1"'::jsonb, true)
          WHEN 'keyframes' THEN jsonb_set(n, '{model_code}', '"image_fast_v1"'::jsonb, true)
          WHEN 'video_segments' THEN jsonb_set(n, '{model_code}', '"video_demo_v1"'::jsonb, true)
          WHEN 'narrations' THEN jsonb_set(n, '{model_code}', '"audio_minimax_speech_28_hd"'::jsonb, true)
          WHEN 'compose' THEN jsonb_set(n, '{model_code}', '""'::jsonb, true)
          ELSE n
        END
        ORDER BY x.ord
      )
      FROM jsonb_array_elements(w.nodes) WITH ORDINALITY AS x(n, ord)
    ),
    updated_at = now()
WHERE w.code = 'ai_comic_drama';

UPDATE system_configs SET value = '"chat_demo_v1"'::jsonb, updated_at = now()
WHERE key = 'comic_drama_default_dialogue_model';

UPDATE system_configs SET value = '["chat_demo_v1"]'::jsonb, updated_at = now()
WHERE key = 'comic_drama_backup_dialogue_models';

UPDATE system_configs SET value = '"image_fast_v1"'::jsonb, updated_at = now()
WHERE key = 'comic_drama_default_image_model';

UPDATE system_configs SET value = '"video_demo_v1"'::jsonb, updated_at = now()
WHERE key = 'comic_drama_default_video_model';

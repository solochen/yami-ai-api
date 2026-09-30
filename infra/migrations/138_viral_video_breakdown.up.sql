INSERT INTO workflow_definitions (
  code,
  name,
  description,
  icon,
  category,
  nodes,
  input_schema,
  price_rule,
  display_config,
  runtime_config,
  is_enabled,
  sort_order,
  created_at,
  updated_at
) VALUES (
  'viral_video_breakdown',
  '爆款视频拆解',
  '粘贴抖音分享文本、视频链接，或上传视频。系统下载源视频后拆解镜头、字幕和关键帧，并整理成可编辑的剧本。',
  '🎬',
  'tool',
  '[]'::jsonb,
  jsonb_build_object(
    'type', 'object',
    'properties', jsonb_build_object(
      'share_text', jsonb_build_object('type', 'string', 'title', '抖音分享文本或链接'),
      'video_url', jsonb_build_object('type', 'string', 'title', '本地视频')
    )
  ),
  jsonb_build_object('billing_type', 'duration_second', 'unit_price', 0),
  jsonb_build_object(
    'theme', 'emerald',
    'hero_tags', jsonb_build_array('抖音链接', '镜头拆解', '剧本', '关键帧'),
    'feature_tags', jsonb_build_array('理解', '字幕', '拆帧', '印证', '剧本'),
    'steps', jsonb_build_array(
      jsonb_build_object('icon', '🔗', 'title', '解析来源', 'subtitle', '从分享文本或上传文件取得源视频', 'tags', jsonb_build_array('抖音', '本地上传')),
      jsonb_build_object('icon', '🧠', 'title', '理解视频', 'subtitle', '整理角色、场景、道具和节奏', 'tags', jsonb_build_array('整段理解')),
      jsonb_build_object('icon', '🎞️', 'title', '字幕与拆帧', 'subtitle', '提取字幕、切镜并抽出参考关键帧', 'tags', jsonb_build_array('字幕', '关键帧')),
      jsonb_build_object('icon', '✅', 'title', '印证成稿', 'subtitle', '对照画面和字幕写出剧本', 'tags', jsonb_build_array('剧本', '冲突保留'))
    ),
    'input', jsonb_build_object('placeholder', '粘贴抖音分享文本或视频链接', 'modes', jsonb_build_array('智能托管')),
    'help', '粘贴整段抖音分享文案、视频链接，或上传本地视频。分析完成后给出剧本和原片关键帧。',
    'toolbox', jsonb_build_object(
      'enabled', true,
      'category', 'video',
      'subtitle', '从抖音链接或本地视频拆出剧本和关键帧',
      'tags', jsonb_build_array('抖音', '剧本', '关键帧'),
      'badges', jsonb_build_array('available'),
      'billing', '按视频秒数计费，单价在后台配置'
    )
  ),
  jsonb_build_object(
    'agent_mode', 'viral_video_breakdown',
    'preset_code', 'viral_video_breakdown',
    'generation_type', 'tool',
    'max_duration_sec', 180,
    'analysis_model_code', '',
    'input_capabilities', jsonb_build_object(
      'allow_text_only', true,
      'support_reference_image', false,
      'support_reference_video', true,
      'support_multiple_references', false,
      'support_first_last_frame', false
    ),
    'flow_options', jsonb_build_object(
      'enable_step_confirm', false,
      'enable_autopilot', true,
      'allow_prompt_edit', false
    )
  ),
  true,
  41,
  now(),
  now()
) ON CONFLICT (code) DO NOTHING;

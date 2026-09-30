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
  'extract_product_images',
  '提取商品图片',
  '粘贴京东商品链接，提取主图、销售规格、参数和详情图，并保存到资产库。',
  '🛍️',
  'tool',
  '[]'::jsonb,
  jsonb_build_object(
    'type', 'object',
    'required', jsonb_build_array('product_url'),
    'properties', jsonb_build_object(
      'product_url', jsonb_build_object('type', 'string', 'title', '京东商品链接')
    )
  ),
  jsonb_build_object('billing_type', 'per_request', 'unit_price', 0),
  jsonb_build_object(
    'theme', 'amber',
    'hero_tags', jsonb_build_array('京东商品', '主图', '详情图', '规格参数'),
    'feature_tags', jsonb_build_array('主图', '规格', '详情图', '资产库'),
    'steps', jsonb_build_array(
      jsonb_build_object('icon', '🔗', 'title', '粘贴商品链接', 'subtitle', '支持京东电脑端和手机端商品页', 'tags', jsonb_build_array('单品链接')),
      jsonb_build_object('icon', '🖼️', 'title', '提取图片和规格', 'subtitle', '读取主图、颜色规格、参数表和详情图', 'tags', jsonb_build_array('主图', '详情图')),
      jsonb_build_object('icon', '📥', 'title', '保存结果', 'subtitle', '预览、打包下载，并写入资产库', 'tags', jsonb_build_array('资产库', '打包'))
    ),
    'input', jsonb_build_object('placeholder', 'https://item.jd.com/10041037225218.html', 'modes', jsonb_build_array('智能托管')),
    'help', '粘贴一件京东商品的链接。系统会提取这件商品的主图、销售规格、参数和详情图。',
    'toolbox', jsonb_build_object(
      'enabled', true,
      'category', 'image',
      'subtitle', '从京东商品链接提取主图、规格和详情图',
      'tags', jsonb_build_array('京东', '主图', '详情图'),
      'badges', jsonb_build_array('available'),
      'billing', '按次，当前免费'
    )
  ),
  jsonb_build_object(
    'agent_mode', 'product_image_extract',
    'preset_code', 'product_image_extract',
    'generation_type', 'image',
    'input_capabilities', jsonb_build_object(
      'allow_text_only', true,
      'support_reference_image', false,
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
  42,
  now(),
  now()
) ON CONFLICT (code) DO NOTHING;

UPDATE workflow_definitions
SET
  description = '粘贴京东商品链接，提取主图、销售规格、参数和详情图。提取成功后按次扣除算力，失败不扣费。',
  input_schema = jsonb_set(input_schema, '{properties,product_url,title}', '"京东商品链接"'),
  display_config = COALESCE(display_config, '{}'::jsonb) || jsonb_build_object(
    'hero_tags', jsonb_build_array('京东商品', '主图', '详情图', '规格参数'),
    'help', '粘贴一件京东商品的链接。提取成功后扣除算力，失败不扣费。',
    'steps', jsonb_build_array(
      jsonb_build_object('icon', '🔗', 'title', '粘贴商品链接', 'subtitle', '支持京东电脑端和手机端商品页', 'tags', jsonb_build_array('单品链接')),
      jsonb_build_object('icon', '🖼️', 'title', '提取图片和规格', 'subtitle', '读取主图、颜色规格、参数表和详情图', 'tags', jsonb_build_array('主图', '详情图')),
      jsonb_build_object('icon', '📥', 'title', '保存结果', 'subtitle', '预览、打包下载，并写入资产库', 'tags', jsonb_build_array('资产库', '打包'))
    ),
    'toolbox', COALESCE(display_config->'toolbox', '{}'::jsonb) || jsonb_build_object(
      'subtitle', '从京东商品链接提取主图、规格和详情图',
      'tags', jsonb_build_array('京东', '主图', '详情图'),
      'billing', '提取成功后扣费，失败不扣费'
    )
  ),
  updated_at = now()
WHERE code = 'extract_product_images';

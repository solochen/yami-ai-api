-- 电商图片工厂（复刻 qqwapi.cn「电商图片工厂」）：模块目录 + 套装 + 批量 + 反推 + 精修。
-- 规格依据 docs/ecom-factory-spec.md；提示词模板与目录同源于
-- services/worker/cmd/worker/ecom_factory_catalog.json（worker embed，API catalog 下发同一份）。
INSERT INTO workflow_definitions(code,name,description,icon,category,nodes,input_schema,price_rule,display_config,runtime_config,is_enabled,sort_order,created_at,updated_at)
SELECT 'ecom_factory','电商图片工厂','上传商品图和参考图，一次生成主图、白底图、详情页、种草封面、直播贴片与风格复刻图，并输出可编辑反推提示词。','🛍️','workflow','[]'::jsonb,
'{"type":"object","additionalProperties":true,"properties":{"mode":{"type":"string","enum":["suite","batch","quick"]},"productName":{"type":"string"},"sellingPoints":{"type":"string"},"audience":{"type":"string"},"scenario":{"type":"string"},"platform":{"type":"string"},"language":{"type":"string"},"visualStyle":{"type":"string"},"customStyle":{"type":"string"},"selectedSuite":{"type":"string"},"selectedModuleIds":{"type":"array","items":{"type":"string"}},"ratio":{"type":"string"},"resolution":{"type":"string"},"countPerModule":{"type":"integer","minimum":1,"maximum":3},"copyEnabled":{"type":"boolean"},"extraPrompt":{"type":"string"},"assets":{"type":"array","items":{"type":"object"}},"batchPrompt":{"type":"string"},"batchCount":{"type":"integer","minimum":1,"maximum":200},"batchStrategy":{"type":"string"},"batchSimilarity":{"type":"string"},"batchVariation":{"type":"string"},"batchKeepKeys":{"type":"array","items":{"type":"string"}},"batchChangeKeys":{"type":"array","items":{"type":"string"}},"quickPrompt":{"type":"string"},"itemScope":{"type":"object"}}}'::jsonb,
'{"billing_type":"model_actual","unit_price":0}'::jsonb,
jsonb_build_object(
  'theme','orange',
  'hero_tags', '["套装一键出图","14 类电商图","参考图批量","提示词反推"]'::jsonb,
  'toolbox', jsonb_build_object('enabled',true,'category','image','subtitle','商品图、详情页、种草图一次配齐','tags','["主图","详情页","种草","批量"]'::jsonb,'badges','["recommended","available"]'::jsonb,'billing','按模型实际消耗')
),
jsonb_build_object(
  'agent_mode','ecom_factory',
  'preset_code','ecom_factory',
  'generation_type','image',
  'generation_model_code',COALESCE(
    (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.request_mode='images' AND m.code='gpt-image-2' LIMIT 1),
    (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.request_mode='images'
      AND COALESCE(m.runtime_rule#>>'{upstream,adapter}','')='openai_images'
      ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
    'gpt-image-2'),
  'analysis_model_code',COALESCE(
    (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.code='glm-5-3-flash' AND m.category='chat' LIMIT 1),
    (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.category='chat' AND (m.new_api_model ILIKE '%glm%' OR m.code ILIKE '%glm%') ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
    'glm-5-3-flash'),
  'default_count',1,
  'max_plan_items',24,
  'max_concurrency',8,
  'supported_resolutions','["1K","2K"]'::jsonb
),
true,78,now(),now()
ON CONFLICT(code) DO NOTHING;

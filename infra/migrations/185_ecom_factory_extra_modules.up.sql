-- 电商图片工厂：新增模块级资料模块（产品尺寸图/接口定义图/规格参数表）。
-- 模块与套装目录在代码内（worker ecom_factory_catalog.json + 前端 ecomFactory.ts 同源，双端 golden 对拍），
-- 本迁移仅同步 workflow_definitions 的展示文案（14 类→17 类）与输入 schema 描述（moduleExtras）。
UPDATE workflow_definitions
SET display_config = jsonb_set(display_config, '{hero_tags}', '["套装一键出图","17 类电商图","参考图批量","提示词反推"]'::jsonb),
    input_schema = input_schema || '{"properties":{"moduleExtras":{"type":"object","description":"模块级必填资料，键为模块 extraKey：sizeData 尺寸数据 / interfaceData 接口清单 / specData 参数内容","additionalProperties":{"type":"string"}}}}'::jsonb,
    updated_at = now()
WHERE code = 'ecom_factory';

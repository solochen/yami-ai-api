-- 回滚：hero_tags 恢复 14 类电商图，并移除 input_schema 中的 moduleExtras 字段。
UPDATE workflow_definitions
SET display_config = jsonb_set(display_config, '{hero_tags}', '["套装一键出图","14 类电商图","参考图批量","提示词反推"]'::jsonb),
    input_schema = input_schema #- '{properties,moduleExtras}',
    updated_at = now()
WHERE code = 'ecom_factory';

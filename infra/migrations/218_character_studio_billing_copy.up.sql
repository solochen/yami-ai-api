-- 角色替换卡片的计费文案不写死价格，避免与后台售价配置漂移。
UPDATE workflow_definitions
SET display_config = jsonb_set(
      display_config,
      '{toolbox,billing}',
      '"按开播时长计费，开播前显示实时价格"'::jsonb
    ),
    updated_at = now()
WHERE code = 'character_studio';

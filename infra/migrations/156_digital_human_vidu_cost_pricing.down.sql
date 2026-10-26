-- 回滚 156：恢复 154 之后的配置值与旧展示文案。
UPDATE system_configs
SET value = '3.5'::jsonb, updated_at = now()
WHERE key = 'vidu_video_price_per_minute';

UPDATE system_configs
SET value = '0.1'::jsonb, updated_at = now()
WHERE key = 'vidu_voice_price_per_minute';

UPDATE system_configs
SET value = '1'::jsonb, updated_at = now()
WHERE key = 'vidu_text_credit_rate';

UPDATE workflow_definitions
SET display_config = jsonb_set(
      display_config,
      '{toolbox,billing}',
      '"文字按会话计费，语音 100 算力/分钟"'::jsonb
    ),
    updated_at = now()
WHERE code = 'realtime_avatar';

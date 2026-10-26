-- 数字人（Vidu）计价修正。
--
-- 官方口径：Vidu 实时数字人 1.5 积分/秒，1 积分 = 0.03125 元，折合 2.8125 元/分钟
-- （纯语音会话在 Vidu 侧同为 audio 会话，按同一费率计费）。
--
-- 这里把 vidu_* 配置改成「上游成本价」，用户售价由代码按
--   售价 = 上游成本 ÷ (1 − 平台销售佣金率)
-- 推导，佣金率取 system_configs.platform_gross_margin_percent（后台可视化配置）。

UPDATE system_configs
SET value = '2.8125'::jsonb, updated_at = now()
WHERE key = 'vidu_video_price_per_minute';

UPDATE system_configs
SET value = '2.8125'::jsonb, updated_at = now()
WHERE key = 'vidu_voice_price_per_minute';

-- 文字会话按 Vidu 返回的积分折算：1 积分 = 0.03125 元。
UPDATE system_configs
SET value = '0.03125'::jsonb, updated_at = now()
WHERE key = 'vidu_text_credit_rate';

-- 展示文案不再写死价格数字，避免与配置漂移（此前遗留「语音 100 算力/分钟」）。
UPDATE workflow_definitions
SET display_config = jsonb_set(
      display_config,
      '{toolbox,billing}',
      '"语音、视频按通话时长计费，文字按实际用量计费"'::jsonb
    ),
    updated_at = now()
WHERE code = 'realtime_avatar'
  AND display_config->'toolbox'->>'billing' IS NOT NULL;

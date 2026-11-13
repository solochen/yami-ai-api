-- 216_official_price_currency：把从 legacy price_rule 回填的"国外模型官方价"从人民币换算回美元。
-- 旧 price_rule 是「美元官方价 × 7.2」写成人民币，官方价字段应保存厂商原币种。
UPDATE models
SET official_price_rule = jsonb_build_object(
      'currency', 'USD',
      'billing_type', 'per_token',
      'input_price_per_m',  ROUND((official_price_rule->>'input_price_per_m')::numeric / 7.2, 4),
      'output_price_per_m', ROUND((official_price_rule->>'output_price_per_m')::numeric / 7.2, 4),
      'cache_read_price_per_m', CASE
        WHEN (official_price_rule->>'cache_read_price_per_m') IS NULL THEN NULL
        ELSE ROUND((official_price_rule->>'cache_read_price_per_m')::numeric / 7.2, 4) END,
      'source', 'legacy_converted',
      'verified_at', to_char(now(), 'YYYY-MM-DD')
    ) - 'cache_read_price_per_m'
    || CASE WHEN (official_price_rule->>'cache_read_price_per_m') IS NULL THEN '{}'::jsonb
            ELSE jsonb_build_object('cache_read_price_per_m',
                 ROUND((official_price_rule->>'cache_read_price_per_m')::numeric / 7.2, 4)) END
WHERE official_price_rule->>'source' = 'legacy_price_rule'
  AND (official_price_rule->>'currency' = 'CNY' OR official_price_rule->>'currency' IS NULL)
  AND lower(code) ~ '(gpt|claude|gemini|grok|veo|sora|dall|o1|o3|o4|whisper|imagen|banana)';

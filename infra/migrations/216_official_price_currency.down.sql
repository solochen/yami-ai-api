-- 216_official_price_currency (down)：把 legacy_converted 的美元官方价换回人民币。
-- 仅用于回滚；只处理 216 打过标的 source='legacy_converted' 行。
UPDATE models
SET official_price_rule = jsonb_build_object(
      'currency', 'CNY',
      'billing_type', 'per_token',
      'input_price_per_m',  ROUND((official_price_rule->>'input_price_per_m')::numeric * 7.2, 4),
      'output_price_per_m', ROUND((official_price_rule->>'output_price_per_m')::numeric * 7.2, 4),
      'source', 'legacy_price_rule',
      'verified_at', to_char(now(), 'YYYY-MM-DD')
    )
    || CASE WHEN (official_price_rule->>'cache_read_price_per_m') IS NULL THEN '{}'::jsonb
            ELSE jsonb_build_object('cache_read_price_per_m',
                 ROUND((official_price_rule->>'cache_read_price_per_m')::numeric * 7.2, 4)) END
WHERE official_price_rule->>'source' = 'legacy_converted'
  AND official_price_rule->>'currency' = 'USD'
  AND lower(code) ~ '(gpt|claude|gemini|grok|veo|sora|dall|o1|o3|o4|whisper|imagen|banana)';

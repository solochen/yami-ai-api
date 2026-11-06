-- AI 写真馆与 OpenAI 图片模型统一使用 OpenAI 官方公开价格口径。
-- 金额单位是项目内部算力（人民币元等值），汇率沿用 system_configs 的 6.75。
--
-- OpenAI 官方 API Token 价因模型不同：GPT Image 2 为图片输入 $8/M、输出 $30/M，
-- GPT Image 1.5 / ChatGPT Image latest 为图片输入 $8/M、图片输出 $32/M、文本输出 $10/M，
-- GPT Image 1 mini 为图片输入 $2.50/M、图片输出 $8/M。文本、缓存价格也按各模型官方表设置。
-- GPT Image 2/2.5 的预估使用官方 Image Generation guide 的 Token 公式；1.5/latest/1 mini
-- 使用官方每图示例换算成估算 Token。可取到真实 usage 时始终按真实 Token 结算。

CREATE TABLE IF NOT EXISTS openai_image_official_pricing_backup_195 (
  model_id BIGINT PRIMARY KEY,
  model_code TEXT NOT NULL,
  old_price_rule JSONB NOT NULL,
  old_route_rules JSONB NOT NULL,
  backed_up_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS openai_image_official_workflow_backup_195 (
  workflow_code TEXT PRIMARY KEY,
  old_price_rule JSONB NOT NULL,
  backed_up_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Final official prices may be below a particular upstream route's cost.
-- Keep the user's listed charge exact and record the difference as negative margin.
ALTER TABLE platform_billing_accounts
  DROP CONSTRAINT IF EXISTS platform_billing_accounts_commission_compute_check,
  DROP CONSTRAINT IF EXISTS platform_billing_accounts_balanced;
ALTER TABLE billing_settlements
  DROP CONSTRAINT IF EXISTS billing_settlements_commission_compute_check,
  DROP CONSTRAINT IF EXISTS billing_settlements_charge_covers_cost;
ALTER TABLE billing_settlement_items
  DROP CONSTRAINT IF EXISTS billing_settlement_items_commission_compute_check;

INSERT INTO openai_image_official_workflow_backup_195 (workflow_code, old_price_rule)
SELECT code, COALESCE(price_rule, '{}'::jsonb)
FROM workflow_definitions
WHERE code = 'ai_photo_studio'
ON CONFLICT (workflow_code) DO NOTHING;

INSERT INTO openai_image_official_pricing_backup_195 (model_id, model_code, old_price_rule, old_route_rules)
SELECT m.id,
       m.code,
       COALESCE(m.price_rule, '{}'::jsonb),
       COALESCE((
         SELECT jsonb_agg(jsonb_build_object('route_id', r.id, 'user_price_rule', COALESCE(r.user_price_rule, '{}'::jsonb)) ORDER BY r.id)
         FROM model_routes r
         WHERE r.model_id = m.id
       ), '[]'::jsonb)
FROM models m
WHERE m.code IN (
  'gpt-image-2', 'gpt-image-2-5-flare', 'gpt-image-2-5-sunburst',
  'gpt-image-1.5', 'gpt-image-1-mini', 'chatgpt-image-latest'
)
ON CONFLICT (model_id) DO NOTHING;

CREATE TEMP TABLE openai_image_official_rule_195 ON COMMIT DROP AS
SELECT model.code,
jsonb_strip_nulls(jsonb_build_object(
  'currency', 'CREDITS',
  'billing_type', 'per_token',
  'price_unit_normalized', true,
  'source', 'openai_official',
  'source_currency', 'USD',
  'source_fx_rate', 6.75,
  'input_cost_per_m', CASE WHEN model.code = 'gpt-image-1-mini' THEN 16.875 ELSE 54 END,
  'image_input_cost_per_m', CASE WHEN model.code = 'gpt-image-1-mini' THEN 16.875 ELSE 54 END,
  'text_input_cost_per_m', CASE WHEN model.code = 'gpt-image-1-mini' THEN 13.5 ELSE 33.75 END,
  'output_cost_per_m', CASE
    WHEN model.code = 'gpt-image-1-mini' THEN 54
    WHEN model.code IN ('gpt-image-1.5','chatgpt-image-latest') THEN 216
    ELSE 202.5
  END,
  'image_output_cost_per_m', CASE
    WHEN model.code = 'gpt-image-1-mini' THEN 54
    WHEN model.code IN ('gpt-image-1.5','chatgpt-image-latest') THEN 216
    ELSE 202.5
  END,
  'text_output_cost_per_m', CASE
    WHEN model.code IN ('gpt-image-1.5','chatgpt-image-latest') THEN 67.5
    ELSE 0
  END,
  'cache_read_cost_per_m', CASE WHEN model.code = 'gpt-image-1-mini' THEN 1.6875 ELSE 13.5 END,
  'text_cache_read_cost_per_m', CASE WHEN model.code = 'gpt-image-1-mini' THEN 1.35 ELSE 8.4375 END,
  'cache_write_cost_per_m', CASE WHEN model.code = 'gpt-image-1-mini' THEN 16.875 ELSE 54 END,
  'user_price_is_final', true,
  'default_quality', 'medium',
  'image_output_token_estimator', CASE
    WHEN model.code = 'gpt-image-2' THEN 'gpt_image_2'
    WHEN model.code IN ('gpt-image-2-5-flare','gpt-image-2-5-sunburst') THEN 'gpt_image_2_5'
    ELSE NULL
  END,
  'estimated_output_tokens_by_variant', CASE
    WHEN model.code IN ('gpt-image-1.5','chatgpt-image-latest') THEN jsonb_build_object(
      'low_1k_square', 281, 'medium_1k_square', 1063, 'high_1k_square', 4156,
      'low_1k_portrait', 406, 'medium_1k_portrait', 1563, 'high_1k_portrait', 6250,
      'low_1k_landscape', 406, 'medium_1k_landscape', 1563, 'high_1k_landscape', 6250
    )
    WHEN model.code = 'gpt-image-1-mini' THEN jsonb_build_object(
      'low_1k_square', 625, 'medium_1k_square', 1375, 'high_1k_square', 4500,
      'low_1k_portrait', 750, 'medium_1k_portrait', 1875, 'high_1k_portrait', 6500,
      'low_1k_landscape', 750, 'medium_1k_landscape', 1875, 'high_1k_landscape', 6500
    )
    ELSE jsonb_build_object(
      'low_1k_square', 200, 'medium_1k_square', 1767, 'high_1k_square', 7034,
      'low_1k_portrait', 167, 'medium_1k_portrait', 1367, 'high_1k_portrait', 5500,
      'low_1k_landscape', 167, 'medium_1k_landscape', 1367, 'high_1k_landscape', 5500,
      'low_2k_square', 800, 'medium_2k_square', 7067, 'high_2k_square', 28133,
      'low_2k_portrait', 667, 'medium_2k_portrait', 5467, 'high_2k_portrait', 22000,
      'low_2k_landscape', 667, 'medium_2k_landscape', 5467, 'high_2k_landscape', 22000
    )
  END
)) AS rule
FROM models model
WHERE model.code IN (
  'gpt-image-2', 'gpt-image-2-5-flare', 'gpt-image-2-5-sunburst',
  'gpt-image-1.5', 'gpt-image-1-mini', 'chatgpt-image-latest'
);

UPDATE models m
SET price_rule = rule.rule,
    updated_at = now()
FROM openai_image_official_rule_195 rule
WHERE m.code IN (
  'gpt-image-2', 'gpt-image-2-5-flare', 'gpt-image-2-5-sunburst',
  'gpt-image-1.5', 'gpt-image-1-mini', 'chatgpt-image-latest'
)
  AND rule.code = m.code;

UPDATE model_routes r
SET user_price_rule = rule.rule,
    updated_at = now()
FROM models m
JOIN openai_image_official_rule_195 rule ON rule.code = m.code
WHERE r.model_id = m.id
;

-- 写真馆的模型实际用量已经包含造型分析与每张图片生成费用，避免再收固定工作流费。
UPDATE workflow_definitions
SET price_rule = jsonb_set(COALESCE(price_rule, '{}'::jsonb), '{unit_price}', '0'::jsonb, true),
    updated_at = now()
WHERE code = 'ai_photo_studio'
  AND COALESCE(price_rule->>'billing_type', '') = 'model_actual';

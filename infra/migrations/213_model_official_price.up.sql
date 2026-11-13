-- 213_model_official_price：模型级「官方价 + 售价倍率」。
-- 售价 = 官方价（国内 CNY 直存；国外 USD 按汇率折算）× price_multiplier。
ALTER TABLE models ADD COLUMN IF NOT EXISTS official_price_rule jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE models ADD COLUMN IF NOT EXISTS price_multiplier numeric(12,4) NOT NULL DEFAULT 0;

-- 初始官方价：沿用 price_rule（多数已是官方价换算成人民币），标记来源便于人工校正。
UPDATE models
SET official_price_rule = price_rule || jsonb_build_object('currency', 'CNY', 'source', 'legacy_price_rule')
WHERE official_price_rule = '{}'::jsonb AND price_rule <> '{}'::jsonb;

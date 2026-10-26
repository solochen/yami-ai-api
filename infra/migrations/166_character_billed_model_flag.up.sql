-- 把「按字符计费」标记同时写到模型价格规则上。
--
-- 预估走 estimatedTokenCounts(model.PriceRule, ...)，它只读模型价格规则；
-- 165 把 input_unit 只写在线路成本规则上，导致前台预估仍按 token 启发式估算
-- （中文 0.75 token/字符 + 8），与实际扣费（精确字符数）不一致。
UPDATE models
SET price_rule = price_rule || '{"input_unit":"character"}'::jsonb,
    updated_at = now()
WHERE code = 'audio_minimax_speech_28_hd'
  AND NOT (price_rule ? 'input_unit');

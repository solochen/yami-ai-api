-- 215_settlement_matrix_fx：上游价格矩阵在入库时使用的汇率，用于反推厂商官方美元价。
ALTER TABLE upstream_settlements ADD COLUMN IF NOT EXISTS matrix_fx numeric(12,6) NOT NULL DEFAULT 1;
UPDATE upstream_settlements SET matrix_fx = 6.75 WHERE provider IN ('codexauv','openrouter','aiboli','new_api');
UPDATE upstream_settlements SET matrix_fx = 1    WHERE provider IN ('packycode','qiniu','tokenhub','bailian');

-- 215_settlement_matrix_fx (down)：回滚汇率列。
ALTER TABLE upstream_settlements DROP COLUMN IF EXISTS matrix_fx;

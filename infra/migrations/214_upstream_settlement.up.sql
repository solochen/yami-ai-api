-- 214_upstream_settlement：上游结算汇率（人民币 / 上游标价单位）。
-- CodexAuv、PackyCode 充值 1:1，结算汇率 = 1.0；OpenRouter 按真实美元结算 = 7.2。
CREATE TABLE IF NOT EXISTS upstream_settlements (
  provider     varchar(64) PRIMARY KEY,
  cny_per_unit numeric(12,6) NOT NULL DEFAULT 1,
  matrix_note  text NOT NULL DEFAULT '',
  updated_at   timestamptz NOT NULL DEFAULT now()
);

INSERT INTO upstream_settlements (provider, cny_per_unit, matrix_note) VALUES
  ('codexauv',   1.0, '充值 1:1，标价即人民币等值'),
  ('packycode',  1.0, '充值 1:1，标价即人民币等值'),
  ('openrouter', 7.2, '按真实美元结算'),
  ('qiniu',      1.0, '人民币计价'),
  ('aiboli',     7.2, '按美元结算'),
  ('new_api',    7.2, '中转站按美元结算'),
  ('tokenhub',   1.0, '人民币计价'),
  ('bailian',    1.0, '人民币计价')
ON CONFLICT (provider) DO NOTHING;

-- 210_upstream_account_rate：账号记录所属分组的倍率（来自价格同步），便于按价格识别账号。
ALTER TABLE upstream_accounts ADD COLUMN IF NOT EXISTS rate_multiplier numeric(12,6) NOT NULL DEFAULT 0;

UPDATE upstream_accounts a
SET rate_multiplier = COALESCE((
  SELECT max(c.billing_multiplier) FROM upstream_price_current c
  WHERE c.provider = a.provider AND c.group_name = a.group_name AND COALESCE(c.group_name,'') <> ''
), 0);

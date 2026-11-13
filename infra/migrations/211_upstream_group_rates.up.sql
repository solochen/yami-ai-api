-- 211_upstream_group_rates：记录各上游分组的倍率（即使该分组没有可计价模型，如纯生图分组）。
CREATE TABLE IF NOT EXISTS upstream_group_rates (
  provider   varchar(64)  NOT NULL,
  group_name varchar(128) NOT NULL,
  rate       numeric(12,6) NOT NULL DEFAULT 0,
  updated_at timestamptz  NOT NULL DEFAULT now(),
  PRIMARY KEY (provider, group_name)
);

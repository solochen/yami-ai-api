-- 204_upstream_accounts：把「API Key + 所属分组」抽象为上游账号，线路可绑定账号。
--
-- 背景：CodexAuv / PackyCode 这类上游，一个 API Key 只属于一个分组，分组下才有模型。
-- 之前 model_routes 的 api_key 与 price_group 是两个独立字段，无法表达这个约束，
-- 换分组不换 Key 会调用失败，因此新增账号实体，并反向从现有线路生成账号。
--
-- 兼容策略：
--   1) 线路保留 api_key / price_group 字段（运行时代码不变），绑定账号时写入快照；
--   2) 已有线路按 (provider, api_key) 去重生成账号；分组取该 Key 下出现最多的非空分组；
--   3) 仅当线路 price_group 为空时才用账号分组回填，显式绑定过的分组不覆盖。

CREATE TABLE IF NOT EXISTS upstream_accounts (
  id               bigserial PRIMARY KEY,
  provider         varchar(64)  NOT NULL,
  name             varchar(128) NOT NULL,
  api_key          text         NOT NULL,
  group_name       varchar(128) NOT NULL DEFAULT '',
  is_enabled       boolean      NOT NULL DEFAULT true,
  notes            text,
  last_test_at     timestamptz,
  last_test_ok     boolean,
  last_test_error  text,
  created_at       timestamptz  NOT NULL DEFAULT now(),
  updated_at       timestamptz  NOT NULL DEFAULT now(),
  UNIQUE (provider, api_key)
);

ALTER TABLE model_routes ADD COLUMN IF NOT EXISTS account_id bigint REFERENCES upstream_accounts(id);

-- 反向生成账号：每个 (provider, api_key) 一个账号，分组取出现次数最多的非空分组
INSERT INTO upstream_accounts (provider, name, api_key, group_name)
SELECT g.provider,
       g.provider || ' · ' || COALESCE(NULLIF(g.group_name, ''), '未绑定分组') || ' · ' || left(md5(g.api_key), 4),
       g.api_key,
       COALESCE(g.group_name, '')
FROM (
  SELECT r.provider, r.api_key,
         (SELECT r2.price_group FROM model_routes r2
           WHERE r2.provider = r.provider AND r2.api_key = r.api_key AND COALESCE(r2.price_group,'') <> ''
           GROUP BY r2.price_group ORDER BY count(*) DESC, r2.price_group ASC LIMIT 1) AS group_name
  FROM model_routes r
  WHERE COALESCE(r.api_key, '') <> ''
  GROUP BY r.provider, r.api_key
) g
ON CONFLICT (provider, api_key) DO NOTHING;

-- 回填 account_id；仅在分组为空时用账号分组补齐
UPDATE model_routes r
SET account_id = a.id,
    price_group = CASE WHEN COALESCE(r.price_group, '') = '' THEN a.group_name ELSE r.price_group END,
    updated_at = now()
FROM upstream_accounts a
WHERE a.provider = r.provider AND a.api_key = r.api_key AND COALESCE(r.api_key,'') <> '';

-- 充值店铺地址修正。
--
-- 150/152 的种子把 recharge_shop_url 写成旧店铺 https://catfk.com/shop/yardai，
-- 品牌迁移后实际在用的是 https://catfk.com/shop/orbit-e。这里只修正这个残留的
-- 种子值，管理员自定义过的地址不会被覆盖。

UPDATE system_configs
SET value = '"https://catfk.com/shop/orbit-e"'::jsonb,
    updated_at = now()
WHERE key = 'recharge_shop_url'
  AND value #>> '{}' = 'https://catfk.com/shop/yardai';

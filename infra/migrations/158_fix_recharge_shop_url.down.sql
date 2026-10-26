-- 回滚 158：恢复 152 的种子值。
UPDATE system_configs
SET value = '"https://catfk.com/shop/yardai"'::jsonb,
    updated_at = now()
WHERE key = 'recharge_shop_url'
  AND value #>> '{}' = 'https://catfk.com/shop/orbit-e';

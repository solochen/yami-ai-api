-- Keep the original seeded package labels consistent with the CNY recharge
-- currency introduced by migration 150. Custom operator labels are untouched.
UPDATE payment_packages
SET name = CASE public_id
  WHEN 'pay_10' THEN '10 元套餐'
  WHEN 'pay_30' THEN '30 元套餐'
  WHEN 'pay_50' THEN '50 元套餐'
  WHEN 'pay_100' THEN '100 元套餐'
  WHEN 'pay_200' THEN '200 元套餐'
END,
updated_at = now()
WHERE (public_id, name) IN (
  ('pay_10', '10 USD'),
  ('pay_30', '30 USD'),
  ('pay_50', '50 USD'),
  ('pay_100', '100 USD'),
  ('pay_200', '200 USD')
);

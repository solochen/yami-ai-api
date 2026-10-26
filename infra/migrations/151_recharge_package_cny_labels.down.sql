UPDATE payment_packages
SET name = CASE public_id
  WHEN 'pay_10' THEN '10 USD'
  WHEN 'pay_30' THEN '30 USD'
  WHEN 'pay_50' THEN '50 USD'
  WHEN 'pay_100' THEN '100 USD'
  WHEN 'pay_200' THEN '200 USD'
END,
updated_at = now()
WHERE (public_id, name) IN (
  ('pay_10', '10 元套餐'),
  ('pay_30', '30 元套餐'),
  ('pay_50', '50 元套餐'),
  ('pay_100', '100 元套餐'),
  ('pay_200', '200 元套餐')
);

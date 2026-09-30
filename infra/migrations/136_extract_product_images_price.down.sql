UPDATE workflow_definitions
SET
  price_rule = jsonb_build_object('billing_type', 'per_request', 'unit_price', 0),
  updated_at = now()
WHERE code = 'extract_product_images';

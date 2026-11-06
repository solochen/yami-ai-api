-- Restore the pre-191 seed state: the three presets pointed at the demo model
-- in the legacy bare-array shape.
UPDATE model_channel_presets
SET model_codes = '["chat_demo_v1"]'::jsonb, updated_at = now()
WHERE key IN ('success_first', 'speed_first', 'price_first');

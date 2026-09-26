ALTER TABLE conversations
  ADD COLUMN IF NOT EXISTS compression_hash TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS compression_summary TEXT NOT NULL DEFAULT '';

INSERT INTO system_configs (key, value) VALUES
  ('chat_compression_enabled', 'true'::jsonb),
  ('chat_compression_model_code', '""'::jsonb),
  ('chat_compression_min_chars', '6000'::jsonb),
  ('chat_compression_min_messages', '8'::jsonb),
  ('chat_compression_keep_messages', '6'::jsonb)
ON CONFLICT (key) DO NOTHING;

UPDATE system_configs
SET value = '"gpt-6-luna"'::jsonb
WHERE key = 'chat_compression_model_code'
  AND EXISTS (SELECT 1 FROM models WHERE code = 'gpt-6-luna' AND is_enabled);

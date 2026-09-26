ALTER TABLE conversations
  DROP COLUMN IF EXISTS compression_hash,
  DROP COLUMN IF EXISTS compression_summary;

DELETE FROM system_configs WHERE key IN (
  'chat_compression_enabled',
  'chat_compression_model_code',
  'chat_compression_min_chars',
  'chat_compression_min_messages',
  'chat_compression_keep_messages'
);

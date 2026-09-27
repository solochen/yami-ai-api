DROP TABLE IF EXISTS digital_human_sessions;
DROP TABLE IF EXISTS digital_human_knowledge;
DROP TABLE IF EXISTS digital_human_messages;
DROP TABLE IF EXISTS digital_human_roles;
DELETE FROM workflow_definitions WHERE code = 'realtime_avatar';
DELETE FROM system_configs WHERE key IN (
  'vidu_api_key', 'vidu_api_host', 'vidu_public_base_url', 'vidu_voice_price_per_minute', 'vidu_text_credit_rate'
);

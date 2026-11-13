DELETE FROM system_configs WHERE key LIKE 'character_studio_%';
DELETE FROM workflow_definitions WHERE code = 'character_studio';
DROP TABLE IF EXISTS character_studio_takes;
DROP TABLE IF EXISTS character_studio_sessions;
DROP TABLE IF EXISTS character_studio_characters;

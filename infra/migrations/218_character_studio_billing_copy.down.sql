UPDATE workflow_definitions
SET display_config = jsonb_set(
      display_config,
      '{toolbox,billing}',
      '"按开播时长计费，2.49 算力/分钟"'::jsonb
    ),
    updated_at = now()
WHERE code = 'character_studio';

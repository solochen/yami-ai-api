UPDATE workflow_definitions
SET display_config = display_config - 'toolbox'
WHERE display_config ? 'toolbox';

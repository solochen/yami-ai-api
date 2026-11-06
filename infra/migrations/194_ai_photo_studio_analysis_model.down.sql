UPDATE workflow_definitions workflow
SET runtime_config = CASE
      WHEN backup.runtime_config ? 'analysis_model_code'
        THEN jsonb_set(COALESCE(workflow.runtime_config, '{}'::jsonb), '{analysis_model_code}', backup.runtime_config->'analysis_model_code', true)
      ELSE COALESCE(workflow.runtime_config, '{}'::jsonb) - 'analysis_model_code'
    END,
    nodes = backup.nodes,
    updated_at = now()
FROM ai_photo_studio_analysis_model_194_backup backup
WHERE workflow.id = backup.workflow_id
  AND workflow.runtime_config->>'analysis_model_code' = backup.applied_model_code;

DROP TABLE ai_photo_studio_analysis_model_194_backup;

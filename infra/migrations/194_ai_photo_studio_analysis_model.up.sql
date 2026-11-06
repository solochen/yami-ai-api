CREATE TABLE ai_photo_studio_analysis_model_194_backup (
  workflow_id BIGINT PRIMARY KEY REFERENCES workflow_definitions(id) ON DELETE CASCADE,
  runtime_config JSONB NOT NULL,
  nodes JSONB NOT NULL,
  applied_model_code TEXT NOT NULL
);

CREATE TEMP TABLE ai_photo_studio_analysis_model_194_selection ON COMMIT DROP AS
SELECT workflow.id,
       workflow.runtime_config,
       workflow.nodes,
       COALESCE(
         (
           SELECT model.code
           FROM models model
           WHERE model.code = NULLIF(workflow.runtime_config->>'analysis_model_code', '')
             AND model.is_enabled
             AND model.category = 'chat'
             AND model.request_mode IN ('chat_completions', 'responses')
             AND model.runtime_rule #>> '{capabilities,vision}' = 'true'
             AND model.runtime_rule #>> '{capabilities,image_input}' = 'true'
             AND EXISTS (
               SELECT 1
               FROM model_routes route
               WHERE route.model_id = model.id
                 AND route.is_enabled
                 AND route.health_status = 'healthy'
                 AND (route.cooldown_until IS NULL OR route.cooldown_until <= now())
             )
           LIMIT 1
         ),
         (
           SELECT model.code
           FROM models model
           WHERE model.code = 'qwen3-8-flash'
             AND model.is_enabled
             AND model.category = 'chat'
             AND model.request_mode IN ('chat_completions', 'responses')
             AND model.runtime_rule #>> '{capabilities,vision}' = 'true'
             AND model.runtime_rule #>> '{capabilities,image_input}' = 'true'
             AND EXISTS (
               SELECT 1
               FROM model_routes route
               WHERE route.model_id = model.id
                 AND route.is_enabled
                 AND route.health_status = 'healthy'
                 AND (route.cooldown_until IS NULL OR route.cooldown_until <= now())
             )
           LIMIT 1
         ),
         (
           SELECT model.code
           FROM models model
           WHERE model.is_enabled
             AND model.category = 'chat'
             AND model.request_mode IN ('chat_completions', 'responses')
             AND model.runtime_rule #>> '{capabilities,vision}' = 'true'
             AND model.runtime_rule #>> '{capabilities,image_input}' = 'true'
             AND EXISTS (
               SELECT 1
               FROM model_routes route
               WHERE route.model_id = model.id
                 AND route.is_enabled
                 AND route.health_status = 'healthy'
                 AND (route.cooldown_until IS NULL OR route.cooldown_until <= now())
             )
           ORDER BY model.sort_order, model.id
           LIMIT 1
         )
       ) AS analysis_model_code
FROM workflow_definitions workflow
WHERE workflow.code = 'ai_photo_studio';

INSERT INTO ai_photo_studio_analysis_model_194_backup (workflow_id, runtime_config, nodes, applied_model_code)
SELECT id, runtime_config, nodes, analysis_model_code
FROM ai_photo_studio_analysis_model_194_selection
WHERE analysis_model_code IS NOT NULL
  AND analysis_model_code IS DISTINCT FROM runtime_config->>'analysis_model_code';

UPDATE workflow_definitions workflow
SET runtime_config = jsonb_set(
      COALESCE(workflow.runtime_config, '{}'::jsonb),
      '{analysis_model_code}',
      to_jsonb(selection.analysis_model_code),
      true
    ),
    nodes = CASE
      WHEN jsonb_typeof(workflow.nodes) = 'array' THEN COALESCE((
        SELECT jsonb_agg(
          CASE WHEN node.value->>'id' = 'styling'
            THEN jsonb_set(node.value, '{model_code}', to_jsonb(selection.analysis_model_code), true)
            ELSE node.value
          END ORDER BY node.ordinality
        )
        FROM jsonb_array_elements(workflow.nodes) WITH ORDINALITY AS node(value, ordinality)
      ), workflow.nodes)
      ELSE workflow.nodes
    END,
    updated_at = now()
FROM ai_photo_studio_analysis_model_194_selection selection
WHERE workflow.id = selection.id
  AND selection.analysis_model_code IS NOT NULL
  AND selection.analysis_model_code IS DISTINCT FROM workflow.runtime_config->>'analysis_model_code';

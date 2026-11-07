UPDATE workflow_definitions
SET runtime_config = jsonb_set(runtime_config, '{embedding_model_code}', '"embedding_demo_v1"'::jsonb, true),
    updated_at = now()
WHERE code = 'renovation_budget_agent'
  AND COALESCE(runtime_config->>'embedding_model_code', '') IN (
    '',
    'kinfra-text-embedding-0-6b',
    'kinfra-text-embedding-4b',
    'kinfra-vl-embedding-2b',
    'kinfra-vl-embedding-8b',
    'qwen3-7-text-embedding',
    'qwen3-7-text-embedding-flash',
    'text-embedding-v4',
    'text-embedding-v3',
    'text-embedding-v2',
    'qwen3-vl-embedding',
    'tongyi-embedding-vision-plus-2026-03-06',
    'tongyi-embedding-vision-flash-2026-03-06'
  );

DELETE FROM models
WHERE code IN (
  'kinfra-text-embedding-0-6b',
  'kinfra-text-embedding-4b',
  'kinfra-vl-embedding-2b',
  'kinfra-vl-embedding-8b',
  'qwen3-7-text-embedding',
  'qwen3-7-text-embedding-flash',
  'text-embedding-v4',
  'text-embedding-v3',
  'text-embedding-v2',
  'qwen3-vl-embedding',
  'tongyi-embedding-vision-plus-2026-03-06',
  'tongyi-embedding-vision-flash-2026-03-06'
);

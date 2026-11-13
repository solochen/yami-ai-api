DELETE FROM upstream_model_mappings
WHERE provider IN ('packycode','codexauv','openrouter')
  AND upstream_model IN ('gpt-6.1-sol','openai/gpt-6.1-sol');
DELETE FROM model_routes r USING models m WHERE r.model_id=m.id AND m.code='gpt-6-1-sol';
DELETE FROM models WHERE code='gpt-6-1-sol';

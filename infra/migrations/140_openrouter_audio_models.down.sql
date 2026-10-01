DELETE FROM model_routes
WHERE route_name = 'OpenRouter 官方'
  AND model_id IN (
    SELECT id FROM models WHERE code IN (
      'audio_openrouter_lyria_3_pro',
      'audio_openrouter_lyria_3_clip',
      'audio_openrouter_gpt_audio',
      'audio_openrouter_gpt_audio_mini'
    )
  );

DELETE FROM models
WHERE code IN (
  'audio_openrouter_lyria_3_pro',
  'audio_openrouter_lyria_3_clip',
  'audio_openrouter_gpt_audio',
  'audio_openrouter_gpt_audio_mini'
);

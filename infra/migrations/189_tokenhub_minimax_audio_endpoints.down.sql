UPDATE model_routes AS r
SET endpoint = '/v1/t2a_v2',
    updated_at = now()
FROM models AS m
WHERE m.id = r.model_id
  AND m.code = 'audio_minimax_speech_28_hd'
  AND r.provider = 'tokenhub'
  AND r.endpoint = '/v1/wand/minimax-tts/sync_tts';

UPDATE model_routes AS r
SET endpoint = '/v1/music_generation',
    updated_at = now()
FROM models AS m
WHERE m.id = r.model_id
  AND m.code = 'audio_minimax_music_26'
  AND r.provider = 'tokenhub'
  AND r.endpoint = '/v1/wand/minimax-music/generation';

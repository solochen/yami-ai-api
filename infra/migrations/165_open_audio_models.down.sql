-- 回滚 165：关闭音频模型线路、恢复演示模型与音乐 schema。
UPDATE model_routes r SET is_enabled = false, updated_at = now()
FROM models m WHERE m.id = r.model_id AND m.code = 'audio_openrouter_lyria_3_pro';

DELETE FROM model_routes r USING models m
WHERE m.id = r.model_id AND m.code IN ('audio_minimax_speech_28_hd','audio_minimax_music_26') AND r.provider = 'tokenhub';

UPDATE models SET is_enabled = true, updated_at = now() WHERE code = 'image_fast_v1';

UPDATE models SET input_schema = input_schema #- '{properties,lyrics}', updated_at = now()
WHERE code = 'audio_minimax_music_26';

UPDATE models SET price_rule = '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"input_price_per_m":2,"output_price_per_m":4}'::jsonb, updated_at = now()
WHERE code = 'audio_minimax_speech_28_hd';

UPDATE models SET price_rule = '{"currency":"CREDITS","billing_type":"per_request","price_unit_normalized":true,"unit_price":1}'::jsonb, updated_at = now()
WHERE code = 'audio_minimax_music_26';

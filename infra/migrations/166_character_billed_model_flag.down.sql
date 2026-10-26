UPDATE models
SET price_rule = price_rule - 'input_unit',
    updated_at = now()
WHERE code = 'audio_minimax_speech_28_hd';

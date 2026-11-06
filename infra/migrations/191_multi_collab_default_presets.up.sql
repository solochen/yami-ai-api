-- 010_multi_collab.up.sql pointed all three channel presets at the demo model
-- 'chat_demo_v1'. That model is seeded disabled, so every collaboration request
-- died in the billing estimate with a generic "model not found" 500 and the
-- feature was unusable out of the box.
--
-- This migration re-points only the presets that are still on the stale demo
-- value, so environments that already configured real members keep them. It
-- also writes the current {"answer": [...], "summary": [...]} shape required by
-- PresetService.UpsertChannelPreset.

DO $$
DECLARE
  preferred_answers text[] := ARRAY['deepseek-v4-1-flash-1017', 'qwen3-8-flash'];
  preferred_summary text := 'deepseek-v4-pro-0813';
  answer_codes text[];
  summary_code text;
  payload jsonb;
BEGIN
  -- Prefer the verified default pair, but only for models that really exist and
  -- are enabled chat models in this database.
  SELECT array_agg(candidate.code ORDER BY candidate.ord)
  INTO answer_codes
  FROM (
    SELECT t.code, t.ord
    FROM unnest(preferred_answers) WITH ORDINALITY AS t(code, ord)
    WHERE EXISTS (
      SELECT 1 FROM models m
      WHERE m.code = t.code AND m.is_enabled AND m.category = 'chat'
    )
  ) AS candidate;

  -- Fall back to whatever enabled chat models this environment has.
  IF answer_codes IS NULL OR array_length(answer_codes, 1) < 2 THEN
    SELECT array_agg(picked.code)
    INTO answer_codes
    FROM (
      SELECT code FROM models
      WHERE is_enabled AND category = 'chat' AND code <> 'multi_collab_chat'
      ORDER BY sort_order, id
      LIMIT 2
    ) AS picked;
  END IF;

  IF answer_codes IS NULL OR array_length(answer_codes, 1) < 2 THEN
    RAISE NOTICE 'multi_collab preset migration skipped: fewer than 2 enabled chat models';
    RETURN;
  END IF;

  SELECT code INTO summary_code
  FROM models
  WHERE code = preferred_summary AND is_enabled AND category = 'chat';

  IF summary_code IS NULL THEN
    SELECT code INTO summary_code
    FROM models
    WHERE is_enabled AND category = 'chat' AND code <> 'multi_collab_chat'
      AND NOT (code = ANY(answer_codes))
    ORDER BY sort_order, id
    LIMIT 1;
  END IF;

  IF summary_code IS NULL THEN
    summary_code := answer_codes[1];
  END IF;

  payload := jsonb_build_object(
    'answer', to_jsonb(answer_codes),
    'summary', jsonb_build_array(summary_code)
  );

  UPDATE model_channel_presets
  SET model_codes = payload, updated_at = now()
  WHERE key IN ('success_first', 'speed_first', 'price_first')
    AND (
      model_codes IS NULL
      OR model_codes = '[]'::jsonb
      -- Only the retired demo model configured: treat as unconfigured.
      OR NOT EXISTS (
        SELECT 1
        FROM jsonb_array_elements_text(
          CASE
            WHEN jsonb_typeof(model_codes) = 'array' THEN model_codes
            ELSE COALESCE(model_codes -> 'answer', '[]'::jsonb)
          END
        ) AS configured(code)
        WHERE configured.code <> 'chat_demo_v1'
      )
    );
END $$;

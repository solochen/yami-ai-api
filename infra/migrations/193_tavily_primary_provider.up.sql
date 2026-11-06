-- 193_tavily_primary_provider.up.sql
--
-- Adds the `tavily_primary` web-search provider: paid Tavily first, then the
-- free self-hosted SearXNG layer, then Exa as the second paid layer. The
-- existing `hybrid` value keeps its original meaning (SearXNG first, Tavily as
-- backup), so no existing configuration is rewritten here.
--
-- Two things are needed for the new value to be usable:
--   1) the provider whitelist enforced by ValidateWebSearchConfig (Go) and the
--      admin dropdown must accept it — both are code changes, not schema;
--   2) `web_search_enabled` must follow the same "selecting a provider means the
--      feature is meant to be live" convention as the other provider values.
--
-- `system_configs` only has a primary key on `key`, so no constraint needs to be
-- relaxed for the new value; this migration only fixes up the enabled flag.
--
-- The Exa fallback key reuses the existing `web_search_exa_api_key` config, and
-- the free middle layer reuses `web_search_base_url`, so no new keys are added.

DO $$
BEGIN
  -- 100_agent_media_contract only rewrote the flag for the providers known at
  -- the time. Re-run the same rule for the new value so an administrator who
  -- already selected it does not silently keep the feature disabled.
  UPDATE system_configs
  SET value = 'true'::jsonb, updated_at = now()
  WHERE key = 'web_search_enabled'
    AND trim(both '"' from value::text) <> 'true'
    AND EXISTS (
      SELECT 1 FROM system_configs provider
      WHERE provider.key = 'web_search_provider'
        AND trim(both '"' from provider.value::text) = 'tavily_primary'
    );
END $$;

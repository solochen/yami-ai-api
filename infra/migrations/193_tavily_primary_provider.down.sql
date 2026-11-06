-- 193_tavily_primary_provider.down.sql
--
-- The provider value itself lives in system_configs and is operator data, so the
-- rollback only undoes what this migration may have changed automatically: the
-- enabled flag it flipped for the new provider. An administrator who selected
-- `tavily_primary` keeps that selection; the application falls back to the
-- default provider handling for an unknown value.
UPDATE system_configs
SET value = 'false'::jsonb, updated_at = now()
WHERE key = 'web_search_enabled'
  AND trim(both '"' from value::text) = 'true'
  AND EXISTS (
    SELECT 1 FROM system_configs provider
    WHERE provider.key = 'web_search_provider'
      AND trim(both '"' from provider.value::text) = 'tavily_primary'
  );

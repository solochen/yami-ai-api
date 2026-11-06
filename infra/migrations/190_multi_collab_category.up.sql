-- The multi-model collaboration entry was seeded with category='chat'
-- (see 010_multi_collab.up.sql). Every server-side branch that recognises the
-- collaboration entry tests `category = 'multi_collab'`, so with 'chat' the
-- entry silently degraded into a single-model request against a model that has
-- no upstream of its own, and the admin model form never rendered the
-- "默认渠道预设" panel that writes default_params.channel_key.
--
-- Server-side detection now also accepts the pseudo-model code, so this
-- migration is a data correction rather than a hard requirement — but the
-- category is what the admin UI, the public model list and the pricing view
-- branch on, so it must be right.

UPDATE models
SET category = 'multi_collab', updated_at = now()
WHERE code = 'multi_collab_chat' AND category <> 'multi_collab';

UPDATE models
SET category = 'chat', updated_at = now()
WHERE code = 'multi_collab_chat' AND category = 'multi_collab';

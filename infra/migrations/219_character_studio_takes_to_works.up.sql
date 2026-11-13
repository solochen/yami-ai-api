-- 角色替换片段此前只写入 assets，未写入 works，导致「我的作品」看不到。
-- 为历史片段补建作品记录；新片段由 API 在登记 take 时同步创建。
INSERT INTO works (
    public_id,
    user_id,
    type,
    title,
    prompt,
    asset_id,
    thumbnail_url,
    metadata,
    expires_at
)
SELECT
    'work_' || substr(md5(t.public_id), 1, 27),
    t.user_id,
    'video',
    '角色替换片段 ' || to_char(t.created_at, 'MM-DD HH24:MI'),
    '角色替换片段 ' || to_char(t.created_at, 'MM-DD HH24:MI'),
    a.id,
    t.url,
    jsonb_build_object(
        'video_url', t.url,
        'source', 'character_studio_backfill',
        'asset_public_id', t.asset_public_id
    ),
    now() + interval '7 days'
FROM character_studio_takes t
JOIN assets a ON a.public_id = t.asset_public_id AND a.user_id = t.user_id
WHERE t.asset_public_id <> ''
  AND NOT EXISTS (
      SELECT 1 FROM works w WHERE w.asset_id = a.id AND w.user_id = t.user_id
  );

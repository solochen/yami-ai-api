-- 角色替换（实时换角）智能体。
--
-- 产品链路：浏览器摄像头 -> 实时媒体通道 -> 流式重绘 -> 合成流回传。
-- 数据只保存角色卡、会话元信息与片段索引，不保存原始视频流。
-- 上游服务地址、密钥与计费口径只留在服务端；前台展示层使用中性术语。

CREATE TABLE character_studio_characters (
  id BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(32) UNIQUE NOT NULL,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name VARCHAR(64) NOT NULL,
  image_url TEXT NOT NULL,
  rights_confirmed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_character_studio_characters_user ON character_studio_characters(user_id, updated_at DESC);

CREATE TABLE character_studio_sessions (
  id BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(32) UNIQUE NOT NULL,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  character_id BIGINT REFERENCES character_studio_characters(id) ON DELETE SET NULL,
  current_character_id BIGINT REFERENCES character_studio_characters(id) ON DELETE SET NULL,
  live_id TEXT NOT NULL DEFAULT '',
  conn_id TEXT NOT NULL DEFAULT '',
  render_uid TEXT NOT NULL DEFAULT '',
  rtc_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  resolution VARCHAR(16) NOT NULL DEFAULT '720p',
  status VARCHAR(16) NOT NULL DEFAULT 'starting',
  switch_count INTEGER NOT NULL DEFAULT 0,
  error_code VARCHAR(64) NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  billed_seconds INTEGER NOT NULL DEFAULT 0,
  credits NUMERIC(18,6) NOT NULL DEFAULT 0,
  provider_cost NUMERIC(18,6) NOT NULL DEFAULT 0,
  cost NUMERIC(18,6) NOT NULL DEFAULT 0,
  started_at TIMESTAMPTZ,
  ended_at TIMESTAMPTZ,
  last_heartbeat_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_character_studio_sessions_user ON character_studio_sessions(user_id, created_at DESC);
CREATE INDEX idx_character_studio_sessions_status ON character_studio_sessions(status, created_at DESC);

CREATE TABLE character_studio_takes (
  id BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(32) UNIQUE NOT NULL,
  session_id BIGINT NOT NULL REFERENCES character_studio_sessions(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  character_id BIGINT REFERENCES character_studio_characters(id) ON DELETE SET NULL,
  asset_public_id VARCHAR(64) NOT NULL DEFAULT '',
  url TEXT NOT NULL DEFAULT '',
  duration_ms INTEGER NOT NULL DEFAULT 0,
  size_bytes BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_character_studio_takes_session ON character_studio_takes(session_id, created_at);

INSERT INTO workflow_definitions (
  code, name, description, category, icon, nodes, input_schema, price_rule,
  display_config, runtime_config, is_enabled, sort_order, created_at, updated_at
) VALUES (
  'character_studio',
  '角色替换',
  '选定角色参考形象，打开摄像头即可实时替换画面主体；直播中可随时换角，并抓取片段导出。',
  'tool',
  '角',
  '[]'::jsonb,
  '{}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  jsonb_build_object(
    'theme', 'rose',
    'feature_tags', jsonb_build_array('实时', '角色替换', '片段导出'),
    'toolbox', jsonb_build_object(
      'enabled', true,
      'category', 'video',
      'subtitle', '实时换角 · 保留动作与镜头节奏',
      'tags', jsonb_build_array('角色替换', '实时视频'),
      'badges', jsonb_build_array('available'),
      'billing', '按开播时长计费，2.49 算力/分钟'
    )
  ),
  '{"agent_mode":"character_studio"}'::jsonb,
  true,
  31,
  now(),
  now()
) ON CONFLICT (code) DO NOTHING;

INSERT INTO system_configs (key, value) VALUES
  ('character_studio_sale_price_per_minute', '2.49'::jsonb),
  ('character_studio_credit_per_second', '1'::jsonb),
  ('character_studio_max_minutes', '10'::jsonb),
  ('character_studio_default_resolution', '"720p"'::jsonb),
  ('character_studio_engine_url', '"https://g.alicdn.com/apsara-media-box/imp-web-rtc/7.1.9/aliyun-rtc-sdk.js"'::jsonb),
  ('character_studio_source_url', '"https://github.com/starai-platform/starai"'::jsonb)
ON CONFLICT (key) DO NOTHING;

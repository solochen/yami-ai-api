CREATE TABLE digital_human_roles (
  id BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(32) UNIQUE NOT NULL,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name VARCHAR(64) NOT NULL,
  relation VARCHAR(32) NOT NULL DEFAULT '',
  user_title VARCHAR(64) NOT NULL DEFAULT '',
  memory TEXT NOT NULL DEFAULT '',
  style TEXT NOT NULL DEFAULT '',
  persona TEXT NOT NULL DEFAULT '',
  voice VARCHAR(128) NOT NULL DEFAULT 'Tina',
  avatar_url TEXT NOT NULL DEFAULT '',
  knowledge_title VARCHAR(128) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_digital_human_roles_user ON digital_human_roles(user_id, updated_at DESC);

CREATE TABLE digital_human_messages (
  id BIGSERIAL PRIMARY KEY,
  role_id BIGINT NOT NULL REFERENCES digital_human_roles(id) ON DELETE CASCADE,
  speaker VARCHAR(16) NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE digital_human_knowledge (
  id BIGSERIAL PRIMARY KEY,
  role_id BIGINT NOT NULL REFERENCES digital_human_roles(id) ON DELETE CASCADE,
  title VARCHAR(128) NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE digital_human_sessions (
  id BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(32) UNIQUE NOT NULL,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id BIGINT NOT NULL REFERENCES digital_human_roles(id) ON DELETE CASCADE,
  live_id TEXT NOT NULL DEFAULT '',
  call_mode VARCHAR(16) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'active',
  knowledge_token TEXT NOT NULL DEFAULT '',
  rtc_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at TIMESTAMPTZ,
  cost NUMERIC(18,6) NOT NULL DEFAULT 0
);

INSERT INTO workflow_definitions (
  code, name, description, category, icon, nodes, input_schema, price_rule,
  display_config, runtime_config, is_enabled, sort_order, created_at, updated_at
) VALUES (
  'realtime_avatar',
  '实时数字人对话',
  '选择角色，用文字或语音和数字人对话。超长资料可通过知识库在对话中引用。',
  'tool',
  '聊',
  '[]'::jsonb,
  '{}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  jsonb_build_object(
    'theme', 'cyan',
    'feature_tags', jsonb_build_array('文字', '语音', '知识库'),
    'toolbox', jsonb_build_object(
      'enabled', true,
      'category', 'assistant',
      'subtitle', '选择角色，用文字或语音和数字人实时对话',
      'tags', jsonb_build_array('数字人', '语音'),
      'badges', jsonb_build_array('available'),
      'billing', '文字按会话计费，语音 100 算力/分钟'
    )
  ),
  '{"agent_mode":"realtime_avatar"}'::jsonb,
  true,
  30,
  now(),
  now()
) ON CONFLICT (code) DO NOTHING;

INSERT INTO system_configs (key, value) VALUES
  ('vidu_api_key', '""'::jsonb),
  ('vidu_api_host', '"api.vidu.com"'::jsonb),
  ('vidu_public_base_url', '""'::jsonb),
  ('vidu_voice_price_per_minute', '100'::jsonb),
  ('vidu_text_credit_rate', '1'::jsonb)
ON CONFLICT (key) DO NOTHING;

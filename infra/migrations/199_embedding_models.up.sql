-- 接入腾讯云 TokenHub 与阿里云百炼的文本/多模态向量模型。
--
-- 当前唯一消费方是装修预算助理（renovation_budget_agent）的知识库检索。
-- 文本模型走 OpenAI 兼容 /embeddings；腾讯多模态走 /embeddings/multimodal；
-- 百炼多模态走 DashScope 原生 multimodal-embedding。调用参数写在 models.runtime_rule.embedding。
--
-- 密钥从同厂商已启用、且不是占位符的线路复制。复制到真实密钥时线路启用，
-- 并把仍指向 embedding_demo_v1 的知识库槽改到可用模型。没有真实密钥时线路停用，
-- 槽位清空，检索自动退回关键词，避免演示模型编码把整个助理拦住。
--
-- 价格为北京地域官方牌价，单位元/百万 tokens。平台佣金沿用系统设置，不在这里加价。
-- 回滚：见 199_embedding_models.down.sql

INSERT INTO models (
  code, display_name, new_api_model, new_api_endpoint, request_mode, category,
  description, tags, capabilities, input_schema, default_params,
  new_api_extra_params, price_rule, runtime_rule, retention_days, is_enabled, sort_order
)
SELECT
  v.code, v.display_name, v.upstream_model, v.endpoint, 'embeddings', 'embedding',
  v.description, v.tags::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
  jsonb_build_object('connection', jsonb_build_object(
    'provider', v.provider,
    'protocol', 'openai',
    'base_url', v.base_url,
    'api_key', '',
    'auth_type', 'bearer',
    'api_key_header', 'Authorization'
  )),
  v.price_rule::jsonb,
  v.runtime_rule::jsonb,
  7, true, v.sort_order
FROM (VALUES
  ('kinfra-text-embedding-0-6b', 'Kinfra Text Embedding 0.6B', 'kinfra-text-embedding-0.6b', '/embeddings',
   '腾讯云 TokenHub 轻量文本向量，固定 1024 维，32k 上下文。适合大规模召回。单条不超过 2000 字。',
   '["向量","腾讯云"]', 'tokenhub', 'https://tokenhub.tencentmaas.com/v1',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.5,"output_price_per_m":0,"source_input_price_per_m":0.5,"source_output_price_per_m":0}',
   '{"embedding":{"adapter":"openai","endpoint":"/embeddings","batch_size":32,"max_input_runes":2000}}',
   910),
  ('kinfra-text-embedding-4b', 'Kinfra Text Embedding 4B', 'kinfra-text-embedding-4b', '/embeddings',
   '腾讯云 TokenHub 高质量文本向量，固定 2560 维，32k 上下文。适合深层语义检索。单条不超过 2000 字。',
   '["向量","腾讯云"]', 'tokenhub', 'https://tokenhub.tencentmaas.com/v1',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.6,"output_price_per_m":0,"source_input_price_per_m":0.6,"source_output_price_per_m":0}',
   '{"embedding":{"adapter":"openai","endpoint":"/embeddings","batch_size":32,"max_input_runes":2000}}',
   911),
  ('kinfra-vl-embedding-2b', 'Kinfra VL Embedding 2B', 'kinfra-vl-embedding-2b', '/embeddings/multimodal',
   '腾讯云 TokenHub 轻量多模态向量，固定 2048 维，支持文本、图片和视频。知识库检索只发送文本。',
   '["向量","多模态","腾讯云"]', 'tokenhub', 'https://tokenhub.tencentmaas.com/v1',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.5,"text_input_price_per_m":0.5,"image_input_price_per_m":0.7,"output_price_per_m":0,"source_input_price_per_m":0.5,"source_text_input_price_per_m":0.5,"source_image_input_price_per_m":0.7,"source_output_price_per_m":0,"source_pricing_note":"视频输入 1.5 元/百万 tokens"}',
   '{"embedding":{"adapter":"tokenhub_multimodal","endpoint":"/embeddings/multimodal","batch_size":1,"max_input_runes":2000}}',
   912),
  ('kinfra-vl-embedding-8b', 'Kinfra VL Embedding 8B', 'kinfra-vl-embedding-8b', '/embeddings/multimodal',
   '腾讯云 TokenHub 高精度多模态向量，固定 4096 维，支持文本、图片和视频。知识库检索只发送文本。',
   '["向量","多模态","腾讯云"]', 'tokenhub', 'https://tokenhub.tencentmaas.com/v1',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.6,"text_input_price_per_m":0.6,"image_input_price_per_m":0.9,"output_price_per_m":0,"source_input_price_per_m":0.6,"source_text_input_price_per_m":0.6,"source_image_input_price_per_m":0.9,"source_output_price_per_m":0,"source_pricing_note":"视频输入 1.8 元/百万 tokens"}',
   '{"embedding":{"adapter":"tokenhub_multimodal","endpoint":"/embeddings/multimodal","batch_size":1,"max_input_runes":2000}}',
   913),
  ('qwen3-7-text-embedding', 'Qwen3.7 Text Embedding', 'qwen3.7-text-embedding', '/embeddings',
   '阿里云百炼文本向量。默认 1024 维，可在 256 到 2560 之间选择，128k 上下文，覆盖 201 种语言。装修知识库默认使用这款。',
   '["向量","阿里云","百炼"]', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.5,"output_price_per_m":0,"source_input_price_per_m":0.5,"source_output_price_per_m":0}',
   '{"embedding":{"adapter":"openai","endpoint":"/embeddings","batch_size":20,"dimensions":1024}}',
   920),
  ('qwen3-7-text-embedding-flash', 'Qwen3.7 Text Embedding Flash', 'qwen3.7-text-embedding-flash', '/embeddings',
   '阿里云百炼轻量文本向量。默认 1024 维，最高 1024 维，128k 上下文。成本和吞吐优先。',
   '["向量","阿里云","百炼"]', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.125,"output_price_per_m":0,"source_input_price_per_m":0.125,"source_output_price_per_m":0}',
   '{"embedding":{"adapter":"openai","endpoint":"/embeddings","batch_size":20,"dimensions":1024}}',
   921),
  ('text-embedding-v4', 'Text Embedding V4', 'text-embedding-v4', '/embeddings',
   '阿里云百炼 Qwen3-Embedding 文本向量。默认 1024 维，可选 64 到 2048 维，单行 8192 token。',
   '["向量","阿里云","百炼"]', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.5,"output_price_per_m":0,"source_input_price_per_m":0.5,"source_output_price_per_m":0}',
   '{"embedding":{"adapter":"openai","endpoint":"/embeddings","batch_size":10,"dimensions":1024}}',
   922),
  ('text-embedding-v3', 'Text Embedding V3', 'text-embedding-v3', '/embeddings',
   '阿里云百炼上一代文本向量。默认 1024 维，用于已有 v3 索引迁移。',
   '["向量","阿里云","百炼"]', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.5,"output_price_per_m":0,"source_input_price_per_m":0.5,"source_output_price_per_m":0}',
   '{"embedding":{"adapter":"openai","endpoint":"/embeddings","batch_size":10,"dimensions":1024}}',
   923),
  ('text-embedding-v2', 'Text Embedding V2', 'text-embedding-v2', '/embeddings',
   '阿里云百炼文本向量 v2。维度固定 1536，单行 2048 token。',
   '["向量","阿里云","百炼"]', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.7,"output_price_per_m":0,"source_input_price_per_m":0.7,"source_output_price_per_m":0}',
   '{"embedding":{"adapter":"openai","endpoint":"/embeddings","batch_size":25}}',
   924),
  ('qwen3-vl-embedding', 'Qwen3 VL Embedding', 'qwen3-vl-embedding', '/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding',
   '阿里云百炼多模态向量，支持文本、图片和视频。默认 2560 维。知识库检索按文本逐条编码。',
   '["向量","多模态","阿里云","百炼"]', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.7,"text_input_price_per_m":0.7,"image_input_price_per_m":1.8,"output_price_per_m":0,"source_input_price_per_m":0.7,"source_text_input_price_per_m":0.7,"source_image_input_price_per_m":1.8,"source_output_price_per_m":0,"source_pricing_note":"图片和视频输入 1.8 元/百万 tokens"}',
   '{"embedding":{"adapter":"dashscope_multimodal","endpoint":"/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding","batch_size":8}}',
   930),
  ('tongyi-embedding-vision-plus-2026-03-06', '通义视觉向量 Plus', 'tongyi-embedding-vision-plus-2026-03-06', '/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding',
   '阿里云百炼通义视觉向量 Plus（2026-03-06）。默认 1152 维，支持文搜图、图搜图。知识库检索按文本逐条编码。',
   '["向量","多模态","阿里云","百炼"]', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.5,"output_price_per_m":0,"source_input_price_per_m":0.5,"source_output_price_per_m":0}',
   '{"embedding":{"adapter":"dashscope_multimodal","endpoint":"/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding","batch_size":8}}',
   931),
  ('tongyi-embedding-vision-flash-2026-03-06', '通义视觉向量 Flash', 'tongyi-embedding-vision-flash-2026-03-06', '/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding',
   '阿里云百炼通义视觉向量 Flash（2026-03-06）。默认 768 维，成本优先的多模态向量。知识库检索按文本逐条编码。',
   '["向量","多模态","阿里云","百炼"]', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com',
   '{"currency":"CREDITS","billing_type":"per_token","price_unit_normalized":true,"source_currency":"CNY","input_price_per_m":0.15,"output_price_per_m":0,"source_input_price_per_m":0.15,"source_output_price_per_m":0}',
   '{"embedding":{"adapter":"dashscope_multimodal","endpoint":"/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding","batch_size":8}}',
   932)
) AS v(code, display_name, upstream_model, endpoint, description, tags, provider, base_url, price_rule, runtime_rule, sort_order)
ON CONFLICT (code) DO NOTHING;

INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT
  m.id,
  CASE v.provider WHEN 'tokenhub' THEN '腾讯云 TokenHub' ELSE '阿里云百炼 · 向量模型' END,
  v.provider, 'openai', m.new_api_model, m.new_api_endpoint, v.base_url,
  COALESCE(k.api_key, v.placeholder_key),
  'bearer', 'Authorization',
  '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
  v.cost_rule::jsonb,
  5, 100, 60, 1,
  k.api_key IS NOT NULL
FROM models m
JOIN (VALUES
  ('kinfra-text-embedding-0-6b', 'tokenhub', 'https://tokenhub.tencentmaas.com/v1', 'REPLACE_ME_TENCENT_TOKENHUB_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.5,"output_cost_per_m":0,"source_input_cost_per_m":0.5,"source_output_cost_per_m":0}'),
  ('kinfra-text-embedding-4b', 'tokenhub', 'https://tokenhub.tencentmaas.com/v1', 'REPLACE_ME_TENCENT_TOKENHUB_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.6,"output_cost_per_m":0,"source_input_cost_per_m":0.6,"source_output_cost_per_m":0}'),
  ('kinfra-vl-embedding-2b', 'tokenhub', 'https://tokenhub.tencentmaas.com/v1', 'REPLACE_ME_TENCENT_TOKENHUB_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.5,"text_input_cost_per_m":0.5,"image_input_cost_per_m":0.7,"output_cost_per_m":0,"source_input_cost_per_m":0.5,"source_output_cost_per_m":0,"source_pricing_note":"视频输入 1.5 元/百万 tokens"}'),
  ('kinfra-vl-embedding-8b', 'tokenhub', 'https://tokenhub.tencentmaas.com/v1', 'REPLACE_ME_TENCENT_TOKENHUB_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.6,"text_input_cost_per_m":0.6,"image_input_cost_per_m":0.9,"output_cost_per_m":0,"source_input_cost_per_m":0.6,"source_output_cost_per_m":0,"source_pricing_note":"视频输入 1.8 元/百万 tokens"}'),
  ('qwen3-7-text-embedding', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1', 'REPLACE_ME_BAILIAN_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.5,"output_cost_per_m":0,"source_input_cost_per_m":0.5,"source_output_cost_per_m":0}'),
  ('qwen3-7-text-embedding-flash', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1', 'REPLACE_ME_BAILIAN_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.125,"output_cost_per_m":0,"source_input_cost_per_m":0.125,"source_output_cost_per_m":0}'),
  ('text-embedding-v4', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1', 'REPLACE_ME_BAILIAN_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.5,"output_cost_per_m":0,"source_input_cost_per_m":0.5,"source_output_cost_per_m":0}'),
  ('text-embedding-v3', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1', 'REPLACE_ME_BAILIAN_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.5,"output_cost_per_m":0,"source_input_cost_per_m":0.5,"source_output_cost_per_m":0}'),
  ('text-embedding-v2', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1', 'REPLACE_ME_BAILIAN_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.7,"output_cost_per_m":0,"source_input_cost_per_m":0.7,"source_output_cost_per_m":0}'),
  ('qwen3-vl-embedding', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com', 'REPLACE_ME_BAILIAN_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.7,"text_input_cost_per_m":0.7,"image_input_cost_per_m":1.8,"output_cost_per_m":0,"source_input_cost_per_m":0.7,"source_output_cost_per_m":0,"source_pricing_note":"图片和视频输入 1.8 元/百万 tokens"}'),
  ('tongyi-embedding-vision-plus-2026-03-06', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com', 'REPLACE_ME_BAILIAN_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.5,"output_cost_per_m":0,"source_input_cost_per_m":0.5,"source_output_cost_per_m":0}'),
  ('tongyi-embedding-vision-flash-2026-03-06', 'bailian', 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com', 'REPLACE_ME_BAILIAN_APIKEY',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"CNY","input_cost_per_m":0.15,"output_cost_per_m":0,"source_input_cost_per_m":0.15,"source_output_cost_per_m":0}')
) AS v(code, provider, base_url, placeholder_key, cost_rule) ON v.code = m.code
LEFT JOIN (
  SELECT DISTINCT ON (provider) provider, api_key
  FROM model_routes
  WHERE provider IN ('tokenhub', 'bailian')
    AND is_enabled
    AND btrim(api_key) <> ''
    AND api_key NOT LIKE '%REPLACE_ME%'
    AND api_key NOT LIKE '%CHANGEME%'
    AND api_key NOT LIKE '%YOUR_API_KEY%'
  ORDER BY provider, updated_at DESC
) k ON k.provider = v.provider
WHERE NOT EXISTS (
  SELECT 1 FROM model_routes r WHERE r.model_id = m.id AND r.provider = v.provider
);

UPDATE workflow_definitions AS workflow
SET runtime_config = jsonb_set(workflow.runtime_config, '{embedding_model_code}', to_jsonb(pick.code), true),
    updated_at = now()
FROM (
  SELECT model.code
  FROM models model
  JOIN model_routes route ON route.model_id = model.id AND route.is_enabled
  WHERE model.code IN (
    'qwen3-7-text-embedding',
    'kinfra-text-embedding-4b',
    'qwen3-7-text-embedding-flash',
    'kinfra-text-embedding-0-6b'
  )
  ORDER BY CASE model.code
    WHEN 'qwen3-7-text-embedding' THEN 1
    WHEN 'kinfra-text-embedding-4b' THEN 2
    WHEN 'qwen3-7-text-embedding-flash' THEN 3
    ELSE 4
  END
  LIMIT 1
) AS pick
WHERE workflow.code = 'renovation_budget_agent'
  AND COALESCE(workflow.runtime_config->>'embedding_model_code', '') IN ('', 'embedding_demo_v1');

UPDATE workflow_definitions
SET runtime_config = jsonb_set(runtime_config, '{embedding_model_code}', '""'::jsonb, true),
    updated_at = now()
WHERE code = 'renovation_budget_agent'
  AND COALESCE(runtime_config->>'embedding_model_code', '') = 'embedding_demo_v1';

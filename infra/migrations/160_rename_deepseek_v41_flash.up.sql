-- 模型编码与上游模型对齐到 DeepSeek V4.1 Flash 1017。
--
-- 背景：该模型编码是 deepseek-v4-flash-0731（创建后 admin 不允许改编码，因为
-- 编码是模型的稳定标识，被 API 文档、会话、工作流运行时配置等引用），但实际
-- 上游早已升级到 DeepSeek V4.1 Flash 1017：
--   - 腾讯云 TokenHub：V4.1-Flash-1017 原厂直供的 model 参数为 deepseek/deepseek-flash
--   - OpenRouter：deepseek/deepseek-v4.1-flash-1017（$0.15/$0.60 每 M tokens）
-- 这里把编码、展示名、两条线路的上游模型以及所有引用点一次性对齐。

-- ========= 1. 编码重命名 =========
UPDATE models
SET code = 'deepseek-v4-1-flash-1017',
    display_name = 'DeepSeek-V4.1-Flash-1017',
    new_api_model = 'deepseek/deepseek-flash',
    updated_at = now()
WHERE code = 'deepseek-v4-flash-0731';

-- ========= 2. 引用点：聊天压缩模型配置 =========
UPDATE system_configs
SET value = '"deepseek-v4-1-flash-1017"'::jsonb, updated_at = now()
WHERE key = 'chat_compression_model_code'
  AND value #>> '{}' = 'deepseek-v4-flash-0731';

-- ========= 3. 引用点：装修预算助理的对话模型 =========
UPDATE workflow_definitions
SET runtime_config = jsonb_set(runtime_config, '{chat_model_code}', '"deepseek-v4-1-flash-1017"'::jsonb),
    updated_at = now()
WHERE runtime_config ->> 'chat_model_code' = 'deepseek-v4-flash-0731';

-- ========= 4. 腾讯云 TokenHub 线路：上游换成 V4.1-Flash-1017 原厂直供 =========
-- 成本价沿用峰时价 2/8（V4.1-Flash-1017 与 V4-Flash-0731 同价：闲 1/4、峰 2/8）。
UPDATE model_routes r
SET upstream_model = 'deepseek/deepseek-flash',
    updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND r.provider = 'tokenhub'
  AND r.upstream_model = 'deepseek/deepseek-v4-flash-0731';

-- ========= 5. OpenRouter 线路：上游换成 deepseek-v4.1-flash-1017 并按现价修正成本 =========
-- OR 现价 $0.15 / $0.60 / 缓存 $0.003（每 M tokens），按 1 USD = 7.2 元折算为
-- 1.08 / 4.32 / 0.0216，替换此前从模型价格回填的 2/8（那是过时的导入价）。
UPDATE model_routes r
SET upstream_model = 'deepseek/deepseek-v4.1-flash-1017',
    cost_rule = '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"price_unit_normalized":true,"source_currency":"USD","usd_to_compute":7.2,"input_cost_per_m":1.08,"output_cost_per_m":4.32,"cache_read_cost_per_m":0.0216,"source_input_cost_per_m":0.15,"source_output_cost_per_m":0.6,"source_cache_read_cost_per_m":0.003}'::jsonb,
    updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND r.provider = 'openrouter'
  AND r.upstream_model IN ('deepseek/deepseek-v4-flash-0731', 'deepseek/deepseek-v4.1-flash-1017');

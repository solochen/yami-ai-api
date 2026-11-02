-- 接入 PackyCode（packyapi.ai）为上游：按现有 model code 逐项匹配，能便宜 30%~65% 的设主线路，
-- 仅小幅便宜或只能做灾备的设备选线路。
--
-- 计价口径（PackyCode 公开 /api/pricing，充值倍率 1 元 = 1 美元额度）：
--   ¥/M = model_ratio × 2 × 分组倍率
-- 分组倍率按下列分组计价；**实际分组由 Key 所属分组决定**，若账号不在对应分组，账单会高于本表，
-- 需在后台把 cost_rule 改成实际单价：
--   bailian           0.5   qwen3.8-flash / qwen3.8-max-0902（阿里百炼官方渠道）
--   deepseek-sale     0.5   deepseek-v4-flash / deepseek-v4-pro
--   azure-officially  5     gpt-5.6-sol / gpt-6-astra / gpt-6-sol（Azure 官方，5 元/$）
--   gemini-officially 7     gemini-3.8-flash / gemini-3.7-flash（官方正价）
--   claude-officially 7     claude 系列（官方正价）
--   glm-sale          1     glm-5.3-flash
--   grok-officially   2.5   grok-imagine-image-2.0
--
-- 与现有线路对比（人民币/百万 token）：
--   qwen3.8-flash      现 0.8/2.7      → PackyCode 0.4/1.35   主线路
--   qwen3.8-max-0902   现 12/36        → PackyCode 6/18       主线路
--   deepseek flash     现 1/4          → PackyCode 0.5/2      主线路
--   deepseek v4-pro    现 4.5/13.5     → PackyCode 2.25/6.75  主线路
--   gpt-5.6-sol        现 28.8/144     → PackyCode 20/100     主线路
--   gpt-6-astra        现 72/360       → PackyCode 50/250     主线路
--   gpt-6-sol          现 14.4/72      → PackyCode 10/50      主线路
--   gemini-3.8-flash   现 5.94/29.7    → PackyCode 5.25/26.25 主线路
--   grok-image-2.0     现 0.288/张     → PackyCode 0.1/张     主线路
--   glm-5.3-flash      现 0.8/2.8      → PackyCode 0.8/2.8    备选（同价）
--   claude 系列        现 OpenRouter   → PackyCode 官方组约便宜 2.8%，作灾备
--
-- 注意：
--   1. PackyCode 官方组之外还有 cc / cc-sale / codex-sale 等订阅号池分组，价格更低但会降智/封号，
--      本迁移一律不使用，避免污染对外 SLA。
--   2. gpt-5.6-sol / gpt-6-sol 在 PackyCode 有 272k 上下文阶梯（超出后输入 ×2、输出 ×1.5），
--      当前 cost_rule 不建模长上下文阶梯，超长请求会低估成本。
--   3. api_key 为占位符且 is_enabled=false：需在后台填入 PackyCode Key，确认 Key 所属分组与
--      cost_rule 一致后再启用；启用后 priority=2 的主线路会自动接管。
-- 回滚：见 178_packycode_routes.down.sql

-- ========= 1. 主线路（priority=2）：比现有最优线路便宜 10% 以上 =========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai',
       v.upstream_model, '/v1/chat/completions', 'https://www.packyapi.ai',
       'REPLACE_ME_PACKYCODE_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, v.cost_rule::jsonb,
       2, 100, 120, 1, false
FROM models m
JOIN (VALUES
  ('qwen3-8-flash', 'qwen3.8-flash',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":0.4,"output_cost_per_m":1.35,"cache_read_cost_per_m":0.05,
     "price_unit_normalized":true,"source_input_cost_per_m":0.4,"source_output_cost_per_m":1.35,"source_cache_read_cost_per_m":0.05}'),
  ('qwen3-8-max-0902', 'qwen3.8-max-0902',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":6,"output_cost_per_m":18,"cache_read_cost_per_m":0.75,
     "price_unit_normalized":true,"source_input_cost_per_m":6,"source_output_cost_per_m":18,"source_cache_read_cost_per_m":0.75}'),
  ('deepseek-v4-1-flash-1017', 'deepseek-v4-flash',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":0.5,"output_cost_per_m":2,"cache_read_cost_per_m":0.01,
     "price_unit_normalized":true,"source_input_cost_per_m":0.5,"source_output_cost_per_m":2,"source_cache_read_cost_per_m":0.01}'),
  ('deepseek-v4-pro-0813', 'deepseek-v4-pro',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":2.25,"output_cost_per_m":6.75,"cache_read_cost_per_m":0.074925,
     "price_unit_normalized":true,"source_input_cost_per_m":2.25,"source_output_cost_per_m":6.75,"source_cache_read_cost_per_m":0.074925}'),
  ('gpt-5-6-sol', 'gpt-5.6-sol',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":20,"output_cost_per_m":100,"cache_read_cost_per_m":2,"cache_write_cost_per_m":25,
     "price_unit_normalized":true,"source_input_cost_per_m":20,"source_output_cost_per_m":100,
     "source_cache_read_cost_per_m":2,"source_cache_write_cost_per_m":25}'),
  ('gpt-6-astra', 'gpt-6-astra',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":50,"output_cost_per_m":250,"cache_read_cost_per_m":5,"cache_write_cost_per_m":62.5,
     "price_unit_normalized":true,"source_input_cost_per_m":50,"source_output_cost_per_m":250,
     "source_cache_read_cost_per_m":5,"source_cache_write_cost_per_m":62.5}'),
  ('gpt-6-sol', 'gpt-6-sol',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":10,"output_cost_per_m":50,"cache_read_cost_per_m":1,"cache_write_cost_per_m":12.5,
     "price_unit_normalized":true,"source_input_cost_per_m":10,"source_output_cost_per_m":50,
     "source_cache_read_cost_per_m":1,"source_cache_write_cost_per_m":12.5}'),
  ('gemini-3-8-flash', 'gemini-3.8-flash',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":5.25,"output_cost_per_m":26.25,"cache_read_cost_per_m":0.525,
     "price_unit_normalized":true,"source_input_cost_per_m":5.25,"source_output_cost_per_m":26.25,"source_cache_read_cost_per_m":0.525}')
) AS v(code, upstream_model, cost_rule) ON v.code = m.code;

-- ========= 2. 备选线路：GLM Flash 同价、Gemini 3.7 仅便宜 2.8% =========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, 'PackyCode 备选线路', 'packycode', 'openai',
       v.upstream_model, '/v1/chat/completions', 'https://www.packyapi.ai',
       'REPLACE_ME_PACKYCODE_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, v.cost_rule::jsonb,
       v.priority, 100, 120, 1, false
FROM models m
JOIN (VALUES
  ('glm-5-3-flash', 'glm-5.3-flash', 50,
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":0.8,"output_cost_per_m":2.8,"cache_read_cost_per_m":0.23,
     "price_unit_normalized":true,"source_input_cost_per_m":0.8,"source_output_cost_per_m":2.8,"source_cache_read_cost_per_m":0.23}'),
  ('gemini-3-7-flash', 'gemini-3.7-flash', 110,
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":5.25,"output_cost_per_m":26.25,"cache_read_cost_per_m":0.525,
     "price_unit_normalized":true,"source_input_cost_per_m":5.25,"source_output_cost_per_m":26.25,"source_cache_read_cost_per_m":0.525}')
) AS v(code, upstream_model, priority, cost_rule) ON v.code = m.code;

-- ========= 3. Claude 系列：官方正价约便宜 2.8%，只做 OpenRouter 之后的灾备 =========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, 'PackyCode 备选线路', 'packycode', 'claude',
       v.upstream_model, '/v1/messages', 'https://www.packyapi.ai',
       'REPLACE_ME_PACKYCODE_APIKEY', 'bearer', 'x-api-key',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, v.cost_rule::jsonb,
       110, 100, 120, 1, false
FROM models m
JOIN (VALUES
  ('claude-sonnet-5', 'claude-sonnet-5',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":14,"output_cost_per_m":70,"cache_read_cost_per_m":1.4,"cache_write_cost_per_m":17.5,
     "price_unit_normalized":true,"source_input_cost_per_m":14,"source_output_cost_per_m":70,
     "source_cache_read_cost_per_m":1.4,"source_cache_write_cost_per_m":17.5}'),
  ('claude-opus-5-5', 'claude-opus-5-5',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":28,"output_cost_per_m":140,"cache_read_cost_per_m":1.4,"cache_write_cost_per_m":35,
     "price_unit_normalized":true,"source_input_cost_per_m":28,"source_output_cost_per_m":140,
     "source_cache_read_cost_per_m":1.4,"source_cache_write_cost_per_m":35}'),
  ('claude-opus-4-8', 'claude-opus-4-8',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":35,"output_cost_per_m":175,"cache_read_cost_per_m":3.5,"cache_write_cost_per_m":43.75,
     "price_unit_normalized":true,"source_input_cost_per_m":35,"source_output_cost_per_m":175,
     "source_cache_read_cost_per_m":3.5,"source_cache_write_cost_per_m":43.75}'),
  ('claude-fable-5', 'claude-fable-5',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":70,"output_cost_per_m":350,"cache_read_cost_per_m":7,"cache_write_cost_per_m":87.5,
     "price_unit_normalized":true,"source_input_cost_per_m":70,"source_output_cost_per_m":350,
     "source_cache_read_cost_per_m":7,"source_cache_write_cost_per_m":87.5}'),
  ('claude-fable-5-1', 'claude-fable-5-1',
   '{"currency":"CREDITS","billing_type":"per_token","compute_per_cny":1,"source_currency":"CNY",
     "input_cost_per_m":70,"output_cost_per_m":350,"cache_read_cost_per_m":1.75,"cache_write_cost_per_m":87.5,
     "price_unit_normalized":true,"source_input_cost_per_m":70,"source_output_cost_per_m":350,
     "source_cache_read_cost_per_m":1.75,"source_cache_write_cost_per_m":87.5}')
) AS v(code, upstream_model, cost_rule) ON v.code = m.code;

-- ========= 4. Grok 图像：¥0.1/张，比 OpenRouter ¥0.288/张 便宜 65% =========
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url,
  api_key, auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule,
  priority, weight, timeout_seconds, max_retries, is_enabled
)
SELECT m.id, 'PackyCode 主线路', 'packycode', 'openai',
       'grok-imagine-image-2.0', '/v1/images/generations', 'https://www.packyapi.ai',
       'REPLACE_ME_PACKYCODE_APIKEY', 'bearer', 'Authorization',
       '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
       '{"currency":"CREDITS","billing_type":"per_image","compute_per_cny":1,"source_currency":"CNY",
         "unit_cost":0.1,"source_unit_cost":0.1,"price_unit_normalized":true,
         "unit_cost_by_variant":{"low_1k":0.1,"low_2k":0.2,"medium_1k":0.2,"medium_2k":0.2},
         "unit_cost_by_size":{},"unit_cost_by_resolution":{}}'::jsonb,
       2, 100, 180, 1, false
FROM models m WHERE m.code = 'grok-imagine-image-2-0';

-- ========= 5. GPT 三条线的本地 new_api 线路降为灾备（原 priority=0，实际无流量）=========
UPDATE model_routes r
SET priority = 90, updated_at = now()
FROM models m
WHERE m.id = r.model_id
  AND r.provider = 'new_api'
  AND m.code IN ('gpt-5-6-sol', 'gpt-6-astra', 'gpt-6-sol');

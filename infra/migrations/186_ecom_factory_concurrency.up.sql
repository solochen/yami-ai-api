-- 电商图片工厂：生成并发 8 → 3。
-- 上游（OpenRouter）按「在途请求总额度」校验，8 路并发会一次性占用过多预留额度，
-- 部分请求被以 "exceed your available credits given your current in-flight requests" 拒绝。
UPDATE workflow_definitions
SET runtime_config = jsonb_set(runtime_config, '{max_concurrency}', '3'::jsonb),
    updated_at = now()
WHERE code = 'ecom_factory';

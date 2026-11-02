-- 回滚：并发恢复 8。
UPDATE workflow_definitions
SET runtime_config = jsonb_set(runtime_config, '{max_concurrency}', '8'::jsonb),
    updated_at = now()
WHERE code = 'ecom_factory';

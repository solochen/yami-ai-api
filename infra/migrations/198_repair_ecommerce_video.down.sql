UPDATE workflow_definitions w
SET nodes = b.nodes,
    runtime_config = b.runtime_config,
    price_rule = b.price_rule,
    display_config = b.display_config,
    updated_at = now()
FROM ecommerce_video_repair_198_backup b
WHERE w.id = b.id;

DROP TABLE IF EXISTS ecommerce_video_repair_198_backup;

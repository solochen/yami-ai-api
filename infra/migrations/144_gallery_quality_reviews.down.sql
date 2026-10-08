DROP TABLE IF EXISTS gallery_quality_reviews;
DROP INDEX IF EXISTS uq_gallery_items_generation_task;
ALTER TABLE gallery_items DROP COLUMN IF EXISTS generation_task_id;

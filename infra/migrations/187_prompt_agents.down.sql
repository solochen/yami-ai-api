-- 回滚 187：移除 10 个内置提示词智能体与会话隔离字段

DELETE FROM workflow_definitions WHERE code IN (
  'video_script_writer',
  'ecom_sales_copy',
  'spoken_copy',
  'wechat_article',
  'moments_copy',
  'event_plan',
  'franchise_copy',
  'quotation_copy',
  'prompt_engineer',
  'visual_asset_extractor'
);

DROP INDEX IF EXISTS idx_conversations_agent;
ALTER TABLE conversations DROP COLUMN IF EXISTS agent_code;

-- 回滚 176：百炼线路还原为公共域名。

UPDATE model_routes
SET base_url = 'https://dashscope.aliyuncs.com/compatible-mode/v1', updated_at = now()
WHERE provider = 'bailian'
  AND base_url = 'https://ws-bzvsw2wexhbe3hpf.cn-beijing.maas.aliyuncs.com/compatible-mode/v1';

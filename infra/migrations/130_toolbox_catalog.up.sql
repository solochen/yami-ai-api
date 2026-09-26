UPDATE workflow_definitions AS w
SET display_config = COALESCE(w.display_config, '{}'::jsonb) || jsonb_build_object('toolbox', v.toolbox)
FROM (
  VALUES
    ('video_upscale', '{"enabled":true,"category":"video","subtitle":"提升清晰度并修复压缩瑕疵","tags":["超分","降噪"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('video_redraw', '{"enabled":true,"category":"video","subtitle":"保留动作，重绘画面风格","tags":["转绘","风格"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('subtitle_remove', '{"enabled":true,"category":"video","subtitle":"移除硬字幕并修复画面","tags":["去字幕","修复"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('viral_remake', '{"enabled":true,"category":"video","subtitle":"按爆款结构重做视频","tags":["爆款","复刻"],"badges":["recommended","available"],"billing":"按模型实际消耗"}'::jsonb),
    ('one_click_viral_remake', '{"enabled":true,"category":"video","subtitle":"一键拆解并重做爆款视频","tags":["一键","复刻"],"badges":["recommended","available"],"billing":"按模型实际消耗"}'::jsonb),
    ('video_remake', '{"enabled":true,"category":"video","subtitle":"上传成片并按新需求重做","tags":["重做","成片"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('ecommerce_video', '{"enabled":true,"category":"video","subtitle":"从商品信息生成带货短视频","tags":["带货","短视频"],"badges":["available"],"billing":"按次计费"}'::jsonb),
    ('ai_novel_workshop', '{"enabled":true,"category":"content","subtitle":"从选题写到可导出的长篇","tags":["小说","大纲"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('content_image_post', '{"enabled":true,"category":"content","subtitle":"一次整理标题、正文和配图","tags":["图文","多图"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('video_creation', '{"enabled":true,"category":"content","subtitle":"把故事做成可发布的短视频","tags":["短剧","分镜"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('video_creation_v2', '{"enabled":true,"category":"content","subtitle":"故事短视频的新一代制作流","tags":["短剧","成片"],"badges":["recommended","available"],"billing":"按模型实际消耗"}'::jsonb),
    ('ai_comic_drama', '{"enabled":true,"category":"content","subtitle":"剧本、分镜、关键帧到成片","tags":["漫剧","分镜"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('ai_photo_studio', '{"enabled":true,"category":"ai_create","subtitle":"上传照片，生成一整套写真","tags":["写真","人像"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('ai_virtual_tryon', '{"enabled":true,"category":"ai_create","subtitle":"人物照加服装图，生成试穿效果","tags":["试衣","商品"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('product_refine', '{"enabled":true,"category":"ai_create","subtitle":"按商品规则精修主图和细节","tags":["商品","精修"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('ecommerce_image', '{"enabled":true,"category":"image","subtitle":"商品图一键出主图和场景图","tags":["主图","电商"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('general_image', '{"enabled":true,"category":"image","subtitle":"用文字描述直接生成图片","tags":["生图","方案"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb),
    ('general_creative_agent', '{"enabled":true,"category":"assistant","subtitle":"一个入口完成对话、图片和视频创作","tags":["对话","创作"],"badges":["recommended","available"],"billing":"按模型实际消耗"}'::jsonb),
    ('infinite_canvas', '{"enabled":true,"category":"assistant","subtitle":"在画布上编排素材和生成步骤","tags":["画布","工作流"],"badges":["available"],"billing":"按模型实际消耗"}'::jsonb)
) AS v(code, toolbox)
WHERE w.code = v.code;

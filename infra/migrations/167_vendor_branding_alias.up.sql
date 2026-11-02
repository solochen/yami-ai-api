-- 境外模型代号化：对外展示与对外标识全部切换为自有代称，不再暴露原厂品牌。
--
-- 设计：
--   1. models 新增 public_alias（对外标识，唯一）。用户侧接口返回的 code 使用该别名，
--      请求侧接受别名并解析回内部 code；内部 code / new_api_model / 线路配置全部不动，
--      计费、日志、会话、任务关联不受影响。
--   2. display_name / description / tags / icon_url 一并改为自有代称与中性描述，
--      图标改为站内相对路径（/assets/brand/*.svg），不再引用 MinIO 里的原厂 logo。
--   3. 国产模型（DeepSeek / GLM / Qwen / 可灵 / Seedream / 海螺等）保持原名，不在此列。
--
-- 代号体系（两字母前缀，按厂商分组，型号后缀保留以便用户比价）：
--   HL = 太阳系旗舰文本/图像/语音       CT = 高阶推理与编程
--   NV = 多模态/视频/图像/音乐          PS = 图像与视频生成
--   MS = 音乐生成
--
-- 回滚：见 167_vendor_branding_alias.down.sql（依赖备份表恢复原始展示字段）。

-- ========= 0. 备份原始展示字段 =========
CREATE TABLE IF NOT EXISTS models_vendor_branding_backup_167 AS
SELECT id, code, display_name, description, tags, icon_url, now() AS backed_up_at
FROM models
WHERE code IN (
  'audio_gemini_tts','audio_openrouter_gpt_audio','audio_openrouter_gpt_audio_mini',
  'audio_openrouter_lyria_3_clip','audio_openrouter_lyria_3_pro','audio_suno',
  'claude-fable-5','claude-fable-5-1','claude-opus-4-8','claude-opus-5-5','claude-sonnet-5',
  'gemini-3-7-flash','gemini-3-8-flash',
  'gpt-5-6-luna','gpt-5-6-sol','gpt-6-astra','gpt-6-luna','gpt-6-sol',
  'gemini-2-5-flash-image','gemini-3-pro-image',
  'gpt-image-2','gpt-image-2-5-flare','gpt-image-2-5-sunburst','grok-imagine-image-2-0',
  'grok-imagine-video','grok-imagine-video-1-5',
  'veo-3-1','veo-3-1-fast','veo-3-1-lite'
);

-- ========= 1. 新增 public_alias 列 =========
ALTER TABLE models ADD COLUMN IF NOT EXISTS public_alias text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_models_public_alias
  ON models (public_alias) WHERE public_alias IS NOT NULL;

-- ========= 2. 音频（HL / NV / MS）=========
UPDATE models SET
  public_alias = 'nv-tts-3.1',
  display_name = 'NV TTS 3.1',
  description  = '旗舰级语音合成，提供 30 种音色与 24 种语言，可用自然语言描述情绪起伏与递进。',
  tags         = '["音频","TTS","NV 系列"]'::jsonb,
  icon_url     = '/assets/brand/nv.svg',
  updated_at   = now()
WHERE code = 'audio_gemini_tts';

UPDATE models SET
  public_alias = 'hl-audio',
  display_name = 'HL Audio',
  description  = '支持文本与参考音频输入，同时返回文本和音频；适合对话配音、声音改写、音频问答与多模态语音交互。',
  tags         = '["音频","TTS","HL 系列"]'::jsonb,
  icon_url     = '/assets/brand/hl.svg',
  updated_at   = now()
WHERE code = 'audio_openrouter_gpt_audio';

UPDATE models SET
  public_alias = 'hl-audio-mini',
  display_name = 'HL Audio Mini',
  description  = 'HL Audio 的轻量版，成本更低，支持文本与参考音频输入、文本与音频输出，适合批量语音生成与轻量音频任务。',
  tags         = '["音频","TTS","HL 系列"]'::jsonb,
  icon_url     = '/assets/brand/hl.svg',
  updated_at   = now()
WHERE code = 'audio_openrouter_gpt_audio_mini';

UPDATE models SET
  public_alias = 'nv-music-clip',
  display_name = 'NV Music Clip',
  description  = '轻量音乐生成模型，适合 30 秒音乐片段、广告钩子与短视频 BGM。',
  tags         = '["音频","音乐","NV 系列"]'::jsonb,
  icon_url     = '/assets/brand/nv.svg',
  updated_at   = now()
WHERE code = 'audio_openrouter_lyria_3_clip';

UPDATE models SET
  public_alias = 'nv-music-pro',
  display_name = 'NV Music Pro',
  description  = '完整歌曲生成，支持文本提示词与图片参考，可用于短片配乐与完整编曲。',
  tags         = '["音频","音乐","NV 系列"]'::jsonb,
  icon_url     = '/assets/brand/nv.svg',
  updated_at   = now()
WHERE code = 'audio_openrouter_lyria_3_pro';

UPDATE models SET
  public_alias = 'ms-4.5',
  display_name = 'MS 4.5',
  description  = '歌词生曲，支持歌曲与纯音乐两种模式，最长约 4 分钟，输出歌词与卡拉 OK 字幕。',
  tags         = '["音频","音乐","MS 系列"]'::jsonb,
  icon_url     = '/assets/brand/ms.svg',
  updated_at   = now()
WHERE code = 'audio_suno';

-- ========= 3. 对话 · CT（高阶推理与编程）=========
UPDATE models SET
  public_alias = 'ct-fable-5',
  display_name = 'CT Fable 5',
  description  = '面向自主知识工作与编程的高阶型号，支持文本、图片与文件输入、文本输出以及推理。',
  tags         = '["对话","CT 系列"]'::jsonb,
  icon_url     = '/assets/brand/ct.svg',
  updated_at   = now()
WHERE code = 'claude-fable-5';

UPDATE models SET
  public_alias = 'ct-fable-5.1',
  display_name = 'CT Fable 5.1',
  description  = '相对 CT Fable 5 全面提升，增益集中在智能体编程、长时间运行的智能体流程与知识工作，包括长代码重构。',
  tags         = '["对话","CT 系列"]'::jsonb,
  icon_url     = '/assets/brand/ct.svg',
  updated_at   = now()
WHERE code = 'claude-fable-5-1';

UPDATE models SET
  public_alias = 'ct-opus-4.8',
  display_name = 'CT Opus 4.8',
  description  = 'Opus 家族中能力最强的公开型号，支持文本、图片与文件输入、文本输出、推理，以及百万级上下文。',
  tags         = '["对话","CT 系列"]'::jsonb,
  icon_url     = '/assets/brand/ct.svg',
  updated_at   = now()
WHERE code = 'claude-opus-4-8';

UPDATE models SET
  public_alias = 'ct-opus-5.5',
  display_name = 'CT Opus 5.5',
  description  = '旗舰型号，面向高要求推理、编程与长程智能体工作，尤其擅长大型代码库中的复杂改造。',
  tags         = '["对话","CT 系列"]'::jsonb,
  icon_url     = '/assets/brand/ct.svg',
  updated_at   = now()
WHERE code = 'claude-opus-5-5';

UPDATE models SET
  public_alias = 'ct-sonnet-5',
  display_name = 'CT Sonnet 5',
  description  = '最强 Sonnet 档型号，在编程、智能体与专业工作上达到前沿水平，支持可调节深度的自适应思考。',
  tags         = '["对话","CT 系列"]'::jsonb,
  icon_url     = '/assets/brand/ct.svg',
  updated_at   = now()
WHERE code = 'claude-sonnet-5';

-- ========= 4. 对话 · NV（多模态快速档）=========
UPDATE models SET
  public_alias = 'nv-3.7-flash',
  display_name = 'NV 3.7 Flash',
  description  = '多模态快速型号，面向高频智能体流程、编程与复杂多步推理，强调响应速度与多步任务的可靠性。',
  tags         = '["对话","NV 系列"]'::jsonb,
  icon_url     = '/assets/brand/nv.svg',
  updated_at   = now()
WHERE code = 'gemini-3-7-flash';

UPDATE models SET
  public_alias = 'nv-3.8-flash',
  display_name = 'NV 3.8 Flash',
  description  = 'NV 系列最强 Flash 型号，相对 3.7 Flash 在软件工程、智能体任务与多步推理上有明显提升。',
  tags         = '["对话","NV 系列"]'::jsonb,
  icon_url     = '/assets/brand/nv.svg',
  updated_at   = now()
WHERE code = 'gemini-3-8-flash';

-- ========= 5. 对话 · HL（旗舰文本）=========
UPDATE models SET
  public_alias = 'hl-5.6-luna',
  display_name = 'HL 5.6 Luna',
  description  = '5.6 系列中的快速、低成本型号，适合高并发、对延迟敏感的对话、分类与轻量智能体流程。',
  tags         = '["对话","HL 系列"]'::jsonb,
  icon_url     = '/assets/brand/hl.svg',
  updated_at   = now()
WHERE code = 'gpt-5-6-luna';

UPDATE models SET
  public_alias = 'hl-5.6-sol',
  display_name = 'HL 5.6 Sol',
  description  = '5.6 系列旗舰，适合复杂推理、编程与智能体流程，尤其擅长命令行与多步编程任务。',
  tags         = '["对话","HL 系列"]'::jsonb,
  icon_url     = '/assets/brand/hl.svg',
  updated_at   = now()
WHERE code = 'gpt-5-6-sol';

UPDATE models SET
  public_alias = 'hl-6-astra',
  display_name = 'HL 6 Astra',
  description  = '面向端到端重任务的旗舰型号，适合高级分析、软件工程、深度研究、科学工作与文档创作。',
  tags         = '["对话","HL 系列"]'::jsonb,
  icon_url     = '/assets/brand/hl.svg',
  updated_at   = now()
WHERE code = 'gpt-6-astra';

UPDATE models SET
  public_alias = 'hl-6-luna',
  display_name = 'HL 6 Luna',
  description  = '6 系列中位于 Sol 之下的快速、低成本型号，适合高并发、对延迟敏感的对话、分类与轻量智能体任务。',
  tags         = '["对话","HL 系列"]'::jsonb,
  icon_url     = '/assets/brand/hl.svg',
  updated_at   = now()
WHERE code = 'gpt-6-luna';

UPDATE models SET
  public_alias = 'hl-6-sol',
  display_name = 'HL 6 Sol',
  description  = '6 系列中位于旗舰 Astra 之下、快速档 Luna 之上的高性价比型号，面向要求较高的专业任务。',
  tags         = '["对话","HL 系列"]'::jsonb,
  icon_url     = '/assets/brand/hl.svg',
  updated_at   = now()
WHERE code = 'gpt-6-sol';

-- ========= 6. 图像（NV / HL / PS）=========
UPDATE models SET
  public_alias = 'nv-image-2.5',
  display_name = 'NV Image 2.5',
  description  = '图像生成模型，具备上下文理解能力，可用于生成与编辑图片。',
  tags         = '["图片","NV 系列"]'::jsonb,
  icon_url     = '/assets/brand/nv.svg',
  updated_at   = now()
WHERE code = 'gemini-2-5-flash-image';

UPDATE models SET
  public_alias = 'nv-image-pro',
  display_name = 'NV Image Pro',
  description  = '基于 NV 3 Pro 的图像生成与编辑模型，加强了多模态推理与现实场景理解。',
  tags         = '["图片","NV 系列"]'::jsonb,
  icon_url     = '/assets/brand/nv.svg',
  updated_at   = now()
WHERE code = 'gemini-3-pro-image';

UPDATE models SET
  public_alias = 'hl-image-2',
  display_name = 'HL Image 2',
  description  = '图像生成模型，通过图片接口支持高保真生成与编辑。',
  tags         = '["图片","HL 系列"]'::jsonb,
  icon_url     = '/assets/brand/hl.svg',
  updated_at   = now()
WHERE code = 'gpt-image-2';

UPDATE models SET
  public_alias = 'hl-image-2.5-flare',
  display_name = 'HL Image 2.5 Flare',
  description  = '偏重速度的图像生成与编辑型号，适合大批量日常出图。',
  tags         = '["图片","HL 系列"]'::jsonb,
  icon_url     = '/assets/brand/hl.svg',
  updated_at   = now()
WHERE code = 'gpt-image-2-5-flare';

UPDATE models SET
  public_alias = 'hl-image-2.5-sunburst',
  display_name = 'HL Image 2.5 Sunburst',
  description  = '偏重精度的图像生成与编辑型号，适合细节要求高的创作。',
  tags         = '["图片","HL 系列"]'::jsonb,
  icon_url     = '/assets/brand/hl.svg',
  updated_at   = now()
WHERE code = 'gpt-image-2-5-sunburst';

UPDATE models SET
  public_alias = 'ps-imagine-2.0',
  display_name = 'PS Imagine 2.0',
  description  = '图像生成与编辑模型，可根据文字提示出图，也可按参考图编辑。',
  tags         = '["图片","PS 系列"]'::jsonb,
  icon_url     = '/assets/brand/ps.svg',
  updated_at   = now()
WHERE code = 'grok-imagine-image-2-0';

-- ========= 7. 视频（PS / NV）=========
UPDATE models SET
  public_alias = 'ps-video',
  display_name = 'PS Video',
  description  = '快速视频模型，支持文字、图片与参考条件，时长 1 到 15 秒。',
  tags         = '["视频","PS 系列"]'::jsonb,
  icon_url     = '/assets/brand/ps.svg',
  updated_at   = now()
WHERE code = 'grok-imagine-video';

UPDATE models SET
  public_alias = 'ps-video-1.5',
  display_name = 'PS Video 1.5',
  description  = '视频模型，可用文字生成，也可用一张起始图引导画面。',
  tags         = '["视频","PS 系列"]'::jsonb,
  icon_url     = '/assets/brand/ps.svg',
  updated_at   = now()
WHERE code = 'grok-imagine-video-1-5';

UPDATE models SET
  public_alias = 'nv-video-3.1',
  display_name = 'NV Video 3.1',
  description  = '面向成片画质的视频模型，可由文字或图片生成高清视频。',
  tags         = '["视频","NV 系列"]'::jsonb,
  icon_url     = '/assets/brand/nv.svg',
  updated_at   = now()
WHERE code = 'veo-3-1';

UPDATE models SET
  public_alias = 'nv-video-3.1-fast',
  display_name = 'NV Video 3.1 Fast',
  description  = '速度与画质折中的视频模型，可由文字或图片生成，并带同步声音。',
  tags         = '["视频","NV 系列"]'::jsonb,
  icon_url     = '/assets/brand/nv.svg',
  updated_at   = now()
WHERE code = 'veo-3-1-fast';

UPDATE models SET
  public_alias = 'nv-video-3.1-lite',
  display_name = 'NV Video 3.1 Lite',
  description  = '成本更低的视频模型，适合大批量与快速试片，可由文字或图片生成。',
  tags         = '["视频","NV 系列"]'::jsonb,
  icon_url     = '/assets/brand/nv.svg',
  updated_at   = now()
WHERE code = 'veo-3-1-lite';

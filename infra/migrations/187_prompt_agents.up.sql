-- 通用「提示词对话智能体」运行时（agent_mode = prompt_chat）
-- 1) 会话按智能体隔离：conversations.agent_code
-- 2) 10 个内置智能体的定义（提示词正文在 Go 侧 agent_prompt_chat_packs.go，
--    这里的 greeting / quick_prompts 等可在后台直接改库覆盖）

ALTER TABLE conversations ADD COLUMN IF NOT EXISTS agent_code VARCHAR(64);
CREATE INDEX IF NOT EXISTS idx_conversations_agent
  ON conversations(user_id, agent_code, updated_at DESC)
  WHERE agent_code IS NOT NULL;

-- 历史装修预算会话：标题固定前缀（旧实现用前缀做 scope 过滤）
UPDATE conversations SET agent_code = 'renovation_budget_agent'
 WHERE agent_code IS NULL AND title ~ '^装修预算助理';

INSERT INTO workflow_definitions (
  code, name, description, icon, category, nodes, input_schema, price_rule,
  display_config, runtime_config, is_enabled, sort_order, created_at, updated_at
) VALUES
(
  'video_script_writer',
  '短视频脚本编剧',
  '把你的视频创意写成可直接开拍的脚本：标题、分镜表、完整口播稿与拍摄贴士。',
  '🎬', 'tool', '[]'::jsonb,
  '{"type":"object","properties":{"prompt":{"type":"string","title":"视频创意"}}}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  '{
    "theme": "rose",
    "hero_tags": ["官方内置", "多轮对话", "结构化输出"],
    "toolbox": {
      "enabled": true,
      "category": "content",
      "subtitle": "标题·分镜·口播，一次成稿",
      "tags": ["分镜", "口播", "带货"],
      "badges": ["recommended", "available"],
      "billing": "按实际消耗"
    }
  }'::jsonb,
  jsonb_build_object(
    'agent_mode', 'prompt_chat',
    'chat_model_code', COALESCE(
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.code='deepseek-v4-1-flash-1017' AND m.category='chat' LIMIT 1),
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.category='chat' ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
      'deepseek-v4-1-flash-1017'),
    'greeting', '把你的视频创意用一两句话告诉我——产品是什么、发哪个平台、要多长，我就给你输出标题、分镜表和能直接念的完整脚本。',
    'quick_prompts', jsonb_build_array(
      '30 秒抖音带货短视频，卖冬季羽绒服，主打轻暖不臃肿',
      '15 秒小红书探店视频，拍一家工作日午休的咖啡店',
      '45 秒科普视频，讲清家用净水器的换芯成本',
      '30 秒职场通勤穿搭分享，人群是 25-35 岁女性'),
    'input_placeholder', '描述你的视频：产品/主题、平台、时长，例如：30 秒抖音带货，卖保温杯，主打 12 小时保温',
    'disclaimer', '脚本中的价格、参数、功效等占位内容需按商品实际信息替换；发布前请自行核对广告法合规。',
    'max_input_chars', 4000,
    'max_tokens', 6000,
    'temperature', 0.7,
    'history_limit', 40
  ),
  true, 101, now(), now()
),
(
  'ecom_sales_copy',
  '电商带货文案',
  '输入产品信息，生成痛点、场景、对比三版促单文案与直播口播话术。',
  '🛒', 'tool', '[]'::jsonb,
  '{"type":"object","properties":{"prompt":{"type":"string","title":"产品信息"}}}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  '{
    "theme": "amber",
    "hero_tags": ["官方内置", "三版打法", "合规自检"],
    "toolbox": {
      "enabled": true,
      "category": "content",
      "subtitle": "三版文案·直播口播",
      "tags": ["带货", "促单", "口播"],
      "badges": ["recommended", "available"],
      "billing": "按实际消耗"
    }
  }'::jsonb,
  jsonb_build_object(
    'agent_mode', 'prompt_chat',
    'chat_model_code', COALESCE(
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.code='deepseek-v4-1-flash-1017' AND m.category='chat' LIMIT 1),
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.category='chat' ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
      'deepseek-v4-1-flash-1017'),
    'greeting', '把产品告诉我（品类、卖点、目标人群、价格），我给你三版不同打法的带货文案，外加一段能直接念的直播口播。',
    'quick_prompts', jsonb_build_array(
      '保温杯，59 元，12 小时保温，目标人群是通勤上班族',
      '全价猫粮，主打鲜肉低敏，面向养猫新手',
      '玻尿酸精华，主打补水不黏腻，面向 25-35 岁女性',
      '扫地机器人，2000 元档，主打自清洁和宠物毛发'),
    'input_placeholder', '输入产品：品类、核心卖点、价格、目标人群，例如：保温杯 59 元，12 小时保温，卖给通勤族',
    'disclaimer', '价格、优惠、销量、资质等未提供的信息会以占位符标注，发布前请替换为真实信息，避免虚假宣传。',
    'max_input_chars', 4000,
    'max_tokens', 6000,
    'temperature', 0.7,
    'history_limit', 40
  ),
  true, 102, now(), now()
),
(
  'spoken_copy',
  '口播文案工厂',
  '带货、科普、观点三类口播稿，带停顿与重音标记，附提词器纯文稿。',
  '🎙️', 'tool', '[]'::jsonb,
  '{"type":"object","properties":{"prompt":{"type":"string","title":"口播主题"}}}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  '{
    "theme": "violet",
    "hero_tags": ["官方内置", "停顿标记", "提词器版"],
    "toolbox": {
      "enabled": true,
      "category": "content",
      "subtitle": "带货·科普·观点三类口播",
      "tags": ["口播", "提词器", "短视频"],
      "badges": ["recommended", "available"],
      "billing": "按实际消耗"
    }
  }'::jsonb,
  jsonb_build_object(
    'agent_mode', 'prompt_chat',
    'chat_model_code', COALESCE(
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.code='deepseek-v4-1-flash-1017' AND m.category='chat' LIMIT 1),
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.category='chat' ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
      'deepseek-v4-1-flash-1017'),
    'greeting', '告诉我要讲什么、讲给谁听、发在哪个平台，我给你一份带停顿和重音标记的口播稿，再附一版直接进提词器的纯文稿。',
    'quick_prompts', jsonb_build_array(
      '科普：为什么手机充电越来越快，面向普通用户',
      '带货：一款 199 元的电动牙刷，主打敏感牙龈',
      '观点：为什么年轻人开始重新用现金',
      '带货：端午粽子礼盒，面向企业采购'),
    'input_placeholder', '输入主题、平台、时长和目标人群，例如：科普视频 60 秒，讲睡眠不足对皮肤的影响，面向 20-35 岁',
    'disclaimer', '涉及医疗、金融、法律等专业内容的口播稿仅供参考，发布前请由专业人士审核。',
    'max_input_chars', 4000,
    'max_tokens', 6000,
    'temperature', 0.7,
    'history_limit', 40
  ),
  true, 103, now(), now()
),
(
  'wechat_article',
  '公众号长文创作',
  '输入主题或标题，生成多版标题、大纲、完整正文、金句与配图建议。',
  '📰', 'tool', '[]'::jsonb,
  '{"type":"object","properties":{"prompt":{"type":"string","title":"文章主题"}}}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  '{
    "theme": "sky",
    "hero_tags": ["官方内置", "长文成稿", "发布清单"],
    "toolbox": {
      "enabled": true,
      "category": "content",
      "subtitle": "标题·大纲·正文·金句",
      "tags": ["长文", "公众号", "选题"],
      "badges": ["recommended", "available"],
      "billing": "按实际消耗"
    }
  }'::jsonb,
  jsonb_build_object(
    'agent_mode', 'prompt_chat',
    'chat_model_code', COALESCE(
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.code='deepseek-v4-1-flash-1017' AND m.category='chat' LIMIT 1),
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.category='chat' ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
      'deepseek-v4-1-flash-1017'),
    'greeting', '把主题、目标读者和想要的角度告诉我，我给你 5 个标题、一份大纲、1500 字左右的完整正文，还有金句和配图建议。',
    'quick_prompts', jsonb_build_array(
      '写一篇关于小户型收纳的实用文章，面向租房年轻人',
      '写一篇公司年度复盘文章，面向内部员工',
      '写一篇 AI 工具提升办公效率的科普文章，面向职场新人',
      '写一篇秋季养生饮食指南，面向 35 岁以上人群'),
    'input_placeholder', '输入文章主题、目标读者、期望角度和字数，例如：2000 字，讲小户型收纳，读者是租房年轻人',
    'disclaimer', '文中数据、引用、案例会标注待补位置，发布前请核实来源；健康与投资类表述不构成专业建议。',
    'max_input_chars', 6000,
    'max_tokens', 8000,
    'temperature', 0.7,
    'history_limit', 40
  ),
  true, 104, now(), now()
),
(
  'moments_copy',
  '朋友圈文案',
  '一次生成 8 条分场景朋友圈文案，附配图建议与合规自检。',
  '💬', 'tool', '[]'::jsonb,
  '{"type":"object","properties":{"prompt":{"type":"string","title":"发圈内容"}}}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  '{
    "theme": "emerald",
    "hero_tags": ["官方内置", "8 条起发", "合规自检"],
    "toolbox": {
      "enabled": true,
      "category": "content",
      "subtitle": "日常·种草·见证·节点",
      "tags": ["短文案", "朋友圈", "私域"],
      "badges": ["recommended", "available"],
      "billing": "按实际消耗"
    }
  }'::jsonb,
  jsonb_build_object(
    'agent_mode', 'prompt_chat',
    'chat_model_code', COALESCE(
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.code='deepseek-v4-1-flash-1017' AND m.category='chat' LIMIT 1),
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.category='chat' ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
      'deepseek-v4-1-flash-1017'),
    'greeting', '告诉我你要发什么、给谁看，我给你 8 条分场景的朋友圈文案，配上配图建议，还有一版合规自检。',
    'quick_prompts', jsonb_build_array(
      '周末露营装备分享，卖户外用品，客户是 30 岁左右家庭',
      '新店开业，社区烘焙店，主打当日现烤',
      '客户反馈：用了三个月减脂餐的感受',
      '中秋节给老客户发一条维护关系的文案'),
    'input_placeholder', '输入你要发的内容与人群，例如：新店开业，社区烘焙店，想让附近住户来尝鲜',
    'disclaimer', '客户见证类文案需取得当事人授权；涉及优惠与资质请填写真实信息。',
    'max_input_chars', 3000,
    'max_tokens', 6000,
    'temperature', 0.8,
    'history_limit', 40
  ),
  true, 105, now(), now()
),
(
  'event_plan',
  '活动策划方案',
  '从活动目标到执行排期，输出含预算表与风险预案的完整策划方案。',
  '🎯', 'tool', '[]'::jsonb,
  '{"type":"object","properties":{"prompt":{"type":"string","title":"活动需求"}}}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  '{
    "theme": "fuchsia",
    "hero_tags": ["官方内置", "可执行排期", "预算与预案"],
    "toolbox": {
      "enabled": true,
      "category": "assistant",
      "subtitle": "目标·流程·预算·预案",
      "tags": ["策划", "排期", "预算"],
      "badges": ["recommended", "available"],
      "billing": "按实际消耗"
    }
  }'::jsonb,
  jsonb_build_object(
    'agent_mode', 'prompt_chat',
    'chat_model_code', COALESCE(
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.code='deepseek-v4-1-flash-1017' AND m.category='chat' LIMIT 1),
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.category='chat' ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
      'deepseek-v4-1-flash-1017'),
    'greeting', '告诉我活动目标、预算、时间和参与人数，我给你一份能直接执行的方案：流程时间轴、物料分工、预算表和风险预案。',
    'quick_prompts', jsonb_build_array(
      '社区商场周末亲子市集：目标拉新 500 组家庭，预算 3 万，本周六日两天，商场中庭，预计 800 人次，主办方是商场运营部',
      '公司 50 人年会：预算 8 万，周六晚，市区酒店宴会厅，含颁奖与抽奖，主办方是行政部',
      '新品牌线上发布会：预算 5 万，周三晚八点直播，面向行业媒体与经销商，预计 300 人在线',
      '门店开业活动：预算 1 万，国庆首日，社区底商门口，预计 200 人到店，主办方是门店'),
    'input_placeholder', '输入活动目标、预算、时间、地点、人数，例如：商场亲子市集，3 万预算，周六日两天，预计 500 人次',
    'disclaimer', '预算单价为参考区间，需按当地实际报价核定；涉及消防、报批、未成年人参与的活动请提前办理合规手续。',
    'max_input_chars', 6000,
    'max_tokens', 8000,
    'temperature', 0.6,
    'history_limit', 40
  ),
  true, 106, now(), now()
),
(
  'franchise_copy',
  '招商文案起草',
  '生成招商会方案、手册大纲、加盟政策与电话话术，附合规提示。',
  '🤝', 'tool', '[]'::jsonb,
  '{"type":"object","properties":{"prompt":{"type":"string","title":"招商项目"}}}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  '{
    "theme": "amber",
    "hero_tags": ["官方内置", "招商会+话术", "收益合规提示"],
    "toolbox": {
      "enabled": true,
      "category": "assistant",
      "subtitle": "招商会·手册·电话话术",
      "tags": ["招商", "加盟", "话术"],
      "badges": ["available"],
      "billing": "按实际消耗"
    }
  }'::jsonb,
  jsonb_build_object(
    'agent_mode', 'prompt_chat',
    'chat_model_code', COALESCE(
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.code='deepseek-v4-1-flash-1017' AND m.category='chat' LIMIT 1),
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.category='chat' ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
      'deepseek-v4-1-flash-1017'),
    'greeting', '告诉我项目、目标加盟商画像和现有政策，我给你招商会方案、手册大纲、电话话术和异议应答，并标出不能承诺的地方。',
    'quick_prompts', jsonb_build_array(
      '奶茶品牌招商：30 平米小店，单店投入 25 万，加盟费 3 万，目标县城创业者，暂无样板店数据',
      '少儿编程加盟：150 平米校区，投入 60 万，目标教培转型机构，有 2 家直营校数据',
      '干洗店招商：设备可分 24 期，单店投入 15 万，目标夫妻店创业者',
      '社区生鲜加盟：主打供应链与日配，单店 80 平米，目标本地个体商户'),
    'input_placeholder', '输入项目类型、加盟模式、目标人群，例如：奶茶加盟，30 平米小店，目标县城创业者',
    'disclaimer', '招商内容不得承诺收益或回本周期；测算表中的假设条件必须如实披露，正式招商前请准备资质与合同文本。',
    'max_input_chars', 6000,
    'max_tokens', 8000,
    'temperature', 0.6,
    'history_limit', 40
  ),
  true, 107, now(), now()
),
(
  'quotation_copy',
  '报价单文案',
  '把产品和服务清单整理成专业报价单，含明细表、条款与发送话术。',
  '🧾', 'tool', '[]'::jsonb,
  '{"type":"object","properties":{"prompt":{"type":"string","title":"服务清单"}}}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  '{
    "theme": "sky",
    "hero_tags": ["官方内置", "明细可核对", "商务条款"],
    "toolbox": {
      "enabled": true,
      "category": "assistant",
      "subtitle": "明细表·条款·发送话术",
      "tags": ["报价", "商务", "清单"],
      "badges": ["available"],
      "billing": "按实际消耗"
    }
  }'::jsonb,
  jsonb_build_object(
    'agent_mode', 'prompt_chat',
    'chat_model_code', COALESCE(
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.code='deepseek-v4-1-flash-1017' AND m.category='chat' LIMIT 1),
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.category='chat' ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
      'deepseek-v4-1-flash-1017'),
    'greeting', '把你的项目和服务清单发给我（项目、数量、单价），我整理成一份专业报价单：明细表、合计、服务范围、付款条款，还有一段发送话术。',
    'quick_prompts', jsonb_build_array(
      '家装半包报价，90 平，含水电、瓦工、油漆',
      '小程序开发报价，含设计、开发、上线维护',
      '企业年会视频拍摄报价，含策划、拍摄、剪辑',
      '办公室保洁年度服务报价，每周两次'),
    'input_placeholder', '输入服务项目与数量，例如：小程序开发，含 UI 设计、前后端、上线部署、一年维护',
    'disclaimer', '未提供单价的项目以占位符标注；合计金额请复核；税费、运费、有效期等条款请按实际情况确认。',
    'max_input_chars', 6000,
    'max_tokens', 8000,
    'temperature', 0.5,
    'history_limit', 40
  ),
  true, 108, now(), now()
),
(
  'prompt_engineer',
  'AI 提示词工程师',
  '说一件事，生成可直接复制的六段式提示词，附示例与变体。',
  '✨', 'tool', '[]'::jsonb,
  '{"type":"object","properties":{"prompt":{"type":"string","title":"目标任务"}}}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  '{
    "theme": "violet",
    "hero_tags": ["官方内置", "可直接复制", "示例+变体"],
    "toolbox": {
      "enabled": true,
      "category": "ai_create",
      "subtitle": "角色·任务·约束·示例",
      "tags": ["提示词", "AI", "效率"],
      "badges": ["recommended", "available"],
      "billing": "按实际消耗"
    }
  }'::jsonb,
  jsonb_build_object(
    'agent_mode', 'prompt_chat',
    'chat_model_code', COALESCE(
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.code='deepseek-v4-1-flash-1017' AND m.category='chat' LIMIT 1),
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.category='chat' ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
      'deepseek-v4-1-flash-1017'),
    'greeting', '告诉我你想让 AI 帮你做什么、给谁用、输出要什么格式，我给你一份能直接复制的完整提示词，再附 3 个示例和两个变体版本。',
    'quick_prompts', jsonb_build_array(
      '帮我写一个小红书爆款标题生成器的提示词',
      '帮我写一个把会议记录整理成待办的提示词',
      '帮我写一个代码审查助手的提示词',
      '帮我写一个企业客服话术训练的提示词'),
    'input_placeholder', '描述你想让 AI 完成的任务，例如：把用户评论分类成好评/差评/建议，并给出回复建议',
    'disclaimer', '提示词效果受模型能力影响，建议先用小样本测试再批量使用。',
    'max_input_chars', 4000,
    'max_tokens', 6000,
    'temperature', 0.6,
    'history_limit', 40
  ),
  true, 109, now(), now()
),
(
  'visual_asset_extractor',
  '视觉资产提取器',
  '输入剧本或小说片段，提取人物、场景、道具资产表，并生成可直接出图的提示词。',
  '🎨', 'tool', '[]'::jsonb,
  '{"type":"object","properties":{"prompt":{"type":"string","title":"剧本片段"}}}'::jsonb,
  '{"billing_type":"model_actual","unit_price":0}'::jsonb,
  '{
    "theme": "rose",
    "hero_tags": ["官方内置", "资产表", "可出图提示词"],
    "toolbox": {
      "enabled": true,
      "category": "ai_create",
      "subtitle": "人物·场景·道具·出图词",
      "tags": ["分镜", "出图", "资产"],
      "badges": ["recommended", "available"],
      "billing": "按实际消耗"
    }
  }'::jsonb,
  jsonb_build_object(
    'agent_mode', 'prompt_chat',
    'chat_model_code', COALESCE(
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.code='deepseek-v4-1-flash-1017' AND m.category='chat' LIMIT 1),
      (SELECT m.code FROM models m WHERE m.is_enabled=true AND m.category='chat' ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1),
      'deepseek-v4-1-flash-1017'),
    'greeting', '把剧本或小说片段贴给我，我按人物、场景、道具整理成资产表，每一项配好可以直接拿去出图的提示词。',
    'quick_prompts', jsonb_build_array(
      '现代都市短剧第 1 集：女主在便利店值夜班遇到男主',
      '古风仙侠片段：少年在瀑布下练剑，师门长老出现',
      '校园青春片段：开学第一天，新生在教室自我介绍',
      '悬疑片段：雨夜的老旧公寓楼道，停电'),
    'input_placeholder', '粘贴剧本或小说片段（建议 500-3000 字），我会提取人物、场景、道具并生成出图提示词',
    'disclaimer', '资产特征只依据原文，不做额外发挥；出图提示词避免使用真实明星姓名和受版权保护的形象。',
    'max_input_chars', 12000,
    'max_tokens', 8000,
    'temperature', 0.6,
    'history_limit', 40
  ),
  true, 110, now(), now()
)
ON CONFLICT (code) DO NOTHING;

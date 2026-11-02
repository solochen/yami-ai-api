# 提示词对话智能体（prompt_chat）实现与验收说明

复刻对象：qqwapi.cn 官方内置的 10 个文本型智能体。
落地形态：本站新的通用运行时 `agent_mode = prompt_chat`，10 个智能体 = 10 条 DB 配置 + 10 份 Go 提示词包。

## 一、为什么新增通用运行时

本站原有智能体（电商工厂、装修预算、虚拟试穿等）每个都是「独立 agent_mode + 独立后端服务 + 独立前端工作区」，适合多步任务编排。qqwapi 这 10 个智能体本质是「一段系统提示词 + 多轮对话」，若逐个照搬会产生 10 套重复代码。

新增的 `prompt_chat` 运行时把共性收敛为一份实现：

| 层 | 文件 | 职责 |
|---|---|---|
| 提示词包 | `services/api/internal/service/agent_prompt_chat_packs.go` | 10 份系统提示词（角色/步骤/输出结构/边界）+ 统一输出纪律 |
| 运行时 | `services/api/internal/service/agent_prompt_chat.go` | 读取配置、会话隔离、历史拼装、流式生成、结算、落库、重跑 |
| 接口层 | `services/api/internal/handler/agent_prompt_chat.go` | SSE 事件协议、历史 CRUD、错误映射 |
| 前端 | `apps/web/src/components/workbench/PromptAgentWorkspace.tsx` | 落地页、对话、历史、消息操作、移动端与暗色 |
| 配置 | `infra/migrations/187_prompt_agents.up.sql` | `conversations.agent_code` + 10 条 `workflow_definitions` |

提示词以 Go 常量保存（可测试、可版本化），展示文案（开场白/快捷问题/占位符/免责声明）存 DB，后台改库即可热更；`runtime_config.system_prompt` 可整体覆盖提示词。

## 二、数据模型

`workflow_definitions` 新增 10 行，均为：

- `category = 'tool'`
- `price_rule = {"billing_type":"model_actual","unit_price":0}`（按 token 实际消耗）
- `display_config.toolbox = { enabled, category, subtitle, tags, badges, billing }`
- `runtime_config`：
  - `agent_mode = prompt_chat`
  - `chat_model_code`（默认 `deepseek-v4-1-flash-1017`，迁移里用 COALESCE 兜底到任一可用 chat 模型）
  - `greeting` / `quick_prompts`（4 条）/ `input_placeholder` / `disclaimer`
  - `max_input_chars` / `max_tokens` / `temperature` / `history_limit`

`conversations` 新增 `agent_code`，会话按智能体隔离（历史列表、跨智能体访问校验都依赖它），旧装修预算会话按标题前缀回填。

## 三、10 个智能体的输入契约与输出结构

所有智能体共用「通用输出纪律」：直接给成品、固定章节不可改名、未提供事实用 `__` 占位、不编造、不输出绝对化用语、违规请求拒绝并给替代方向。

| 智能体 | code | 分类 | 输入 | 固定输出章节 | 输出上限 |
|---|---|---|---|---|---|
| 短视频脚本编剧 | `video_script_writer` | 内容创作 | 产品/主题、平台、时长 | 一句话创意 / 视频标题 / 视频信息 / 分镜脚本 / 完整口播稿 / 拍摄与剪辑贴士 / 需要你补充 | 6000 |
| 电商带货文案 | `ecom_sales_copy` | 内容创作 | 产品、卖点、价格、人群 | 卖点提炼 / 痛点版 / 场景版 / 对比版 / 直播口播稿 / 话题标签 / 合规自检 / 需要你补充 | 6000 |
| 口播文案工厂 | `spoken_copy` | 内容创作 | 主题、平台、时长、人群 | 选题角度 / 口播稿（带停顿重音标记）/ 提词器版 / 标题与封面文案 / 节奏与拍摄建议 / 需要你补充 | 6000 |
| 公众号长文创作 | `wechat_article` | 内容创作 | 主题、读者、角度、字数 | 标题 / 摘要 / 文章大纲 / 正文 / 金句 / 配图建议 / 发布检查清单 / 需要你补充 | 8000 |
| 朋友圈文案 | `moments_copy` | 内容创作 | 发什么、给谁看、目的 | 使用建议 / 8 条文案（四类）/ 每条配图建议 / 互动引导 / 合规自检 / 需要你补充 | 6000 |
| 活动策划方案 | `event_plan` | AI 助手 | 目标、预算、时间、人数 | 活动概览 / 目标与指标 / 人群洞察 / 创意玩法 / 流程 / 物料分工 / 预算表 / 传播排期 / 风险预案 / 复盘指标 / 需要你补充 | 8000 |
| 招商文案起草 | `franchise_copy` | AI 助手 | 项目、单店模型、政策、人群 | 招商会方案 / 手册目录 / 卖点差异化 / 政策与收益测算 / 电话微信话术 / 异议应答 / 合规提示 / 需要你补充 | 8000 |
| 报价单文案 | `quotation_copy` | AI 助手 | 项目、数量、单价、条款 | 报价单抬头 / 项目明细表 / 合计与优惠 / 服务范围 / 交付与付款 / 变更与售后 / 发送话术 / 需要你补充 | 8000 |
| AI 提示词工程师 | `prompt_engineer` | AI 创作 | 目标任务、使用者、输出格式 | 需求理解 / 主提示词（六段式）/ 变量说明 / 使用示例 / 变体版本 / 迭代建议 / 需要你补充 | 6000 |
| 视觉资产提取器 | `visual_asset_extractor` | AI 创作 | 剧本或小说片段 | 资产总览 / 人物设定 / 场景设定 / 道具 / 统一风格锚点 / 出图使用说明 / 需要你补充 | 8000 |

边界规则（写进每个提示词）：
- 信息不足时先按行业常规假设输出，并在开头一行声明假设；仅当完全无法判断主题时才反问（最多 3 个问题）。
- 价格、参数、销量、资质、案例一律占位并列入「需要你补充」。
- 广告法：禁用「最/第一/国家级」等绝对化用语；医疗、金融、法律内容声明不构成专业建议。
- 招商类禁止承诺收益与回本周期；报价单合计必须与明细一致。

## 四、接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/agents/:code/chat` | 一轮对话，SSE：`meta` / `delta` / `done` / `error` |
| GET | `/api/agents/:code/conversations` | 历史会话（按 agent_code 隔离，过滤空会话） |
| GET | `/api/agents/:code/conversations/:id` | 会话详情（消息带 id，供编辑/重跑） |
| PATCH | `/api/agents/:code/conversations/:id` | 重命名（≤60 字） |
| DELETE | `/api/agents/:code/conversations/:id` | 删除会话 |
| POST | `/api/agents/:code/conversations/:id/messages/:message_id/rerun` | 从某条用户消息重跑；带 `content` 即「编辑并重跑」，服务端截断后续轮次 |
| DELETE | `/api/agents/:code/conversations/:id/messages/:message_id` | 删除单条消息 |

`done` 事件返回 `conversation_id / message_id / user_message_id / content / usage`。

## 五、实现要点

1. **计费**：走 `ChatService.CompletionStreamRaw` + `FinalizeStream`，冻结→结算按 token 实际扣费；账单标签为智能体名称。
2. **取消**：客户端断开即取消上游并释放冻结额度，不落库；已创建但无消息的空会话自动清理。
3. **错误脱敏**：上游英文错误（如 `unexpected EOF`）统一转「模型服务暂时不可用，请稍后重试」，中文业务错误原样透出；不暴露上游域名/厂商。
4. **输入校验**：空消息 400、超长 400（按 `max_input_chars`）、未登录 401、智能体停用 400、跨智能体访问 404、余额不足返回 `INSUFFICIENT_BALANCE` 并给充值入口。
5. **多语言**：新增 UI 文案补齐 en-US / ja-JP / ko-KR / vi-VN；开场白、快捷问题、占位符、免责声明纳入内容翻译目录（`/runtime_config/greeting` 等 JSON Pointer），可在后台「内容翻译」一键生成，未翻译时回退中文。
6. **渲染**：复用 `AgentRichText`（标题/列表/表格/代码块）；表格在窄屏改为横向滚动（`min-w-[560px]`），不再挤压列宽。
7. **顺带修复**：`TestEcomFactoryPlanCount` 详情套装期望值随 185 迁移的 8 模块更新为 16；补齐 ja/ko/vi 缺失的 16 个字典键（`common.recommended`、`agentCategory.*`、`recharge.*`），恢复 CI 前端测试绿灯。

## 六、验收记录（2026-11-02，本地 docker 环境）

| 项目 | 结果 |
|---|---|
| 迁移 187 up/down（事务内演练 + 实际执行） | 通过，可回滚 |
| 10 个智能体出现在 `/api/agents` 与侧栏分类 | 通过（内容创作 5 / AI 助手 3 / AI 创作 2） |
| 10 个智能体输出章节契约（mock 线路逐条实测） | 10/10 通过（见下） |
| SSE 流式 + 停止 + 空会话清理 | 通过 |
| 历史列表 / 详情 / 重命名 / 删除会话 | 通过 |
| 重新生成 / 编辑并重跑（服务端截断） | 通过（message_id 88→89→90 验证） |
| 边界：空消息、超长、未登录、未知智能体、跨智能体会话 | 400 / 400 / 401 / 400 / 404 |
| 前端类型检查与 lint | 通过 |
| 前端 CI 测试集（144 项） | 全部通过 |
| Go `go test ./internal/...` | 全部通过 |
| 桌面暗色 / 移动端 / 表格渲染 | 截图验收通过 |

输出契约实测（每条消息均检查固定章节是否齐全）：

| 智能体 | 字数 | 结果 |
|---|---|---|
| 短视频脚本编剧 | 1424 | 7/7 章节 |
| 电商带货文案 | 1324 | 8/8 |
| 口播文案工厂 | 1028 | 6/6 |
| 公众号长文创作 | 2647 | 8/8 |
| 朋友圈文案 | 1329 | 6/6 |
| 活动策划方案 | 4037 | 11/11 |
| 招商文案起草 | 4060 | 8/8 |
| 报价单文案 | 1630 | 8/8 |
| AI 提示词工程师 | 2437 | 7/7 |
| 视觉资产提取器 | 2190 | 7/7 |

## 七、后续建议

- 在后台「内容翻译」对 10 个智能体执行一次自动翻译，让非中文用户的快捷问题与免责声明跟随语言切换。
- 如需把「视觉资产提取器」产出直接接到本站出图，可在后续版本增加「一键生图」按钮（当前输出已按本站图像提示词惯例编写）。
- 第二批候选：产品Slogan、公司介绍、美食探店脚本、爆款分镜剧本策划师。

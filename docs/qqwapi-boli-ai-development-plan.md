# BOLI AI 同类平台开发计划

基于：`docs/qqwapi-boli-ai-product-requirements.md`  
版本：v1.0（2026-09-23）  
目标：将产品需求转换为可进行线框、接口开发、数据库建模、联调、测试和排期的工程计划。

## 0. 结论：原 PRD 不能直接作为开发计划

原 PRD 适合做产品范围基线，但不适合直接进入 Sprint，原因如下：

| 缺口 | 对开发的直接影响 | 本计划的补充 |
|---|---|---|
| 页面多数以功能族描述 | 前端无法确定页面层级、组件边界、空态/异常态和路由行为 | 逐页给出布局、输入输出、状态、权限和线框要点 |
| API 只有路径和少量字段 | 前后端会对分页、鉴权、错误、幂等和异步任务产生不同理解 | 统一响应信封、错误码、P0/P1 API 契约和请求示例 |
| 数据实体仅为概念清单 | 无法建表、迁移、加索引或确定账务一致性 | 给出 PostgreSQL 表、字段类型、PK/FK、索引和关系 |
| 计费/支付/退款规则未形成状态机 | 余额、订单、任务和退款可能重复扣费或账实不符 | 增加账本、订单、报价、任务、退款状态机与幂等规则 |
| CLI/API/MCP/主站身份边界未落实到中间件 | 可能把 JWT、API Key、设备 Token 误用到错误接口 | 设计三类认证中间件、scope 和审计链路 |
| 外部供应商、对象存储、短信、支付依赖未分层 | 测试环境无法替换，故障难以隔离 | 规定 Provider Adapter、Mock、超时、重试和熔断边界 |
| 没有非功能需求和上线门槛 | 无法估算容量、安全、监控、备份和发布风险 | 增加 NFR、可观测性、部署拓扑、测试和验收门槛 |
| P0/P1 仍是业务优先级，不是可交付批次 | 团队无法按依赖拆分任务 | 增加阶段、依赖图、并行工作流和 Definition of Done |

**使用结论**：原 PRD 保留为产品范围与验收背景；本文件作为工程排期、接口评审、数据库评审、线框制作和联调依据。两份文档中出现冲突时，以经产品、后端、前端、财务和安全共同确认的接口契约为准。

## 1. 建设范围与技术基线

### 1.1 首期范围（Release 1）

首期只承诺以下闭环：

1. 手机/邮箱验证码注册登录、密码设置、登录态恢复。
2. PC 工作台：聊天、图片生成、视频生成、TTS/ASR 基础能力。
3. 模型别名、能力参数、价格和会员折扣动态读取。
4. 异步任务、最近作品、作品分类、下载和任务恢复。
5. 算力账户、充值订单、支付回调、兑换码和消费流水。
6. 会员套餐和权益展示/购买。
7. API Key 创建/禁用/删除，以及 Chat/Image/Video 基础 API。
8. 管理后台最小闭环：用户、模型别名/价格、套餐、订单、任务、公告、客服配置。

智能体创作者、课程、灵感广场、工具箱、渠道、CLI/MCP、安卓端列入 Release 2；复杂画布、社区作者、Codex++、CC Switch、BOLI-Claw 列入 Release 3，除非项目负责人明确扩大首期范围。

### 1.2 推荐技术栈

| 层 | 建议实现 | 约束 |
|---|---|---|
| 前端 | Vue 3 + TypeScript + Vite + Pinia + Vue Router | PC/移动端共用 API SDK；所有页面实现 loading/empty/error |
| Go API | Go 1.24+、chi、pgx、sqlc、go-playground/validator | Handler 不直接写 SQL；Service 负责业务；Repository 只负责数据访问 |
| 数据库 | PostgreSQL 16+ | UTC 时间；金额 `numeric`；算力最小单位 `bigint`；迁移版本化 |
| 缓存/限流 | Redis 7+ | OTP、验证码、会话撤销、幂等键、限流、任务短状态 |
| 异步任务 | Redis Streams + worker（或 Asynq） | 生成、轮询、回调、退款、通知均不得阻塞 HTTP 请求 |
| 对象存储 | S3/兼容对象存储 + CDN | 数据库只保存 object key/metadata；下载 URL 短期签名 |
| API 文档 | OpenAPI 3.1 + 自动生成 Go/TS 客户端 | 合并到 CI，接口变更必须通过契约检查 |
| 观测 | OpenTelemetry + Prometheus + Loki/结构化日志 | request_id、user_id 脱敏、job_id、order_no 全链路关联 |
| 部署 | Docker、Nginx/API、worker、cron、PostgreSQL、Redis、对象存储 | dev/staging/prod 隔离；支付和供应商密钥只进 Secret |

### 1.3 统一工程约定

**时间**：数据库保存 UTC `timestamptz`，API 返回 RFC3339；展示由前端按用户时区转换。  
**ID**：业务表使用 UUIDv7；公开任务号、订单号、API Key 前缀不得暴露自增主键。  
**金额**：人民币使用 `numeric(20,6)`，不得使用浮点；算力使用 `bigint`，平台定义 1 算力为最小扣减单位。  
**分页**：列表默认 `page`/`pageSize`，大列表同时支持 `cursor`；最大 pageSize=100。  
**幂等**：所有支付、生成提交、兑换、提现、奖励发放要求 `Idempotency-Key`；后端保存请求摘要，重复键同参返回原结果，异参返回冲突。  
**响应**：

```json
{
  "code": "OK",
  "message": "",
  "data": {},
  "requestId": "req_01J..."
}
```

列表统一：`data.items`、`data.page`、`data.pageSize`、`data.total`、`data.nextCursor`。流式接口不使用上述 JSON 信封，使用 SSE，并在首个事件返回 `requestId`。

## 2. 页面功能细化与线框规格

### 2.1 通用页面状态和组件要求

每个页面必须有以下状态：

- `loading`：首次加载时保留页面骨架；按钮显示处理中且禁止重复提交。
- `ready`：展示数据；不可把空数组误当异常。
- `empty`：说明原因和下一步 CTA，例如“暂无作品→开始创作”。
- `error`：展示可读错误、requestId、重试；不清空用户草稿/上传元数据。
- `forbidden`：区分未登录、无权限、会员不足、内容权益未激活、余额不足。
- `stale`：数据超过刷新时间时显示“上次同步时间”和手动刷新。
- `submitting/processing/succeeded/failed/unknown`：适用于所有生成、支付、兑换和审核动作。

通用组件：`AppShell`、`MarketingNav`、`MobileBottomNav`、`AuthGuard`、`LoginModal`、`BalanceBadge`、`ModelSelector`、`PricePreview`、`UploadDropzone`、`TaskCard`、`WorkCard`、`Pagination`、`Drawer`、`ConfirmDialog`、`Toast`、`ErrorPanel`、`CustomerServiceDialog`。

线框规范：每个页面至少标出顶部导航、主容器宽度、侧栏宽度、内容网格、固定操作区、抽屉/弹窗层级、移动端折叠点；每个提交按钮标出前置校验、报价/确认步骤和成功后去向。

### 2.2 公共与营销页面

#### P-01 首页 `/`

| 项目 | 规格 |
|---|---|
| 布局 | 顶部导航；英雄区（标题、副标题、主 CTA、次 CTA、视觉）；三张价值卡；能力/合作区；公开模型/工具摘要；页脚 |
| 输入 | 搜索/无；CTA 点击；邀请码 URL 参数 |
| 输出 | 跳转 `/agents`、`/business`、`/login` 或打开登录弹窗 |
| 交互 | “开始使用”未登录打开登录；已登录直接进原目标；轮播/公告可关闭并记忆已读 |
| 状态 | site config loading/失败回退默认文案；公告无数据时隐藏；客服配置为空时展示在线反馈入口 |
| 权限 | 公共；邀请码只用于登录注册归因，不在前端发奖励 |

#### P-02 平台介绍 `/platform`

布局为能力英雄、智能对话/算力/兑换码/多场景卡片、三步上手、动态套餐摘要、CTA。套餐摘要只读 `/api/pricing` 或配置快照；点击充值需登录；模型目录失败显示重试，不阻断页面营销内容。

#### P-03 定价 `/pricing`

顶部返回、搜索框、能力 Tab（聊天/图片/视频/音频/智能体/处理）、模型列表。模型卡字段：展示名、品牌、标签、输入/输出或单次/秒/字符价格、会员折扣提示、能力摘要、可用状态。

输入：`q`、`capability`、分页。输出：筛选后的模型快照，不触发生成。加载时显示骨架；无结果显示清空筛选；接口异常支持重试。公开可读，价格必须标注“以提交前报价为准”。

#### P-04 会员 `/membership`

布局：普通会员/合伙人 Tab → 周期切换（月/季/年）→ 套餐卡 → 权益列表 → 购买 CTA → FAQ。套餐卡显示价格、原价、到账算力、有效期、折扣、额度和额外权益。

交互：周期切换只更新服务端返回的套餐；点击购买先登录，随后打开确认弹窗（套餐、应付金额、到账算力、退款规则），再创建订单。支付中禁止再次创建；支付成功跳 `/payment/redirect` 并刷新权益/余额；超时保留订单查询入口。

权限：浏览公开；购买需要主站登录；合伙人权益由套餐和服务端 entitlement 判断。

#### P-05 活动 `/activities`

布局：权益总览、可用额度、会员权益、分享返佣说明、分享统计、创建链接/去钱包按钮。输入为时间筛选和分享渠道名称；输出为权益快照、推广链接、返佣统计。空态应区分“没有活动”和“没有邀请数据”；返佣未启用时不显示伪造比例。

#### P-06 安卓下载 `/app-download`

布局：版本英雄、权限状态卡、下载按钮/二维码、网页/APP 对比、适用人群、安装步骤、FAQ、客服弹窗。进入页面先调用下载资格接口；状态为 `login_required/vip_required/activation_required/available/unavailable`。

可用时显示版本、平台、大小、最低系统、SHA256/签名和下载地址；点击下载只记录事件，不扣算力，除非后台明确将某安装包定义为付费资产。资格失败提供去登录/去激活/开通会员。

#### P-07 商务合作 `/business`、P-08 在线客服 `/customer-service`、P-09 合作伙伴 `/partner-info`

三页共享 `StaticInfoPage`：英雄标题、合作能力/流程、联系方式和二维码。表单输入联系人、公司、业务场景、预计调用量、合作诉求、联系方式；提交后生成线索 ID。若当前版本只允许客服咨询，则按钮打开客服而不伪造提交成功。客服信息为空显示配置缺失和备用邮箱。

#### P-10 支付结果 `/payment/redirect`

布局：订单号、支付状态、金额/套餐、刷新按钮、返回会员/算力按钮。进入页按订单号轮询最多 60 秒；状态 `pending/success/failed/expired/unknown`。unknown 必须提示联系客服，不得引导重复付款。

### 2.3 登录、账户与资产页面

#### P-11 登录/注册 `/login` 与 `LoginModal`

布局：方式 Tab、账号字段、密码/验证码字段、邀请码、协议勾选、提交按钮、错误区。输入输出和规则沿用登录配置；手机号/邮箱发码必须经过 captcha；密码登录滑块/失败锁定由后端配置决定。

状态：配置 loading 时隐藏未启用 Tab；发码倒计时；登录 submitting；成功写入 PC/mobile token 并返回 `redirect`；401/403/429 分别展示账号、权限、频控错误。浏览器刷新不得丢失 redirect 和邀请码。

#### P-12 个人中心 `/user-center`

采用左侧导航 + 右侧内容卡：资料、内容激活码、兑换码、算力、作品、空间、钱包、分享、API Key、反馈、创作者中心、客服。左侧菜单需显示未读/待处理数量，移动端转为下拉或抽屉。

#### P-13 个人资料子页

字段：头像、昵称、性别、出生日期、城市；用户名只读；手机号/邮箱绑定、更换均需验证码；密码设置/修改。保存前校验长度/格式，成功后刷新 `/auth/me`；绑定失败不得覆盖旧联系方式。

#### P-14 算力 `/user-center?tab=power`

余额卡、累计充值/消耗、充值入口、按收入/支出/类型筛选的流水表。每条流水显示业务类型、金额、余额后、订单/任务号、时间、状态、退款关联；loading/empty/error/stale 全覆盖。充值弹窗与 P-04 共用订单组件。

#### P-15 兑换码/内容激活码

兑换码：输入码、服务端预检、展示识别结果、确认兑换、刷新余额/会员。内容激活：电脑端/移动端/双端卡片、权益到期、输入码、激活后返回原页。状态必须来自 entitlement，不可使用本地标记。

#### P-16 作品空间/我的作品

作品空间：容量进度、文件类型统计、扩容卡、最近访问、分页列表。我的作品：图片/视频/音频/文本 Tab、分类筛选、批量删除/下载、预览抽屉。删除二次确认，后端软删除；下载使用短期签名 URL。

#### P-17 钱包/提现

余额、累计收益、冻结、已提现；收益/提现 Tab；提现弹窗字段金额、支付宝/银行卡、实名、账号、开户行。提交前展示手续费、到账时间、月度窗口；提交后状态 `pending/reviewing/approved/paid/rejected/failed`，不可由前端改成功。

#### P-18 分享赚钱

活动卡、邀请链接/邀请码/二维码、点击/注册/有效用户/奖励统计、钱包入口。创建链接需输入名称/渠道；同名不覆盖旧链接。奖励状态单独展示算力和现金；退款/作弊会出现冲正，不直接删除原奖励。

#### P-19 意见反馈

表单：类型、主题、详细内容、联系方式、可选附件；提交返回工单 ID。列表展示状态、官方回复和时间；重复点击由幂等键防止重复工单。

### 2.4 PC 工作台和专项工具页面

#### P-20 工作台壳 `/agents`

布局：左侧主导航（大模型/智能体/小课堂/工具箱/画布/灵感/漫剧/Codex/个人中心）；中间能力工作区；右侧最近作品/任务；顶部用户、余额、公告、客服、主题。

全局状态：切换路由不丢草稿、附件、任务；刷新后由服务端恢复 session/job；余额、权益和模型快照显示更新时间。左侧未登录点击受保护项打开登录，不改变当前公共页。

#### P-21 对话 `/agents/model/chat`

中间为会话消息流，底部 composer（文本、附件、发送、停止），上方模型/参数；右侧为最近作品/引用资产。输入最多 10 个附件，模型能力控制文件类型、字符数和视觉支持。

输出为流式 SSE 或错误消息；发送前显示模型价格、深度思考/联网/压缩等增量费用。停止按钮只终止客户端读取或调用后端 cancel；任务未知时显示查询原任务。

#### P-22 图片 `/agents/model/image`

左侧提示词/参考素材，中部参数卡（模型、模式、比例、分辨率、画质、张数、风格、增强），右侧任务队列/结果画廊。拖入参考图后自动切换模式但保留撤销；分析参考图是独立确认动作。

输入输出见 PRD；状态至少包含预检、报价/估价、提交、排队、生成、成功、失败、退款/未知。结果支持预览、下载、复制提示词、保存作品。

#### P-23 视频 `/agents/model/video`

模式 Tab、提示词、参考图/首尾帧/视频/音频槽位、模型参数、音效开关、价格预估、任务队列。模型切换清理不兼容参数并给出提示；不允许把图生参数提交到纯文生模型。

#### P-24 音频 `/agents/model/audio`

能力 Tab（TTS/ASR/音乐/翻唱/克隆）、模型、文本/文件、音色、语言、格式、语速。TTS 成功返回播放器和下载；ASR 返回可编辑文本并可保存为文本资产；克隆明确参考音频隐私和高价确认。

#### P-25 多模型协作 `/agents/multi-agent`

输入问题、模型多选、模式（并行/辩论/翻译对比/汇总）、输出格式；提交前显示模型数和预估总价。中间显示各模型独立状态，底部显示汇总。取消只取消未提交的分支；部分成功不可标记为全失败。

#### P-26 智能体发现/详情/我的/创建

发现页：搜索、分类、卡片网格、官方/社区/免费/收费标签；详情页：封面、描述、作者、欢迎语、基础模型、计费说明、收藏、开始对话。创建页：名称、图标、封面、简介、分类、模型、提示词、欢迎语、计费策略、技能权限、实时预览；预览不得真实调用付费技能。

#### P-27 工具箱 `/agents/toolbox` 与工具详情

目录页：分类 Tab、搜索、状态、计费方式、输入输出标签；详情页：参数表单、文件上传、价格/报价、确认、进度、结果。工具声明 `sync/async`、文件限制、是否保存结果、退款规则；打开详情不自动执行。

#### P-28 今日素材/知识成图 `/linyi`

四步横向流程：PDF 上传解析 → 知识点列表 → 每条参数卡 → 出图图库；移动端纵向折叠。每步可返回修改；批量生成显示逐项进度和失败项重试，历史按 PDF 分组并显示保留期限。

#### P-29 剧本导演 `/script-director`

需求输入和剧本模型 → 剧本结果编辑 → 角色提示词 → 分镜提示词 → 配音文案 → 视频参数 → 任务结果。每一步单独保存草稿和重跑；视频提交前必须展示价格和参考素材。

#### P-30 电商 AI 一键工厂 `/ecom-factory`

顶部搜索/视觉方向/比例筛选；中间素材网格；右侧或弹窗展示原图、完整/精简提示词、工作流阶段；点击一键生成打开模型/比例/提示词版本/价格确认。素材缺失时仍可复制提示词。

#### P-31 爆款视频分析与其他专项工具

统一采用“来源输入（URL/文件）→上传/预处理→异步分析→关键帧/脚本/资产→导入工作台”的线框。大文件显示分片进度；取消、失败、超时和结果未知独立呈现；导入草稿不覆盖当前会话。

### 2.5 社区、课程和文档页面

#### P-32 AI 探索 `/discover`

公开搜索页，顶部搜索，左侧/顶部分类，主体助手卡网格；卡片显示名称、描述、标签、官方/社区、免费/计费、使用次数。点击开始对话：未登录保存目标 agentId 后登录，登录后回到详情/工作台。

#### P-33 灵感广场 `/inspiration` 与工作台灵感 `/agents/inspiration`

公开页适合浏览；工作台页增加收藏、上传、免费/收费筛选和“用此 Prompt”。无限滚动用 cursor，保留筛选；上传作品先进入草稿/审核，不能自动公开。用 Prompt 生图必须带案例 ID、版本和价格确认。

#### P-34 课程列表/详情 `/inspiration/course/:id`

课程头部、目录树、播放器/图文内容、资料下载、进度、评价和相关推荐。访问资格由后端返回；免费/VIP/已报名/未登录分别渲染 CTA。播放/阅读位置按节流写入 progress；上一节/下一节依目录顺序。

#### P-35 课程创建/编辑 `/inspiration/course/create`、`/edit`

编辑器分基础信息、访问/价格、章节排序、内容编辑、预览、提交审核。章节删除二次确认并提示级联删除；上传先创建资源，再保存内容；提交后锁定编辑直到审核结果或撤回。

#### P-36 文档 `/docs/*`、Vibe Hub `/vibe-hub/*`

左侧章节/术语目录，主体 Markdown/模块化内容，右侧目录或学习路径；支持搜索、复制指令、收藏、上下节。数据加载失败可重试；术语详情不存在回到列表并保留搜索条件。

### 2.6 开发者、下载和渠道页面

#### P-37 API 文档 `/api-docs`

Tab：CLI 下载/模型选择、Codex 接入、完整契约、API Key、模型发现。API Key Tab 需登录；创建弹窗字段名称/scopes/rateLimit/expiresAt，成功后一次性展示完整 Key，要求用户勾选“已保存”。文档示例始终使用占位符，不执行复制后的付费命令。

#### P-38 Codex `/connect-codex`、CLI 授权 `/cli/authorize`

Codex 页按安装 CLI→设备授权→选模型→配置 MCP→重启验收分步展示。授权页显示当前登录用户、设备、只读/付费 scope、风险警告、批准/拒绝；批准不执行生成。拒绝/过期/轮询太快均有独立状态。

#### P-39 Codex/CC Switch/Codex++/BOLI-Claw 下载页

下载落地页展示系统版本、文件大小、版本、校验值、安装/卸载、会员/算力门槛、客服和 GitHub/官方链接。付费下载先预览价格并确认；下载成功不等于安装成功，提供校验失败与回滚说明。

#### P-40 渠道登录/看板 `/channel/login`、`/channel/dashboard`

独立渠道会话；看板有时间筛选、KPI、趋势、结算表、代理贡献、分成规则。每个数字显示统计口径、更新时间和接口缺失态；渠道不能访问主站用户的个人资料或主站 JWT。

#### P-41 接口自检 `/mock-test`

只读检查公开灵感、Banner、定价、会员、帮助和 OpenClaw 信息；每项显示 HTTP、耗时、响应摘要和失败原因。页面必须明确“不触发生成、不扣费”。

### 2.7 页面级交付矩阵

下表是线框、前端 story 和 QA 用例的最小交付清单；各页还必须叠加 2.1 的通用状态。`输出`指成功后页面可消费的服务端数据或下一步路由，不能由前端本地推导最终权益和账单。

| 编号/页面 | 主要输入 → 输出 | 特殊状态 | 权限 |
|---|---|---|---|
| P-01 首页 | CTA、邀请码参数 → 目标路由/登录弹窗 | 配置失败回退、公告关闭 | 公开 |
| P-02 平台介绍 | 能力卡/CTA → 定价、工作台或登录 | 模型目录失败不阻断营销内容 | 公开；购买需登录 |
| P-03 定价 | 搜索、能力、分页 → 模型价格快照 | 无结果、价格版本刷新 | 公开只读 |
| P-04 会员 | 周期、套餐 → 订单、支付结果 | 支付中、超时、unknown | 浏览公开；购买需登录 |
| P-05 活动 | 时间、渠道 → 权益/分享统计 | 活动未启用、无邀请数据 | 登录 |
| P-06 安卓下载 | 平台/版本 → 资格、签名下载 | login/vip/activation/unavailable | 资格由服务端判定 |
| P-07 商务合作 | 联系人、公司、诉求 → 线索 ID/客服 | 提交关闭或配置缺失 | 公开；提交需频控 |
| P-08 在线客服 | 问题、附件 → 工单/客服会话 | 客服离线、附件扫描中 | 公开或登录可选 |
| P-09 合作伙伴 | 合作表单 → 线索/客服 | 渠道关闭、提交失败 | 公开；后台配置 |
| P-10 支付结果 | 订单号 → 订单/支付状态 | pending/success/failed/expired/unknown | 登录且校验订单归属 |
| P-11 登录/注册 | 账号、验证码、密码、邀请码 → token、用户、redirect | 发码倒计时、锁定、频控 | 公开 |
| P-12 个人中心 | Tab、菜单 → 子页数据和未读数 | 权限菜单动态隐藏、登录失效 | 登录 |
| P-13 资料 | 头像、昵称、绑定联系方式 → 用户资料 | 验证码失效、旧联系方式保留 | 登录；改密需二次验证 |
| P-14 算力 | 筛选、分页、充值 → 余额/账本页 | 余额 stale、流水空态、充值中 | 登录 |
| P-15 兑换/激活 | 兑换码、设备端选择 → ledger/entitlement | 无效、已用、过期、部分权益 | 登录 |
| P-16 作品/空间 | 类型、分类、批量操作 → 作品/下载 URL | 扫描中、软删除、配额满 | 登录；归属校验 |
| P-17 钱包/提现 | 金额、实名、收款账号 → 提现单 | 冻结余额、reviewing/rejected/failed | 登录；满足提现资格 |
| P-18 分享赚钱 | 活动、渠道名 → 链接/归因/奖励 | 活动过期、反作弊冲正 | 登录 |
| P-19 意见反馈 | 类型、主题、内容、附件 → 工单 | 提交中、重复提交、客服回复 | 登录或匿名按配置 |
| P-20 工作台壳 | 路由、模型、任务 → 子页面、最近作品 | token 恢复、余额刷新、草稿恢复 | 登录；公共入口可见 |
| P-21 对话 | 消息、附件、参数 → SSE 消息、usage、账务 | streaming、停止、断线、unknown | 登录；模型 scope/余额 |
| P-22 图片 | prompt、参考图、参数 → quote、job、作品 | 预检/报价/排队/退款 | 登录；模型能力/余额 |
| P-23 视频 | prompt、首尾帧/素材、参数 → quote、异步 job | 上传、转码、阶段进度、超时 | 登录；会员/额度按策略 |
| P-24 音频 | 文本/音频、音色、格式 → 音频或文本资产 | 播放器加载、识别中、隐私确认 | 登录；克隆需额外授权 |
| P-25 多模型协作 | 问题、模型集合、模式 → 分支结果/汇总 | 部分成功、取消、总价变化 | 登录；按模型 scope |
| P-26 智能体发现/详情/创建 | 搜索、配置、提示词 → agent 版本/运行 | 草稿、审核、拒绝、发布 | 浏览公开；创建/发布需登录 |
| P-27 工具箱 | 分类、参数、文件 → quote/job/result | sync/async、文件扫描、退款 | 登录；工具 scope |
| P-28 今日素材 | PDF、知识点、批量参数 → 图片作品 | 解析失败、单项失败、历史过期 | 登录；文件配额 |
| P-29 剧本导演 | 需求、步骤草稿、视频参数 → 剧本/提示词/job | 分步保存、重跑、价格确认 | 登录 |
| P-30 电商工厂 | 搜索、视觉方向、素材、版本 → 提示词/生成任务 | 素材缺失、版本切换、报价 | 登录；生成需余额 |
| P-31 爆款分析 | URL/文件 → 分析报告/关键帧/草稿 | 大文件、预处理、unknown | 登录；来源校验 |
| P-32 AI 探索 | 搜索、分类、agentId → 详情/工作台 | 公开列表失败、登录后回跳 | 公开浏览；使用需登录 |
| P-33 灵感广场 | cursor、标签、收藏、用 Prompt → 内容/quote | 审核中、下架、无结果 | 公开浏览；上传/使用需登录 |
| P-34 课程详情 | 课程、章节、播放位置 → 内容/进度 | 未报名/VIP/完成/资料失效 | 公开详情；内容按 entitlement |
| P-35 课程编辑 | 基础信息、章节、内容 → 草稿/审核提交 | 锁定编辑、撤回、审核拒绝 | 创作者本人/审核员 |
| P-36 文档/Vibe Hub | 搜索、章节、复制 → Markdown/学习进度 | 文档缺失、搜索无结果 | 按文档公开/会员策略 |
| P-37 API 文档 | Tab、Key 字段、scope → Key secret/示例 | secret 一次性展示、scope 错误 | 文档公开；Key 需登录 |
| P-38 Codex/授权 | 设备 code、scope → approve/deny/token | 待确认、过期、拒绝、限频 | 登录用户；设备归属 |
| P-39 下载页 | 系统版本、安装包 → 资格/下载/校验 | 版本不兼容、校验失败、回滚 | 公开；付费资产需资格 |
| P-40 渠道看板 | 登录、日期筛选 → KPI/趋势/结算 | 数据延迟、无权限、结算中 | 独立渠道会话 |
| P-41 接口自检 | 检查项 → HTTP/耗时/摘要 | 单项失败、超时、维护 | 公开只读；严禁生成/扣费 |

页面线框交付时，每行至少产出：桌面和移动端两张线框、组件/字段标注、状态切换图、事件埋点清单、接口字段映射和一条主流程 Playwright 用例。

## 3. 前后端拆分与 API 契约

### 3.1 服务边界与依赖

```text
API Gateway / Auth Middleware
        │
        ├── User & Session ──┐
        ├── Catalog/Model ───┼── Billing/Ledger ── Payment
        ├── Chat/Media ──────┤          │
        ├── Works/Storage ───┘          ├── Membership/Entitlement
        ├── Agent/Course ───────────────┤
        ├── Developer(API Key/CLI/MCP) ─┤
        └── Admin/Channel/Promotion ────┘
                         │
                 Provider Adapters / Workers / Outbox
```

前端负责路由、展示、表单校验、报价确认 UI、SSE/轮询、草稿恢复和埋点；不得自行计算最终扣费、会员资格、退款和权限。Go 后端负责鉴权、业务规则、计费、状态机、Provider 调用、对象存储签名、审计和最终数据。

### 3.2 错误码标准

| HTTP | code | 说明 | 前端行为 |
|---:|---|---|---|
| 400 | `REQ_INVALID` | 字段/组合参数非法 | 定位字段，不清空表单 |
| 401 | `AUTH_REQUIRED` | 未登录/Token 过期 | 保存 redirect，打开登录 |
| 401 | `AUTH_INVALID` | Key/设备 Token 无效 | 提示重新授权/轮换 |
| 403 | `PERMISSION_DENIED` | scope/角色不足 | 展示去开通或联系客服 |
| 403 | `ENTITLEMENT_REQUIRED` | 会员/内容权益不足 | 跳会员或激活页 |
| 402 | `POWER_INSUFFICIENT` | 算力不足 | 打开充值，不丢草稿 |
| 404 | `NOT_FOUND` | 资源不存在/不属于当前用户 | 刷新列表或返回上级 |
| 409 | `IDEMPOTENCY_CONFLICT` | 同 Key 不同请求 | 禁止重试，显示 requestId |
| 409 | `STATE_CONFLICT` | 状态不允许操作 | 刷新原资源状态 |
| 410 | `QUOTE_EXPIRED` | 报价过期 | 重新报价，不自动提交 |
| 413 | `FILE_TOO_LARGE` | 文件超限 | 显示限制并保留其他输入 |
| 415 | `FILE_UNSUPPORTED` | MIME/扩展名不支持 | 重新选择文件 |
| 422 | `CONTENT_BLOCKED` | 安全/版权/真人内容拦截 | 展示可替代方案 |
| 429 | `RATE_LIMITED` | 用户/Key/IP 频控 | 使用 retryAfter 倒计时 |
| 500 | `INTERNAL_ERROR` | 服务内部错误 | 重试；付费任务按未知处理 |
| 502 | `PROVIDER_ERROR` | 上游供应商错误 | 查询原任务，不换模重建 |
| 503 | `SERVICE_UNAVAILABLE` | 能力或节点关闭 | 显示维护态 |
| 504 | `PROVIDER_TIMEOUT` | 上游超时 | 标记 unknown，允许查询 |
| 507 | `STORAGE_QUOTA_EXCEEDED` | 空间不足 | 引导扩容 |
| 500 | `RECONCILIATION_REQUIRED` | 账务/任务未知 | 禁止再次扣费，转核查 |

### 3.3 P0 主站 API

以下表格为 Go Handler 必须实现的最小契约。除特别说明，均返回统一 JSON 信封，并从响应头返回 `X-Request-ID`。

#### 3.3.1 认证与用户

| Method | Path | 请求 | 成功 data | 鉴权/错误 |
|---|---|---|---|---|
| GET | `/api/auth/login-config` | 无 | `passwordEnabled,phoneEnabled,emailEnabled,inviteCodeRequired,registerBonusPower,lockPolicy` | public；`SERVICE_UNAVAILABLE` |
| POST | `/api/auth/captcha/generate` | `{scene,subject?}` | `{captchaId,question,expiresAt}` | public；`REQ_INVALID`、`RATE_LIMITED` |
| POST | `/api/auth/captcha/verify` | `{captchaId,scene,subject,answer}` | `{captchaToken,expiresAt}` | public；`REQ_INVALID`、`AUTH_INVALID` |
| GET | `/api/auth/invite/validate?code=` | query code | `{valid,code,inviteeReward,referrerReward}` | public；无效返回 `valid:false` |
| POST | `/api/auth/sms/send` | `{phone,type,captchaToken,inviteCode?}` | `{maskedPhone,cooldownSeconds,loginCaptchaToken?}` | public；`REQ_INVALID`、`RATE_LIMITED` |
| POST | `/api/auth/email/send` | `{email,type,captchaToken}` | `{maskedEmail,cooldownSeconds}` | public；同上 |
| POST | `/api/auth/sms/login` | `{phone,code,captchaToken,inviteCode?,agreeTerms}` | `{token,refreshToken,user}` | public；`AUTH_INVALID`、`AUTH_LOCKED` |
| POST | `/api/auth/email/login` | `{email,code,captchaToken,inviteCode?,agreeTerms}` | `{token,refreshToken,user}` | public；同上 |
| POST | `/api/auth/password/login` | `{account,password,sliderToken?,agreeTerms}` | `{token,refreshToken,user}` | public；`AUTH_INVALID`、`AUTH_LOCKED` |
| POST | `/api/auth/refresh` | `{refreshToken}` | `{token,refreshToken,expiresIn}` | refresh token；`AUTH_INVALID` |
| POST | `/api/auth/logout` | `{allDevices?}` | `{revoked:true}` | JWT；`AUTH_REQUIRED` |
| GET | `/api/auth/me` | 无 | `{user,balances,entitlements,serverTime}` | JWT；`AUTH_REQUIRED` |
| PATCH | `/api/user/profile` | `{nickname,gender,birthday,city,avatarObjectKey?}` | `{user}` | JWT；`REQ_INVALID` |
| POST | `/api/user/bind-phone/send` | `{phone,captchaToken}` | `{cooldownSeconds}` | JWT；验证码错误 |
| POST | `/api/user/bind-phone/confirm` | `{phone,code}` | `{user}` | JWT；`STATE_CONFLICT` |
| POST | `/api/user/bind-email/send` | `{email,captchaToken}` | `{cooldownSeconds}` | JWT |
| POST | `/api/user/bind-email/confirm` | `{email,code}` | `{user}` | JWT |
| POST | `/api/user/password` | `{verifyType,verifyCode?,oldPassword?,newPassword}` | `{updatedAt}` | JWT；`REQ_INVALID` |

#### 3.3.2 模型目录与定价

| Method | Path | 请求 | 成功 data | 错误 |
|---|---|---|---|---|
| GET | `/api/model-aliases/public` | `capability,q,cursor,pageSize` | `items[{id,aliasName,capability,parameters,entitlement,priceSummary}],nextCursor` | public |
| GET | `/api/models/public` | `type?` | 兼容展示目录；明确 `channelId` 仅展示不可直接调用 | public |
| GET | `/api/pricing` | `capability?,q,page,pageSize` | `categories/items`，含 priceVersion | public |
| GET | `/api/user/model-preferences` | `capability?` | 用户默认模型/备用模型 | JWT |
| PUT | `/api/user/model-preferences/{capability}` | `{modelAliasIds[]}` | `{saved:true}` | JWT；至少 1 个可见 |

价格响应必须包含：`currency`、`unit`、`inputCost`、`outputCost`/`unitCost`、`memberDiscount`、`effectiveFrom`、`priceVersion`、`quoteRequired`，避免前端把展示价格当最终账单。

#### 3.3.3 会话、聊天和异步任务

| Method | Path | 请求 | 成功 data | 错误 |
|---|---|---|---|---|
| GET | `/api/chat/sessions` | `page,pageSize,status` | `{items:[session],total}` | JWT |
| POST | `/api/chat/sessions` | `{title?,modelAliasId?}` | `session` | JWT |
| PATCH | `/api/chat/sessions/{id}` | `{title}` | `session` | JWT/`NOT_FOUND` |
| DELETE | `/api/chat/sessions/{id}` | 无 | `{deleted:true}` | JWT |
| GET | `/api/chat/sessions/{id}/messages` | `cursor,limit` | `{items:[message],nextCursor}` | JWT、归属校验 |
| POST | `/api/chat/sessions/{id}/messages` | `{modelAliasId,messages,attachments[],options,stream,idempotencyKey}` | 非流式 `{message,usage,billing}`；流式 SSE | JWT；`POWER_INSUFFICIENT`、`CONTENT_BLOCKED` |
| POST | `/api/generation-jobs/{id}/cancel` | `{reason?}` | `{jobId,status}` | JWT；`STATE_CONFLICT` |
| GET | `/api/generation-jobs/{id}` | 无 | job + events + result | JWT；只读原任务 |
| GET | `/api/generation-jobs` | `kind,status,cursor` | `{items,nextCursor}` | JWT |

流式事件：`start`、`delta`、`usage`、`billing`、`done`、`error`；断线重连使用 `Last-Event-ID`，服务端只允许继续读取同一 message/job，不重复发起计费请求。

#### 3.3.4 图片、视频、音频

| Method | Path | 请求 | 成功 data | 错误 |
|---|---|---|---|---|
| POST | `/api/image/quote` | `{modelAliasId,prompt,n,ratio,resolution,quality,referenceObjectKeys[],options,idempotencyKey}` | `{quoteId,expiresAt,price,powerBalanceAfterPreview,parameterSnapshot,status:"prepared"}` | `REQ_INVALID`、`CONTENT_BLOCKED` |
| POST | `/api/quotes/{quoteId}/confirm` | `{idempotencyKey}` | `{quoteId,confirmToken,expiresAt,status:"confirmed"}` | JWT；`QUOTE_EXPIRED`、`STATE_CONFLICT` |
| POST | `/api/image/generate` | `{quoteId,confirmToken,idempotencyKey}` | `{jobId,status:"queued",quoteId}` | `QUOTE_EXPIRED`、`POWER_INSUFFICIENT` |
| GET | `/api/image/status?workId=` | query | `{jobId,status,resultUrls,cost,refundStatus}` | 只查原任务 |
| POST | `/api/video/quote` | `{modelAliasId,prompt,duration,ratio,resolution,generateAudio,assets[],idempotencyKey}` | `{quoteId,expiresAt,price,powerBalanceAfterPreview,parameterSnapshot,status:"prepared"}` | 同上 |
| POST | `/api/video/generate` | `{quoteId,confirmToken,idempotencyKey}` | job | 同上 |
| GET | `/api/video/status?taskId=` | query | job/result | 只查原任务 |
| POST | `/api/tts/generate` | `{modelAliasId,input,voice,format,speed,idempotencyKey}` | `{jobId,status}` 或音频 result | `POWER_INSUFFICIENT` |
| POST | `/api/asr/recognize` | `{modelAliasId,objectKey?,url?,audioBase64?,language,idempotencyKey}` | `{jobId,text,status}` | `FILE_UNSUPPORTED` |
| POST | `/api/audio/music/generate` | `{modelAliasId,prompt,lyrics?,duration?,idempotencyKey}` | job | 供应商/安全错误 |

报价/确认模型：报价不扣费；确认只锁定 quote；生成提交才扣账本。API Key 网关可单独定义“直接计费”策略，但必须在响应和文档中显式声明。

#### 3.3.5 作品、上传、存储

| Method | Path | 请求 | 成功 data | 错误 |
|---|---|---|---|---|
| POST | `/api/upload/presign` | `{fileName,mime,size,kind}` | `{uploadId,objectKey,uploadUrl,expiresAt}` | `FILE_TOO_LARGE`、`STORAGE_QUOTA_EXCEEDED` |
| POST | `/api/upload/complete` | `{uploadId,checksum,duration?,width?,height?}` | `fileObject` | 校验/病毒扫描失败 |
| GET | `/api/works/list` | `type,category,page,pageSize,includeDeleted=false` | `{items,total}` | JWT |
| GET | `/api/works/recent` | `type?,limit` | `{items}` | JWT |
| GET | `/api/works/{id}` | 无 | work + assets | 归属校验 |
| PATCH | `/api/works/{id}` | `{title,category,visibility}` | work | `STATE_CONFLICT` |
| DELETE | `/api/works/{id}` | `{hardDelete?}` | `{deleted:true}` | JWT；默认软删 |
| POST | `/api/works/text-assets` | `{title,content,category,workId?}` | text work | `REQ_INVALID` |
| GET | `/api/storage/summary` | 无 | `{usedBytes,quotaBytes,fileCount,byType}` | JWT |
| GET | `/api/storage/download/{id}` | 无 | `{downloadUrl,expiresAt}` | 归属/权限 |

#### 3.3.6 算力、会员、订单和兑换

| Method | Path | 请求 | 成功 data | 错误 |
|---|---|---|---|---|
| GET | `/api/wallet/balance` | 无 | `{powerBalance,currency,asOf}` | JWT |
| GET | `/api/consumption/records` | `type,page,pageSize,from,to` | ledger page | JWT |
| GET | `/api/membership-benefits/plans` | `period?` | plans + benefits + priceVersion | public |
| POST | `/api/orders` | `{kind,planId/packageId,quantity,paymentChannel,idempotencyKey}` | order snapshot | JWT；`IDEMPOTENCY_CONFLICT` |
| GET | `/api/orders/{orderNo}` | 无 | order/payment status | JWT |
| POST | `/api/payment/alipay/create` | `{orderNo,returnUrl}` | `{paymentUrl,expiresAt}` | JWT |
| POST | `/api/payment/wechat/create` | `{orderNo,returnUrl}` | `{codeUrl,expiresAt}` | JWT |
| POST | `/api/payment/huifu/create` | `{orderNo,returnUrl}` | channel payload | JWT |
| POST | `/api/payment/webhook/{channel}` | provider signed body | `{received:true}` | provider signature |
| GET | `/api/payment/query` | `orderNo` | payment status | JWT/internal |
| POST | `/api/power/redeem` | `{code,idempotencyKey}` | `{kind,grantedPower,entitlement,ledgerId}` | `CODE_INVALID`、`CODE_USED` |
| GET | `/api/activation-access/status` | 无 | device scopes/entitlement | JWT |
| POST | `/api/activation-access/activate` | `{code,idempotencyKey}` | entitlement | JWT |

账本写入必须在同一数据库事务中完成：锁定 `power_accounts` → 写 `power_ledger` → 写业务记录 → 写 outbox；禁止由前端余额或异步回调直接加余额。

#### 3.3.7 API Key

| Method | Path | 请求 | 成功 data | 错误 |
|---|---|---|---|---|
| GET | `/api/user/api-keys` | 无 | masked keys | JWT |
| POST | `/api/user/api-keys` | `{name,scopes[],rateLimit,expiresAt}` | `{id,prefix,secret,scopes,expiresAt}` | `REQ_INVALID`；secret 只返回一次 |
| PATCH | `/api/user/api-keys/{id}` | `{status:"enabled"|"disabled",rateLimit?}` | masked key | JWT/归属 |
| DELETE | `/api/user/api-keys/{id}` | 无 | `{revoked:true}` | JWT |
| POST | `/api/user/api-keys/{id}/reveal` | `{confirm:true}` | secret（可选；建议不提供） | 高风险审计 |
| GET | `/api/v1/models` | `type?` | OpenAI-compatible model list | API Key；scope `models:read` |
| POST | `/api/v1/chat/completions` | `{model,messages,stream,options}` | Chat/SSE | API Key；scope `chat` |
| POST | `/api/v1/images/generations` | `{model,prompt,n,size,quality}` | image job/result | scope `image` |
| POST | `/api/v1/videos/generations` | `{model,prompt,duration,size}` | `{id,status:"processing"}` | scope `video` |
| GET | `/api/v1/videos/generations` | `taskId` | original job | scope `video` |

### 3.4 P1 API

| 模块 | API 组 | 最小接口 |
|---|---|---|
| 智能体 | `/api/agents` | `GET /discover`、`GET /categories`、`GET /my`、`POST /create`、`PATCH /{id}`、`POST /{id}/publish`、`POST /{id}/favorite`、`POST /{id}/chat` |
| 课程 | `/api/courses` | 列表、详情、报名、进度、创作者申请、创建/编辑、章节 CRUD、内容 CRUD、提交审核、收益 |
| 灵感 | `/api/inspirations` | 分类、游标列表、详情、收藏、上传、审核、使用 Prompt |
| 分享 | `/api/share` | 活动、链接 CRUD、归因、统计 |
| 钱包 | `/api/user/wallet` | 收益、流水、提现申请、提现状态 |
| CLI | `/api/cli/auth` | device start/poll/refresh/revoke、主站 approve/deny、设备列表 |
| MCP | `/mcp` | initialize、ping、tools/list、tools/call；每个工具检查 scope |
| 渠道 | `/api/channel` | 登录、看板汇总、趋势、结算、代理贡献、分成规则 |
| 反馈 | `/api/feedback` | 创建、分页、详情、管理员回复/关闭 |
| 公告/客服 | `/api/announcements`、`/api/auth/main-site-support` | 列表、已读、客服配置 |

P1 接口同样必须沿用统一信封、分页、requestId、归属校验和错误码，不能因“内容类接口”而省略审核状态、软删除和审计字段。

## 4. Go 后端业务模块设计

### 4.1 服务边界与依赖

首期建议采用模块化单体（modular monolith），按领域拆包；只有生成 Worker、支付回调、通知和高吞吐 API 网关独立进程。这样既能共享事务，又避免过早拆成多个服务。推荐依赖方向如下：

```text
HTTP/Gateway
  -> Auth middleware / Rate limit / Request context
  -> Handler
  -> Application Service
  -> Domain policy + Repository
  -> PostgreSQL / Redis / Object Storage
                         \-> Outbox -> Worker -> Provider adapters
```

禁止 Handler 直接操作数据库、扣算力或调用供应商。Provider Adapter 只负责协议转换和原始响应归一化；价格、权限、扣费、退款和最终状态由平台 Service 决定。

### 4.2 模块清单

| 模块 | 责任 | 主要依赖 | Service/Repository | 事务与异步边界 | 可并行条件 |
|---|---|---|---|---|---|
| API Gateway/Middleware | 路由、CORS、鉴权、限流、requestId、统一错误 | Redis、Auth | `AuthMiddleware`、`RateLimitMiddleware` | 请求级；不包含业务事务 | Phase 0 后可先行 |
| Auth & Session | 登录注册、JWT、刷新/撤销、设备会话、OTP/Captcha | users、Redis、短信/邮件 Provider | `AuthService`、`SessionRepo`、`OTPRepo` | OTP/登录尝试原子计数；发码异步 | 依赖用户表 |
| User/Profile | 用户资料、偏好、绑定联系方式 | Auth | `UserService`、`UserRepo` | 更新资料单事务 | 依赖 Auth |
| Catalog/Model/Price | 供应商、模型、别名、能力、版本化价格 | Admin、Redis | `CatalogService`、`PriceRepo` | 发布价格快照单事务；缓存异步失效 | 可与 Auth 并行 |
| Billing/Ledger | 算力账户、消费、充值、奖励、退款冲正 | User、Orders、Jobs | `LedgerService`、`PowerRepo` | 锁 `power_accounts`，账本追加写；outbox 同事务 | 依赖用户和模型价格 |
| Payment | 订单、渠道支付、签名回调、重放 | Billing、Membership | `OrderService`、`PaymentService` | webhook 幂等事务；渠道调用异步/可重试 | 依赖 Billing |
| Membership/Entitlement | 套餐、会员周期、额度、折扣和资格 | Billing、Catalog | `EntitlementService` | 购买成功入账和权益激活同事务 | 依赖订单 |
| Storage/File | 预签名上传、元数据、病毒扫描、配额 | S3、Redis | `UploadService`、`FileRepo` | complete 事务；扫描异步，未通过不可引用 | 依赖 User |
| Works/Assets | 作品、资产、分类、软删除、下载权限 | Storage、Jobs | `WorkService`、`WorkRepo` | 作品与资产关联单事务 | 依赖 Storage |
| Chat | 会话、消息、附件、SSE | User、Catalog、Task | `ChatService`、`MessageRepo` | 消息写入事务；模型流式异步 | 依赖 Auth/Catalog |
| Media Generation | 图片/视频/TTS/ASR/Music 参数校验、报价、提交 | Catalog、Billing、Storage、Task | `QuoteService`、`GenerationService` | quote/confirm/generate 分步事务 | 依赖 Billing/Provider sandbox |
| Task/Worker | 任务队列、轮询、回调、超时、退款 | Redis Streams、Jobs、Billing | `TaskService`、`WorkerHandler` | 状态转移 CAS；最终状态异步 | 依赖 Generation |
| Agent | 智能体、版本、运行、审核、收藏 | Chat、Catalog、Review | `AgentService`、`AgentRepo` | 发布版本单事务；运行异步 | Release 2 |
| Course | 课程、章节、内容、报名和进度 | User、Review、Storage | `CourseService`、`CourseRepo` | 审核/发布事务；视频处理异步 | Release 2 |
| Inspiration | 灵感内容、收藏、使用、审核 | Works、Review、Storage | `InspirationService` | 上传后审核异步 | Release 2 |
| Promotion/Referral | 活动、链接、归因、奖励 | User、Billing、Wallet | `ReferralService`、`RewardRepo` | 奖励由事件消费；防重复发放 | 依赖 Billing |
| Wallet/Withdraw | 收益钱包、冻结、提现审批和打款 | Promotion、User、Audit | `WalletService`、`WithdrawalRepo` | 提现状态机事务；打款异步 | Release 2 |
| API Key | Key 哈希、scope、限额、撤销和调用计量 | Auth、Billing、Catalog | `APIKeyService`、`UsageRepo` | Key 创建/撤销事务；调用扣费异步或同步锁账 | 依赖 Billing |
| CLI Device Auth | device code、授权、刷新、撤销 | Auth、API Key | `DeviceAuthService` | approve/deny 单事务；poll 限频 | Release 2 |
| MCP | 工具发现、报价/确认/执行、审计 | API Key、Quote、Task | `MCPService` | tool call 沿用任务事务；审计必写 | 依赖 Generation |
| Admin | 用户、价格、模型、订单、任务、内容、配置 | 所有领域、Audit | `AdminService` | 后台变更必须审计；批量任务异步 | P0 最小版 |
| Channel | 渠道登录、看板、代理分成、结算 | Auth、Promotion、Wallet | `ChannelService` | 结算批次事务 | Release 2 |
| Announcement/Customer Service | 公告、工单、客服配置 | User、Admin | `AnnouncementService`、`TicketService` | 工单状态单事务；通知异步 | 可与营销页并行 |
| Audit/Observability | 安全审计、业务审计、指标、追踪 | 全部模块、OTel | `AuditService`、`OutboxRepo` | 高风险操作与主事务同写；日志脱敏 | Phase 0 建骨架 |

### 4.3 Go 代码分层和包约定

```text
cmd/api            # HTTP 进程
cmd/worker         # 异步消费者
internal/http      # handler、route、middleware、DTO
internal/app       # application service、事务编排
internal/domain    # 状态机、策略、值对象、错误
internal/repo      # sqlc 生成代码和 repository 适配
internal/provider  # payment/model/storage/sms adapters
internal/jobs      # job handler、retry、dead letter
internal/observability
migrations/
```

Service 方法须接收 `context.Context`，业务写操作必须显式声明事务；Repository 不返回 HTTP 错误码，而返回领域错误（如 `ErrInsufficientPower`），由 transport 层映射为统一错误码。Provider 调用需设置 connect/read/total timeout、重试次数、熔断状态和供应商 request id。

### 4.4 统一错误码

| HTTP | code | 说明 | 前端动作 |
|---:|---|---|---|
| 400 | `REQ_INVALID` | 字段、格式或组合参数无效 | 定位字段并保留草稿 |
| 401 | `AUTH_REQUIRED` / `TOKEN_EXPIRED` | 未登录或 token 过期 | 刷新一次，失败后登录 |
| 403 | `FORBIDDEN` / `SCOPE_DENIED` / `MEMBERSHIP_REQUIRED` | 权限、scope 或会员不足 | 去授权/会员页 |
| 404 | `NOT_FOUND` | 资源不存在或不属于当前用户 | 返回列表或 404 页 |
| 409 | `STATE_CONFLICT` / `IDEMPOTENCY_CONFLICT` | 状态不允许或幂等键复用不同请求 | 查询原资源，不重复提交 |
| 410 | `QUOTE_EXPIRED` / `TOKEN_EXPIRED` | 报价或设备授权过期 | 重新报价/授权 |
| 413 | `FILE_TOO_LARGE` | 文件超过策略 | 提示限制 |
| 422 | `CONTENT_BLOCKED` / `FILE_UNSUPPORTED` | 内容安全或文件类型失败 | 修改输入 |
| 423 | `ACCOUNT_LOCKED` | 登录失败或风控锁定 | 展示解锁时间 |
| 429 | `RATE_LIMITED` | 频率/并发超限 | 按 Retry-After 重试 |
| 402 | `POWER_INSUFFICIENT` | 算力不足 | 去充值/会员 |
| 502 | `PROVIDER_ERROR` | 上游可重试失败 | 任务标记 failed 或重试 |
| 504 | `PROVIDER_TIMEOUT` | 上游超时，结果可能未知 | 查询原任务，禁止重建 |
| 500 | `INTERNAL_ERROR` | 未分类服务错误 | 展示 requestId 并联系客服 |

错误响应必须包含 `code/message/requestId`，字段校验可包含 `details:[{field,reason}]`；不得把供应商密钥、完整 prompt 中的敏感信息或 API Key secret 写入日志。

## 5. 状态机与核心时序

### 5.1 生成任务状态机

`queued -> submitting -> processing -> succeeded`；异常路径为 `submitting/processing -> failed|unknown|cancelled|expired`，成功后若发生扣费冲正则追加 `refunded`。只有服务端 Worker/回调可以推进状态，客户端只能发起取消并查询。

| 状态 | 可进入条件 | 可转移 | UI 含义 |
|---|---|---|---|
| queued | 生成提交事务完成 | submitting、cancelled | 排队中 |
| submitting | Worker 领取任务 | processing、failed、unknown | 已提交上游 |
| processing | 有 provider task id 或持续轮询 | succeeded、failed、unknown、expired | 处理中 |
| succeeded | 结果已落对象存储并校验 | refunded（仅账务冲正） | 可预览/下载 |
| failed | 确认无结果 | refunded（若已预扣） | 显示失败原因和退款状态 |
| cancelled | 用户或系统取消且上游确认 | refunded | 已取消 |
| expired | 超出业务 SLA/有效期 | refunded、unknown | 已过期 |
| unknown | 网络中断或上游无法确认 | succeeded、failed、refunded | 只能查原任务 |
| refunded | 账务已完成冲正 | 终态 | 退款完成 |

`unknown` 不允许自动换模型、重复创建任务或再次扣费；运营只能通过原 provider task id 重试查询或人工对账。

### 5.2 订单、报价、提现、内容审核状态机

- 订单：`created -> pending_payment -> paid`；异常为 `failed|expired`；支付后可 `refunded|partially_refunded`。支付回调必须以渠道签名和金额校验为准。
- 报价：`prepared -> confirmed -> consumed`；超时为 `expired`，用户取消为 `cancelled`。`confirmed` 只锁定报价，不扣账。
- 提现：`pending -> reviewing -> approved -> paid`；风控或资料不符转 `rejected`，打款失败转 `failed`；只能由后台或支付 Worker 推进。
- 智能体/课程审核：`draft -> submitted -> reviewing -> approved -> published`，可转 `rejected` 或 `unpublished`。已发布版本不可直接覆盖，必须新建版本并保留审计记录。

### 5.3 核心时序

**普通用户聊天**：登录 → 拉取模型目录与余额 → 新建/选择会话 → 前端发送消息和附件元数据 → 后端校验 scope、内容安全、余额/权益 → 写入 user message → 创建 assistant job → SSE 返回 `message.delta` → 完成时写 assistant message、usage、ledger 和 outbox → 前端更新会话标题和最近作品。断线后用 `GET /api/chat/sessions/{id}/messages?after=` 补齐，不重复生成。

**图片生成**：填写参数 → `POST /api/image/quote` → 展示价格、有效期和参数快照 → 用户确认 → `POST /api/image/generate`（幂等）→ 锁定/扣减算力并创建 job → Worker 调用供应商 → 写结果和作品 → SSE/轮询更新 → 失败或 unknown 按规则退款/对账。

**视频生成**：上传素材并预检 → 报价/确认 → 队列任务 → Worker 提交上游并持久化 provider task id → 定时器轮询或 webhook → 下载/转码到对象存储 → 完成作品 → 前端在任务中心显示百分比（无上游进度时显示阶段进度，不伪造精确百分比）。

**支付回调与算力入账**：创建订单快照 → 创建渠道支付单 → 接收 webhook → 验证签名、商户号、金额、订单状态、回调 nonce → 事务内更新 payment/order、写 power ledger、激活 entitlement、写 outbox → 返回渠道成功；重复回调返回成功但不重复入账。

**API Key 调用**：网关读取 `Authorization: Bearer` → 对 secret 哈希查找 enabled key → 检查 IP/并发/日额度和 scope → 解析模型别名和价格版本 → 生成同步或异步任务 → 按 API Key owner 扣账 → 响应中返回 usage、job id 和 request id；secret 只在创建时返回一次。

**CLI/MCP 授权**：CLI `device/start` 获取 user code 和 device code → 用户浏览器登录并确认 scope → 服务端记录 approve/deny → CLI 轮询得到 access/refresh token → MCP `initialize` 后 `tools/list` → 对有成本的 tool 先返回 quote → `tools/call` 必须携带 quote confirmation → 写审计和 job；设备 token 过期/撤销后所有调用立即拒绝。

## 6. 数据库表设计（PostgreSQL）

### 6.1 公共约定

所有业务表默认包含 `id uuid primary key`、`created_at timestamptz not null default now()`、`updated_at timestamptz not null default now()`（纯流水表可无 `updated_at`）。ID 采用 UUIDv7；删除优先使用 `deleted_at` 软删除。金额使用 `numeric(20,6)`，算力和字节数使用 `bigint`；枚举使用 `text + check`，便于版本演进。敏感字段加密或只存哈希。

### 6.2 用户与认证

| 表 | 关键字段（类型/约束） | 索引 | 关系 |
|---|---|---|---|
| `users` | `id uuid PK`；`username varchar(64) UNIQUE`；`status text CHECK(active,locked,disabled)`；`display_name varchar(80)`；`avatar_object_key text`；`invite_code_id uuid FK`；`last_login_at timestamptz` | `(status,created_at)`、`lower(username)` | 被所有 user-owned 表引用 |
| `auth_identities` | `id uuid PK`；`user_id uuid FK users`；`type text CHECK(phone,email,password)`；`identifier_hash bytea`；`identifier_masked varchar(128)`；`password_hash text`；`verified_at`；`UNIQUE(type,identifier_hash)` | `(user_id,type)` | 一个用户可有多个登录身份 |
| `user_sessions` | `id uuid PK`；`user_id FK`；`client_type text CHECK(pc,mobile,cli)`；`refresh_token_hash bytea UNIQUE`；`device_id`；`expires_at`；`revoked_at`；`last_seen_at` | `(user_id,revoked_at,expires_at)` | 用户 1:N 会话 |
| `otp_codes` | `id uuid PK`；`purpose text`；`channel text`；`target_hash bytea`；`code_hash bytea`；`attempts smallint`；`expires_at`；`used_at` | `(target_hash,purpose,created_at desc)` | 发码记录，使用后不可复用 |
| `captcha_challenges` | `id uuid PK`；`provider text`；`challenge_hash bytea`；`purpose`；`expires_at`；`verified_at` | `(challenge_hash,expires_at)` | 登录/发码前置校验 |
| `invite_codes` | `id uuid PK`；`code varchar(32) UNIQUE`；`owner_user_id FK`；`max_uses int`；`used_count int`；`status text`；`expires_at` | `(owner_user_id,status)` | 生成邀请码的用户 |
| `user_invites` | `id uuid PK`；`invite_code_id FK`；`invited_user_id FK users UNIQUE`；`attributed_at`；`qualified_at` | `(invite_code_id,created_at)` | 邀请归因和有效用户判定 |
| `user_preferences` | `user_id uuid PK/FK`；`locale`；`timezone`；`theme`；`notice_settings jsonb` | 无需额外索引 | users 1:1 |

### 6.3 模型目录与价格

| 表 | 关键字段（类型/约束） | 索引 | 关系 |
|---|---|---|---|
| `model_providers` | `id uuid PK`；`name varchar(80)`；`kind text`；`base_url text`；`credential_ref text`；`status text`；`health_status text` | `(status,health_status)` | provider 1:N models |
| `provider_models` | `id uuid PK`；`provider_id FK`；`external_model_id text`；`display_name`；`capabilities jsonb`；`status`；`UNIQUE(provider_id,external_model_id)` | `(provider_id,status)`、GIN `capabilities` | 上游原始模型 |
| `model_aliases` | `id uuid PK`；`alias varchar(80) UNIQUE`；`display_name`；`capability text`；`status`；`sort_order int`；`published_at` | `(capability,status,sort_order)` | 对外稳定模型名 |
| `model_alias_capabilities` | `alias_id FK`；`capability text`；`config jsonb`；`PRIMARY KEY(alias_id,capability)` | `(capability,alias_id)` | alias 与能力参数 |
| `model_routes` | `id uuid PK`；`alias_id FK`；`provider_model_id FK`；`priority int`；`weight int`；`conditions jsonb`；`status` | `(alias_id,status,priority)` | 一个 alias 可有多条路由 |
| `model_prices` | `id uuid PK`；`alias_id FK`；`unit text`；`input_price numeric`；`output_price numeric`；`flat_price numeric`；`member_discount jsonb`；`price_version_id FK`；`effective_from/to` | `(alias_id,effective_from desc)` | 价格版本下的条目 |
| `model_price_versions` | `id uuid PK`；`version varchar(32) UNIQUE`；`status draft/published/retired`；`published_by FK users`；`published_at` | `(status,published_at desc)` | 报价和任务保存版本快照 |

### 6.4 会话、任务、供应商请求

| 表 | 关键字段（类型/约束） | 索引 | 关系 |
|---|---|---|---|
| `chat_sessions` | `id uuid PK`；`user_id FK`；`title varchar(160)`；`model_alias_id FK`；`status`；`last_message_at` | `(user_id,last_message_at desc)` | 用户 1:N 会话 |
| `chat_messages` | `id uuid PK`；`session_id FK`；`role text`；`content jsonb`；`sequence bigint`；`job_id FK nullable`；`usage jsonb`；`UNIQUE(session_id,sequence)` | `(session_id,sequence)` | 会话消息顺序不可变 |
| `chat_attachments` | `message_id FK`；`file_object_id FK`；`kind`；`PRIMARY KEY(message_id,file_object_id)` | `(file_object_id)` | 消息引用上传文件 |
| `generation_jobs` | `id uuid PK`；`public_id varchar(40) UNIQUE`；`user_id FK`；`kind`；`model_alias_id FK`；`status`；`request_snapshot jsonb`；`price_version_id FK`；`quoted_power bigint`；`charged_power bigint`；`quote_id FK`；`provider_task_id`；`error_code`；`unknown_since`；`started_at/completed_at` | `(user_id,created_at desc)`、`(status,updated_at)`、`provider_task_id` | 任务主表，关联账务和结果 |
| `generation_job_events` | `id uuid PK`；`job_id FK`；`from_status`；`to_status`；`event_type`；`payload jsonb`；`created_at` | `(job_id,created_at)` | 状态审计/重放 |
| `generation_results` | `id uuid PK`；`job_id FK UNIQUE`；`file_object_id FK`；`result_index smallint`；`metadata jsonb` | `(job_id,result_index)` | 一任务可多结果 |
| `idempotency_records` | `id uuid PK`；`scope`；`key_hash bytea`；`user_id FK nullable`；`request_hash bytea`；`response_code int`；`response_body jsonb`；`expires_at`；`UNIQUE(scope,key_hash)` | `(user_id,created_at desc)` | 支付、生成、兑换、提现幂等 |
| `provider_requests` | `id uuid PK`；`job_id FK`；`provider_id FK`；`request_id`；`attempt int`；`request_meta jsonb`；`response_meta jsonb`；`status`；`latency_ms` | `(job_id,created_at)`、`(provider_id,status,created_at)` | 不保存完整 secret/prompt |

### 6.5 文件、作品与配额

| 表 | 关键字段（类型/约束） | 索引 | 关系 |
|---|---|---|---|
| `file_objects` | `id uuid PK`；`owner_user_id FK`；`object_key text UNIQUE`；`bucket`；`mime_type`；`size_bytes bigint CHECK >=0`；`checksum`；`scan_status`；`width/height/duration`；`deleted_at` | `(owner_user_id,created_at desc)`、`(scan_status)` | 上传文件和生成结果 |
| `works` | `id uuid PK`；`owner_user_id FK`；`type`；`title`；`category_id FK nullable`；`visibility`；`source_job_id FK nullable`；`deleted_at` | `(owner_user_id,type,created_at desc)`、`(visibility,created_at desc)` | 作品聚合多个资产 |
| `work_assets` | `work_id FK`；`file_object_id FK`；`role`；`sort_order`；`PRIMARY KEY(work_id,file_object_id)` | `(file_object_id)` | works N:M files |
| `asset_categories` | `id uuid PK`；`owner_user_id FK nullable`；`name`；`type`；`UNIQUE(owner_user_id,type,name)` | `(owner_user_id,type)` | 系统/用户分类 |
| `storage_quotas` | `user_id uuid PK/FK`；`quota_bytes`；`used_bytes`；`file_count`；`updated_at` | 无需额外索引 | 配额行锁更新 |
| `storage_packages` | `id uuid PK`；`name`；`bytes`；`price numeric`；`status`；`sort_order` | `(status,sort_order)` | 扩容商品 |

### 6.6 账务、会员、订单与支付

| 表 | 关键字段（类型/约束） | 索引 | 关系 |
|---|---|---|---|
| `power_accounts` | `user_id uuid PK/FK`；`available_power bigint CHECK >=0`；`frozen_power bigint CHECK >=0`；`version bigint` | 无；更新必须 `FOR UPDATE` | 用户算力账户 1:1 |
| `power_ledger` | `id uuid PK`；`user_id FK`；`account_id FK`；`direction credit/debit`；`amount bigint CHECK >0`；`balance_after bigint`；`source_type`；`source_id`；`idempotency_key`；`reversal_of FK nullable`；`metadata jsonb` | `(user_id,created_at desc)`、`(source_type,source_id)`、`UNIQUE(user_id,idempotency_key)` | 只追加，不更新历史金额 |
| `membership_plans` | `id uuid PK`；`name`；`tier`；`period`；`price numeric`；`currency`；`status`；`sort_order` | `(status,sort_order)` | 会员套餐 |
| `membership_plan_benefits` | `plan_id FK`；`benefit_key`；`value jsonb`；`PRIMARY KEY(plan_id,benefit_key)` | 无需额外索引 | 套餐权益 |
| `user_entitlements` | `id uuid PK`；`user_id FK`；`source_type`；`source_id`；`benefit_key`；`value jsonb`；`starts_at`；`ends_at`；`status` | `(user_id,benefit_key,status,ends_at)` | 会员/激活码/奖励权益 |
| `orders` | `id uuid PK`；`order_no varchar(40) UNIQUE`；`user_id FK`；`kind`；`product_snapshot jsonb`；`amount numeric`；`currency`；`status`；`idempotency_key`；`expires_at` | `(user_id,created_at desc)`、`(status,expires_at)` | 充值/会员/空间订单 |
| `payments` | `id uuid PK`；`order_id FK`；`channel`；`channel_trade_no`；`amount numeric`；`status`；`raw_meta jsonb`；`paid_at`；`UNIQUE(channel,channel_trade_no)` | `(order_id)`、`(status,updated_at)` | 订单可有多次支付尝试 |
| `refunds` | `id uuid PK`；`order_id FK`；`payment_id FK`；`amount numeric`；`reason`；`status`；`channel_refund_no` | `(order_id,created_at)` | 退款与算力冲正关联 |
| `redeem_codes` | `id uuid PK`；`code_hash bytea UNIQUE`；`code_prefix`；`kind`；`payload jsonb`；`max_uses`；`used_count`；`expires_at`；`status` | `(status,expires_at)` | 兑换码只存 hash |
| `redeem_code_redemptions` | `id uuid PK`；`redeem_code_id FK`；`user_id FK`；`ledger_id FK nullable`；`entitlement_id FK nullable`；`idempotency_key`；`UNIQUE(redeem_code_id,user_id)` | `(user_id,created_at desc)` | 兑换流水 |

账务关键规则：扣减时锁定 `power_accounts` 并检查可用余额；余额、账本和业务主记录必须同一事务；历史流水禁止 UPDATE/DELETE，只能追加 `reversal_of` 冲正。所有金额和算力来源必须能通过 `source_type/source_id` 反查订单、任务或奖励。

### 6.7 API Key、设备授权和 MCP

| 表 | 关键字段（类型/约束） | 索引 | 关系 |
|---|---|---|---|
| `api_keys` | `id uuid PK`；`user_id FK`；`prefix varchar(16) UNIQUE`；`secret_hash bytea`；`name`；`status`；`rate_limit int`；`expires_at`；`last_used_at`；`revoked_at` | `(user_id,status)`、`(prefix)` | 用户可有多个 Key |
| `api_key_scopes` | `api_key_id FK`；`scope`；`PRIMARY KEY(api_key_id,scope)` | `(scope,api_key_id)` | scope 明细 |
| `devices` | `id uuid PK`；`user_id FK nullable`；`device_name`；`platform`；`last_seen_at`；`revoked_at` | `(user_id,revoked_at)` | CLI/移动端设备 |
| `device_tokens` | `id uuid PK`；`device_id FK`；`token_hash bytea UNIQUE`；`type access/refresh`；`expires_at`；`revoked_at` | `(device_id,type,revoked_at)` | 设备令牌 |
| `device_authorization_requests` | `id uuid PK`；`device_code_hash bytea UNIQUE`；`user_code_hash`；`requested_scopes jsonb`；`status pending/approved/denied/expired`；`user_id FK nullable`；`expires_at`；`approved_at` | `(user_code_hash,status)`、`(device_code_hash,status)` | device flow |
| `quotes` | `id uuid PK`；`user_id FK`；`kind`；`model_alias_id FK`；`parameter_snapshot jsonb`；`price_version_id FK`；`power_amount bigint`；`status`；`expires_at`；`confirm_token_hash` | `(user_id,status,expires_at)` | 报价快照 |
| `quote_confirmations` | `id uuid PK`；`quote_id FK UNIQUE`；`user_id FK`；`confirmed_at`；`consumed_at`；`idempotency_key` | `(user_id,confirmed_at desc)` | 报价确认 |
| `mcp_audit_logs` | `id uuid PK`；`user_id FK`；`api_key_id FK nullable`；`device_id FK nullable`；`tool_name`；`quote_id FK nullable`；`job_id FK nullable`；`request_meta jsonb`；`result_code` | `(user_id,created_at desc)`、`(tool_name,created_at desc)` | MCP 每次调用审计 |

### 6.8 智能体、课程、灵感与审核

| 表 | 关键字段（类型/约束） | 索引 | 关系 |
|---|---|---|---|
| `agents` | `id uuid PK`；`owner_user_id FK`；`slug UNIQUE`；`name`；`description`；`category`；`status`；`published_version_id FK nullable`；`deleted_at` | `(status,category,created_at desc)`、`(owner_user_id,status)` | 智能体主记录 |
| `agent_versions` | `id uuid PK`；`agent_id FK`；`version_no int`；`system_prompt`；`tool_config jsonb`；`model_policy jsonb`；`status`；`UNIQUE(agent_id,version_no)` | `(agent_id,status)` | 不可变版本 |
| `agent_reviews` | `id uuid PK`；`agent_version_id FK`；`reviewer_id FK`；`decision`；`reason`；`created_at` | `(agent_version_id,created_at desc)` | 审核记录 |
| `agent_favorites` | `user_id FK`；`agent_id FK`；`PRIMARY KEY(user_id,agent_id)` | `(agent_id,created_at desc)` | 用户收藏 |
| `agent_runs` | `id uuid PK`；`agent_id FK`；`version_id FK`；`user_id FK`；`input_snapshot jsonb`；`job_id FK nullable`；`status`；`usage jsonb` | `(user_id,created_at desc)`、`(agent_id,created_at desc)` | 运行历史 |
| `courses` | `id uuid PK`；`owner_user_id FK`；`title`；`summary`；`cover_object_key`；`status`；`price numeric`；`published_at` | `(status,published_at desc)` | 课程主记录 |
| `course_chapters` | `id uuid PK`；`course_id FK`；`title`；`sort_order`；`status`；`UNIQUE(course_id,sort_order)` | `(course_id,sort_order)` | 章节 |
| `course_contents` | `id uuid PK`；`chapter_id FK`；`type`；`title`；`body jsonb`；`file_object_id FK nullable`；`sort_order` | `(chapter_id,sort_order)` | 章节内容 |
| `course_enrollments` | `id uuid PK`；`course_id FK`；`user_id FK`；`order_id FK nullable`；`status`；`enrolled_at`；`UNIQUE(course_id,user_id)` | `(user_id,status)` | 报名关系 |
| `course_progress` | `enrollment_id FK`；`content_id FK`；`progress numeric`；`completed_at`；`PRIMARY KEY(enrollment_id,content_id)` | `(enrollment_id,completed_at)` | 学习进度 |
| `inspiration_works` | `id uuid PK`；`owner_user_id FK`；`work_id FK`；`title`；`description`；`tags text[]`；`status`；`published_at` | GIN `tags`、`(status,published_at desc)` | 灵感作品引用作品资产 |
| `inspiration_favorites` | `user_id FK`；`inspiration_id FK`；`PRIMARY KEY(user_id,inspiration_id)` | `(inspiration_id)` | 收藏 |
| `content_reviews` | `id uuid PK`；`content_type`；`content_id uuid`；`version_id uuid nullable`；`reviewer_id FK`；`decision`；`risk_level`；`reason`；`created_at` | `(content_type,content_id,created_at desc)` | 通用审核记录 |

### 6.9 推广、钱包、反馈和运营

| 表 | 关键字段（类型/约束） | 索引 | 关系 |
|---|---|---|---|
| `share_campaigns` | `id uuid PK`；`name`；`reward_policy jsonb`；`starts_at/ends_at`；`status` | `(status,starts_at)` | 活动规则 |
| `share_links` | `id uuid PK`；`campaign_id FK`；`owner_user_id FK`；`channel`；`code UNIQUE`；`landing_path`；`status` | `(owner_user_id,campaign_id)`、`(code)` | 可追踪分享链接 |
| `referrals` | `id uuid PK`；`share_link_id FK`；`invited_user_id FK`；`event_type`；`event_at`；`dedupe_key UNIQUE` | `(share_link_id,event_at desc)`、`(invited_user_id)` | 点击/注册/转化事件 |
| `reward_ledger` | `id uuid PK`；`user_id FK`；`kind power/cash`；`amount numeric`；`source_type`；`source_id`；`status`；`reversal_of FK` | `(user_id,created_at desc)`、`(source_type,source_id)` | 奖励只追加/冲正 |
| `wallet_accounts` | `user_id uuid PK/FK`；`available_amount numeric`；`frozen_amount numeric`；`currency` | 无 | 现金收益账户 |
| `wallet_ledger` | `id uuid PK`；`user_id FK`；`direction`；`amount numeric`；`balance_after numeric`；`source_type`；`source_id`；`reversal_of` | `(user_id,created_at desc)` | 钱包流水 |
| `withdrawals` | `id uuid PK`；`withdrawal_no UNIQUE`；`user_id FK`；`amount numeric`；`fee numeric`；`channel`；`account_ciphertext`；`status`；`reviewer_id FK nullable`；`paid_at`；`failure_reason` | `(user_id,created_at desc)`、`(status,created_at)` | 提现状态机 |
| `feedback_tickets` | `id uuid PK`；`ticket_no UNIQUE`；`user_id FK nullable`；`type`；`subject`；`content`；`status`；`priority`；`assignee_id FK nullable` | `(user_id,created_at desc)`、`(status,priority)` | 客服/反馈 |
| `announcements` | `id uuid PK`；`title`；`body`；`audience jsonb`；`status`；`starts_at/ends_at`；`created_by FK` | `(status,starts_at desc)` | 公告 |
| `site_configs` | `key varchar(120) PK`；`value jsonb`；`version int`；`updated_by FK` | `(updated_at desc)` | 动态开关和文案 |
| `audit_logs` | `id uuid PK`；`actor_type`；`actor_id`；`action`；`resource_type`；`resource_id`；`before_json`；`after_json`；`ip_hash`；`request_id` | `(resource_type,resource_id,created_at desc)`、`(actor_id,created_at desc)` | 高风险操作不可删除 |
| `outbox_events` | `id uuid PK`；`topic`；`aggregate_type`；`aggregate_id`；`payload jsonb`；`status`；`attempts`；`available_at`；`published_at` | `(status,available_at)`、`(aggregate_type,aggregate_id)` | 事务内写入，Worker 发布 |

表关系主线：`users -> power_accounts -> power_ledger`；`users -> orders -> payments -> refunds`；`model_aliases -> model_prices -> quotes -> generation_jobs -> generation_results -> works -> file_objects`；`users -> agents/courses/inspirations/share_links/wallet_accounts`。所有跨域关联保留来源 ID 和快照，避免后续改价或删内容改变历史事实。

## 7. 前端工程拆分

### 7.1 路由、状态和 SDK

```text
src/
  app/                 # AppShell、路由、权限守卫、错误边界
  api/                 # OpenAPI 生成类型、fetch client、SSE、上传 client
  stores/              # auth、user、catalog、billing、workspace、jobs、works、membership
  components/          # 表单、卡片、任务、弹窗、状态组件
  pages/               # 按路由组织页面
  composables/         # useQuote、useJobPolling、useUpload、usePagination
  styles/              # token、响应式断点、主题
```

路由守卫区分 `public`、`auth`、`member`、`admin`、`api-key`、`device-authorized`；不能只根据前端菜单隐藏来实现权限。首次 401 只允许静默刷新一次，刷新失败清空内存 token 并回到登录页；所有 redirect 仅允许站内路径，防止开放重定向。

API SDK 由 OpenAPI 生成请求/响应类型，统一注入 `Authorization`、`Idempotency-Key`、`X-Request-ID`。SSE 封装事件重连、`lastEventId` 和结束态；轮询使用指数退避和最大时长。上传封装预签名、分片（如启用）、进度、取消、重试和扫描失败。

### 7.2 Pinia store 责任

| Store | 数据 | 失效策略 |
|---|---|---|
| `auth` | access token 内存态、用户会话、登录配置 | 退出/401 立即清空 |
| `user` | `/auth/me`、偏好、联系方式 | 资料保存后刷新 |
| `catalog` | alias、能力、价格版本 | 5 分钟缓存，发布事件失效 |
| `billing` | 余额、流水、订单、会员权益 | 支付/生成完成后主动刷新 |
| `workspace` | 当前会话、草稿、选择模型 | 本地草稿按会话隔离 |
| `jobs` | 任务状态、重试 cursor、错误 | 页面卸载后仍可恢复查询 |
| `works` | 作品列表、分类、容量 | 生成/删除/上传后局部失效 |
| `membership` | 套餐和 entitlement | 订单完成后刷新 |

草稿只存非敏感输入和 object key，不存 access token、支付数据或完整 API Key。浏览器崩溃恢复时显示“恢复草稿”而不是静默覆盖当前输入。

### 7.3 页面与组件验收要点

每个页面必须有可截图的线框标注：桌面主容器（建议 1200–1440px）、侧栏（240–280px）、顶部导航高度、固定底部操作区、弹窗宽度、移动端折叠规则。组件必须覆盖键盘焦点、Esc 关闭、表单字段错误、重复点击、断网、重新登录、权限不足和接口重试。埋点至少包括 `page_view`、`cta_click`、`quote_created`、`job_submitted`、`job_completed`、`payment_created`、`payment_succeeded`、`error_shown`，事件不得带 prompt 原文、手机号或 Key secret。

## 8. 开发阶段、依赖顺序与团队分工

### 8.1 依赖图

```text
Phase 0 工程准备
      |
      +--> Phase 1 基础平台 ----+
      |                         +--> Phase 3 工作台/聊天 --> Phase 4 媒体生成
      +--> Phase 2 账务商业化 --+                                  |
                                +--> Phase 5 API Key/开发者接入 --+  |
                                                                  +--> Phase 6 智能体/课程/灵感
                                                                  +--> Phase 7 推广/钱包/渠道/CLI/MCP
                                                                  +--> Phase 8 生态增强
```

### 8.2 阶段计划

| 阶段 | 优先级/依赖 | 交付物 | 可并行任务 | 完成标准与主要风险 |
|---|---|---|---|---|
| Phase 0 产品冻结与工程准备 | P0；无前置 | 线框、OpenAPI、错误码、状态机、迁移骨架、CI、环境和 secrets 方案 | 产品冻结、前端壳、Go 骨架、DB/Redis/S3 初始化 | 契约评审通过；风险是隐含规则未确认 |
| Phase 1 基础平台 | P0；Phase 0 | Auth/Session/OTP/Captcha、用户资料、模型目录、站点配置、最小后台、上传 | 前端登录/个人中心；后端 Auth/Catalog；QA 契约测试 | 可注册、登录、刷新、读模型、上传并扫描；风险是第三方验证码/短信 |
| Phase 2 账务与商业化 | P0；Phase 1 | power account/ledger、套餐、订单、支付、兑换、权益和后台对账 | 支付适配、账务并发测试、会员页、流水页 | 充值回调幂等且只入账一次；风险是渠道审核和退款规则 |
| Phase 3 工作台与聊天 | P0；Phase 1+2 | 工作台壳、会话/消息、SSE、模型选择、附件、断线恢复 | 前端聊天/历史；后端 Chat/SSE；Provider mock | 首 token、断线补齐、余额不足和安全拦截可验证 |
| Phase 4 媒体生成 | P0；Phase 2+3+Provider sandbox | 报价/确认/图片视频/TTS/ASR、Worker、任务中心、作品空间、失败退款 | Provider adapters、任务状态机、作品页 | queued→succeeded/failed/unknown 可对账；风险是上游不稳定和媒体转码 |
| Phase 5 API Key/开发者接入 | P0/P1；Phase 1+2+4 | Key 管理、`/api/v1`、scope、限流、开发者文档、调用计量 | OpenAPI/SDK、Key 网关、文档站 | Key secret 只显示一次，API 调用可追溯扣费；风险是滥用和成本失控 |
| Phase 6 智能体/课程/灵感 | P1；Phase 3+4+审核 | 创建/版本/审核/发布、课程学习、灵感广场、收藏 | 内容前端、审核后台、搜索/推荐 | 发布内容可追溯版本，违规内容可下架 |
| Phase 7 推广/钱包/渠道/CLI/MCP | P1；账务、权限、审计 | 邀请/分享/奖励、提现、渠道看板、device flow、MCP | 钱包、渠道、CLI、MCP 可分开开发 | 奖励不重复、提现有人工审批、设备可撤销 |
| Phase 8 生态增强 | P2；核心链路稳定 | Android、Codex++、CC Switch、BOLI-Claw、Vibe Hub、企业多租户 | 各产品线并行 | 不能影响 P0 账务和任务；需独立灰度 |

建议团队：产品/设计 1–2 人负责线框和验收；Go 后端 2–4 人按 Auth/账务、生成/Worker、内容/生态分工；前端 2–3 人按主站/工作台、支付/用户、开发者/内容分工；QA 1–2 人；DevOps/SRE 0.5–1 人；安全/财务顾问按评审节点介入。每阶段拆成可在 1–2 周完成的 story，接口、迁移和验收用例先于页面联调。

## 9. 测试、验收与 Definition of Done

### 9.1 测试矩阵

- 单元测试：状态机、价格计算、会员折扣、scope、权限策略、幂等摘要和退款金额。
- Repository/迁移集成测试：外键、唯一约束、软删除、并发锁、时区和金额精度。
- API contract tests：OpenAPI schema、错误码、分页、ETag/缓存（如启用）、SSE 事件序列。
- Provider mock tests：超时、重复回调、部分结果、未知状态、限频、错误映射；禁止依赖真实供应商才能通过 CI。
- 账务并发：同一账户并发扣减、重复 webhook、重复兑换、重复生成；断言余额不为负且 ledger 可重算。
- 任务恢复：Worker 崩溃、Redis 重启、HTTP 断线、上游 unknown、重启后不重复扣费。
- 文件与内容安全：大小/MIME/扩展名伪装、病毒扫描失败、越权下载、提示词/图片/音频拦截。
- 前端 E2E（Playwright）：PC/mobile 登录态、路由恢复、报价确认、支付结果、作品下载、断网重试、重复提交和权限拦截。
- 性能与安全：API/队列压测、慢查询、SQL 注入、XSS/CSRF、JWT/Key 泄露、越权、限流绕过和依赖漏洞。
- 灾备：数据库备份恢复、对象存储丢失演练、Redis 重建、outbox 重放、支付和任务对账。

### 9.2 Definition of Done

一个 story 只有同时满足以下条件才可关闭：

1. 页面线框、API schema、数据库 migration 和验收用例已评审。
2. 正常、空态、loading、异常、权限不足和重复提交均有实现及测试。
3. Go 单测/集成测通过，前端类型检查、lint、构建和相关 E2E 通过。
4. 产生的账务、任务、支付、审核变化均有审计或 outbox 记录。
5. OpenAPI、错误码、埋点和运行手册同步更新；无 secret、PII 或 prompt 原文进入日志。
6. staging 环境完成产品、QA、安全和（涉及支付/退款时）财务验收。

## 10. 非功能要求与上线门槛

### 10.1 性能、可靠性和一致性

- 普通读 API P95 < 500ms；写 API（不含上游生成）P95 < 1s；列表接口 P95 < 800ms。
- 聊天 SSE 首 token 目标 < 3s（受上游影响时记录 provider latency）；断线后 30s 内可恢复历史。
- 支付 webhook 99.9% 可重放；相同回调重复 N 次只产生一条成功入账。
- 任务状态最终一致：正常供应商任务在 SLA 内达到终态；unknown 明确展示并进入对账队列。
- 账本余额不可为负，账本按 `source_id` 可重算；账务事务提交后才向用户返回成功。
- 初期目标：API 100 RPS、任务队列 20 个并发 Worker、单用户生成并发由套餐和风控配置；压测后再调高。

### 10.2 安全、隐私和运营

- JWT access 短时有效，refresh 可撤销并绑定设备；API Key 只存 hash，展示一次；CLI token 可单设备撤销。
- 全站 HTTPS、Secure/HttpOnly/SameSite Cookie（如采用 Cookie）、CSP、CORS 白名单、CSRF 防护、登录/发码/Key/提现限流。
- 用户上传默认私有，下载 URL 有效期 5–15 分钟；病毒扫描和 MIME 检测未通过不得被生成任务引用。
- API Key、支付凭证、短信验证码、身份证/提现账号均不得进入普通日志；审计日志按最小权限访问。
- RPO ≤ 15 分钟，RTO ≤ 2 小时；PostgreSQL 每日全量+持续归档，跨可用区保留；对象存储开启版本/生命周期策略。
- 指标：API 延迟/错误、队列堆积、Provider 成功率/延迟、unknown 数、扣费/退款差异、支付回调延迟、余额异常、存储扫描失败。

### 10.3 发布、灰度与回滚

迁移采用向前兼容的 expand/contract；禁止在高峰执行破坏性 DDL。新模型、价格、会员和页面使用 feature flag；先 staging，再 1%–10% 灰度，观察错误率、成本和账务差异后扩大。代码回滚不能回滚已提交账务；若出现账务异常，冻结相关产品开关并通过冲正/人工对账修复。每次发布必须有 migration 回滚/前滚说明、监控面板、值班人和回滚命令。

## 11. 实施前必须确认的产品决策

以下事项原 PRD 尚未给出唯一答案，不能由开发人员自行猜测，须在 Phase 0 冻结：

1. 算力最小单位、不同模型的计费公式、会员折扣叠加顺序和退款比例。
2. 手机/邮箱登录是否都启用、密码策略、验证码供应商、失败锁定阈值和 PC/mobile token 的具体 TTL。
3. 视频/音频最大文件、时长、分辨率、并发、排队 SLA 及是否允许取消。
4. 支付渠道、订单过期时间、部分退款和人工对账流程。
5. 会员权益中“作品空间/现金返佣/图像额度”的准确口径与冻结/到期规则。
6. 智能体、课程、灵感是否收费，审核角色、内容安全标准和收益分配比例。
7. API Key 是否允许直接扣费、月度额度/超额策略、企业账户和多租户边界。
8. CLI/MCP 可用工具白名单、报价确认交互和设备授权最长有效期。
9. 安卓安装包是否需要会员/激活码，分发、签名和版本回滚责任人。

这些决策冻结后，应回写 OpenAPI、数据库 migration、页面线框和验收用例，形成单一可执行基线。

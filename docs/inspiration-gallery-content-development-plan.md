# 灵感广场案例内容建设与生产级运营计划

## 1. 文档目的

本计划用于分阶段完善前端 Web 的「灵感广场」，解决分类、风格、场景、语言等标签下案例不足或筛选后无结果的问题，并建立一套可持续运营的案例内容生产能力。

目标不是一次性填满所有标签组合，而是建立可追踪、可审核、可去重、可持续补齐的案例库：

- 用户可以稳定浏览真实可用的案例、提示词和生成结果。
- 运营人员可以在管理后台导入、编辑、审核、精选、下线和批量维护案例。
- 系统可以识别覆盖缺口，生成结构化案例草稿，并在人工确认后批量生成图片。
- 每个案例都有来源、许可证、署名、模型、成本和审核记录，能够支持生产环境追溯。
- 前端筛选能够解释“为什么没有结果”，并区分提示词语言与图片中文字语言。

## 2. 当前基线

本计划基于当前代码库现状制定，第一阶段不会修改业务代码。

### 2.1 当前实现

- 参考案例组件：`apps/web/src/components/workbench/galleryReference.ts`
- 灵感广场组件：`apps/web/src/components/workbench/GalleryPanel.tsx`
- 参考数据当前从 `awesome-gpt-image-2` 的 `data/cases.json` 运行时请求。
- 社区作品使用本地数据库表 `gallery_tags`、`gallery_items`。
- 社区作品服务：`services/api/internal/service/gallery.go`
- 管理后台审核页：`apps/admin/src/app/admin/gallery/page.tsx`
- 初始图库迁移：`infra/migrations/005_gallery.up.sql`

### 2.2 已发现的问题

- 参考数据约 541 个案例，已有分类、风格、场景体系，但它不是本地快照，生产环境依赖 GitHub Raw 的可用性。
- 当前筛选是 AND 逻辑；场景和语言同时筛选时，可能出现合法但为空的交集。
- 当前语言检测主要依据 Prompt 文本，不能代表图片中实际出现的文字语言。
- 参考案例与社区作品的来源、授权、模型、成本、审核状态等字段尚未形成统一契约。
- 缺少覆盖率报表、缺口任务、批量导入、批量审核和批量生成的完整闭环。

### 2.3 当前已存在的无关改动

工作区已有上一阶段 OpenRouter 音频模型接入的未提交修改。本计划不回滚、不覆盖这些修改；后续实施时只在与灵感广场直接相关的文件和迁移中增量修改。

## 3. 目标与非目标

### 3.1 目标

1. 建立统一案例数据模型和标签规范。
2. 将外部参考案例同步为可追溯的本地数据，并支持幂等更新。
3. 建立管理后台内容运营工作流。
4. 建立“缺口分析 -> 案例规格 -> 媒体生成 -> 质量审核 -> 发布”的闭环。
5. 改善前端筛选、空结果提示、相近案例和标签可见性。
6. 完成 API、Worker、Admin、Web、数据库迁移和运行态验证。

### 3.2 非目标

- 不直接复制没有明确授权的第三方 Prompt、图片或仓库内容。
- 不为了追求数量构造所有标签的笛卡尔积。
- 不在未经人工审核的情况下自动发布 AI 生成案例。
- 不把 GitHub Raw 或第三方图片 URL 作为生产环境唯一资源依赖。
- 本阶段不把案例库改造成开放式社交平台；社区投稿、评论、点赞等属于后续范围。

## 4. 核心产品原则

### 4.1 覆盖“有价值的组合”，不追求全组合

标签数量多时，全部组合会产生大量低质量或没有实际意义的案例。采用分层覆盖目标：

- 每个一级分类：至少 8～12 个可发布案例。
- 每个主要场景：至少 8～10 个可发布案例。
- 高频风格：至少 8 个可发布案例。
- 重点的“分类 + 场景”“分类 + 风格”“场景 + 语言”组合：每个 3～5 个可发布案例。
- 空结果组合：记录为“待建设”“无适用案例”或“标签定义需调整”，不简单用虚假案例填充。

### 4.2 来源、技术兼容、购买路径分开记录

案例来源、模型的技术能力、模型的费用和授权路径必须是不同字段或不同实体。不能因为某模型可以生成某类图片，就默认案例内容具备商业授权。

### 4.3 规格先行，生成与发布分离

AI 只负责生成结构化案例草稿和候选媒体；生成结果必须经过自动检查和人工审核后才能进入公开图库。

### 4.4 任何批量任务都可追溯、可重试、可停止

同步、导入、生成、审核和发布均需要任务状态、批次、失败原因、重试次数和操作者记录，避免一次批量操作造成不可逆的数据污染。

## 5. 统一案例数据契约

最终字段会结合现有数据库结构和接口风格落地，以下为目标契约。

### 5.1 案例主体

| 字段 | 说明 |
| --- | --- |
| `id` | 系统内部 UUID 或稳定主键 |
| `title` | 面向用户展示的案例名称 |
| `description` | 简短用途和效果说明 |
| `prompt` | 可复制的完整 Prompt |
| `prompt_language` | Prompt 使用语言，如 `zh-CN`、`en`、`ko` |
| `image_text_language` | 图片中实际文字语言，可多选或 `none` |
| `category_ids` | 分类标签 |
| `style_ids` | 风格标签 |
| `scene_ids` | 场景标签 |
| `aspect_ratio` | 画幅比例 |
| `output_media_type` | 当前以图片为主，预留视频等类型 |
| `preview_asset_id` | 生产环境托管的预览资源 |
| `source_type` | `official`、`community`、`imported`、`generated` |
| `source_url` | 原始来源链接 |
| `source_repository` | 来源仓库或供应商 |
| `source_item_id` | 外部案例 ID，支持幂等同步 |
| `license` | 许可证或授权说明 |
| `attribution` | 署名要求 |
| `model_provider` | 生成服务提供方 |
| `model_id` | 生成模型 |
| `generation_params` | 脱敏后的生成参数 |
| `estimated_cost` | 单案例成本估算 |
| `status` | 公共目录状态：兼容现有 `pending`、`approved`、`rejected`，后续增加 `offline` |
| `quality_score` | 自动检查和人工审核综合结果 |
| `content_hash` | Prompt、标签和资源的去重指纹 |
| `created_by` / `updated_by` | 操作者或任务身份 |
| `created_at` / `updated_at` | 时间戳 |

### 5.2 标签规范

标签必须有稳定的 `slug`、展示名、层级、排序、启用状态和定义说明。每个标签还要保留：

- 适用范围和反例。
- 推荐案例数量目标。
- 当前可发布案例数。
- 是否允许与其他标签组合。
- 同义词、旧标签和迁移关系。

### 5.3 审核记录

审核记录单独保存，不覆盖案例当前状态，至少包括：审核人、审核动作、审核意见、自动检查结果、审核时间和批次 ID。

## 6. 分阶段实施计划

每个阶段完成后都要暂停，由用户确认后才进入下一阶段。每次汇报必须包含：完成内容、变更文件、验证命令和结果、已知风险、是否建议继续、下一阶段范围。

### 阶段 1：方案与数据契约

**本次先完成的步骤：创建本计划文档。**

后续实施内容：

- 盘点现有图库表、API、Admin 页面和 Web 筛选逻辑。
- 确认统一案例字段、标签命名、状态机、来源与许可证字段。
- 明确本地数据库、对象存储和任务队列的边界。
- 补充迁移策略、回滚策略、数据备份策略。

验收标准：字段和状态可以覆盖现有参考案例、社区案例、导入案例和生成案例；没有把未确认的授权或成本写成事实。

### 阶段 2：覆盖率分析与缺口报告

- 编写可重复运行的统计脚本或服务接口。
- 统计分类、风格、场景、Prompt 语言、图片文字语言及重点组合。
- 输出当前数量、目标数量、缺口数量和无结果原因。
- 区分“确实没有案例”“筛选逻辑问题”“标签定义冲突”“来源数据未同步”。

验收标准：同一份数据重复运行得到稳定结果；后台或可下载报告能指导下一批建设任务。

#### 阶段 2 实际交付

- 分析脚本：[scripts/analyze-gallery-coverage.js](../scripts/analyze-gallery-coverage.js)
- Markdown 报告：[docs/inspiration-gallery-coverage-report.md](inspiration-gallery-coverage-report.md)
- JSON 报告：[docs/inspiration-gallery-coverage-report.json](inspiration-gallery-coverage-report.json)
- 项目命令：`GALLERY_COMMUNITY_URL=http://localhost:8080/api/gallery pnpm gallery:coverage`

本次快照结果：参考案例 541 个、13 个分类、19 个风格、10 个场景；本地 API 返回 4 个社区公开案例。参考案例单维度中分类和场景均达到每项 8 个的运营目标，8 个风格低于该目标。重点组合统计发现：分类 + 场景有 18 个零案例组合，分类 + 风格有 127 个，场景 + Prompt 语言有 18 个；用户截图对应的 `Creative + ko` 为 0。

语言统计目前只针对 Prompt/标题文本，图片内文字语言仍标记为未测量，不能用当前数字替代 OCR 结果。报告同时指出社区自由标签与参考案例 taxonomy 不同，不能直接相加计算总覆盖率。

验证已完成：Node.js 语法检查、帮助命令、带本地社区 API 的完整运行、JSON 解析和 `git diff --check` 均通过；本阶段没有生成或发布图片，也没有修改图库业务数据。运行态补充检查中，当前 3001 Admin 应用的 `/admin/gallery` 和 `/admin/dashboard` 返回 200；`/app/gallery` 请求到 3001 返回 404 是因为 Web 应用配置在 3000，且本次检查时 3000 未监听，因此 Web 页面运行态访问留到后续 Web 阶段专门验证。

### 阶段 3：本地案例库与来源同步

- 增加官方/参考案例快照或扩展现有案例表。
- 同步 `awesome-gpt-image-2` 等明确授权来源的数据。
- 保存源仓库、源案例 ID、源链接、许可证、署名和内容 hash。
- 将预览图片迁移到自有对象存储或可控的资源层。
- 使用 `source_repository + source_item_id` 和内容 hash 实现幂等同步。
- 设计失败重试、部分成功、重复数据和来源下线处理。

验收标准：断网或上游暂时不可用时，前端仍可浏览已同步案例；重复同步不会产生重复记录；来源和授权信息可追溯。

#### 阶段 3 实际交付：3A + 3B

- 新增 `gallery_reference_sources` 和 `gallery_reference_cases` 本地快照表，保留来源仓库、manifest、许可证、署名、源案例 ID、内容 hash、审核状态和资源地址。
- 新增 `pnpm gallery:sync -dry-run` 和 `pnpm gallery:sync -mirror-assets`，支持 manifest 校验、幂等 upsert、上游内容变化重置审核状态、资源镜像和失败统计。
- 资源镜像只接受 HTTP/HTTPS 的 PNG、JPEG、WebP、GIF；同时校验响应头、图片魔数、单文件 15 MB 上限和对象路径安全性。镜像不会自动改变 `pending_license`。
- 新增公开接口 `GET /api/gallery/references`。仅 `status='published'`、`is_active=true` 且已有自有资源地址的案例可返回；前端参考案例加载已切换到该接口。

本地验收结果：来源 541 条、资源 541/541 镜像成功、重复镜像第二次上传 0 条、镜像失败 0 条；数据库中 541 条仍为 `pending_license + unverified`，公开接口返回空案例列表，未审核内容未泄漏。实际资源通过 MinIO HTTP 200 和 JPEG 文件类型检查。

### 阶段 4：管理后台运营能力

- 案例列表、详情、预览和筛选。
- JSON/CSV 导入，导入前校验和错误行下载。
- 标签编辑、批量打标和标签合并迁移。
- 批量审核、精选、发布、下线和删除保护。
- 查看来源、许可证、模型、成本、生成批次和审核历史。
- 覆盖率仪表盘和缺口任务入口。
- 权限控制、操作审计和危险操作二次确认。

验收标准：运营人员不需要直接操作数据库即可完成一次导入、审核、发布和下线；非法字段、重复案例和无授权来源能够被拦截或明确提示。

### 阶段 5：AI 案例规格生成

- 根据缺口报告生成结构化案例草稿，不直接发布。
- 输出标题、用途、分类、风格、场景、语言、Prompt、预期图片文字、画幅、候选模型和验收标准。
- 支持一次生成多个候选，并说明选择理由和潜在风险。
- 对敏感品牌、人物、版权素材、真实机构和高风险内容做规则检查。
- 保存提示词版本、使用的模型、输入缺口和生成成本。

验收标准：生成结果可以被后台编辑；缺少必要字段时不能进入媒体生成；每个草稿都能追溯到具体缺口任务。

### 阶段 6：批量媒体生成

- 以批次和任务明细运行图片生成。
- 记录模型、参数、耗时、成本、输入规格、输出资源和错误原因。
- 支持并发上限、限流、超时、重试、取消和断点续跑。
- 生成结果进入 `pending_review`，不自动公开。
- 将成本与当前站的算力/积分转换规则关联，避免只展示 OpenRouter 原始价格。
- 对模型不可用、余额不足、内容拒绝和资源上传失败分别处理。

验收标准：小批量和失败重试均可验证；任务中断后可继续；每个生成结果都有成本和来源记录；不会因为单个任务失败而丢失整个批次。

### 阶段 7：质量审核与发布

- Prompt 与标签一致性检查。
- 图片 OCR，核对预期文字和文字语言。
- 图片与 Prompt 的语义/视觉一致性检查。
- 内容安全、品牌/人物/版权风险检查。
- 相似图片和重复 Prompt 检测。
- 自动检查通过后仍需人工审核，审核通过才可发布。
- 记录拒绝原因和返修建议，支持重新生成指定版本。

验收标准：审核人员能从案例详情看到全部证据；拒绝后可以定位原因并重新进入草稿或生成流程；发布和下线均保留审计记录。

### 阶段 8：前端灵感广场体验

- 标签显示可发布案例数量。
- 空结果时说明是筛选交集为空、暂时没有内容，还是数据加载失败。
- 提供移除某一筛选、查看相近案例和返回全部案例的操作。
- 将 Prompt 语言和图片文字语言分开筛选。
- 展示来源、模型、授权提示和复制 Prompt 的必要信息。
- 优化分页、加载、图片失败、断网缓存和移动端布局。
- 增加多语言 UI 文案，避免把标签内部值直接作为用户文案。

验收标准：截图中出现的“0 个结果”可以解释和恢复；筛选、分页、详情、复制 Prompt 和图片加载在 Web 运行态可用。

### 阶段 9：测试、上线与运营手册

- 数据库迁移和回滚测试。
- 同步幂等、导入校验、批量审核、任务重试和权限测试。
- API、Worker、Admin、Web 的单元测试和集成测试。
- 前端筛选、空状态、错误状态和响应式测试。
- 本地完整启动验证，包括端口、健康检查、登录、管理后台和 Web 页面。
- 建立备份、恢复、监控、告警、成本预算和回滚手册。
- 首批上线采用小批量灰度，观察数据质量和成本后再扩量。

验收标准：至少完成一条从缺口到发布的端到端流程，并能在失败时恢复；所有已知未验证项都写入交付记录。

## 7. 推荐的数据建设顺序

1. 先同步和清洗已有的高质量参考案例。
2. 再为数量不足的一级分类补齐通用案例。
3. 再补齐高频场景和高频风格。
4. 最后根据真实使用数据补重点组合和多语言案例。

首批案例优先选择容易验收、风险较低、复用价值高的方向：产品展示、海报/广告、社交媒体配图、信息图、包装概念、角色/场景设定和文字排版。涉及真实品牌、名人、受版权保护角色和商业素材的案例需要单独审核，不作为无条件批量生成目标。

## 8. 开源项目参考边界

以下项目只作为数据结构、展示方式或运营流程参考，是否使用内容必须单独核对许可证和来源：

- [freestylefly/awesome-gpt-image-2](https://github.com/freestylefly/awesome-gpt-image-2)：MIT，参考案例数据结构、分类/风格/场景组织方式。
- [kadevin/ilab-conjure](https://github.com/kadevin/ilab-conjure)：AGPL-3.0，参考图库、Prompt Chip 和生成队列的产品形态；引入代码前需要评估 AGPL 义务。
- [poloclub/diffusiondb](https://github.com/poloclub/diffusiondb)：MIT，参考大规模 Prompt 数据集和元数据设计。
- [YouMind-OpenLab/awesome-gemini-3-prompts](https://github.com/YouMind-OpenLab/awesome-gemini-3-prompts)：参考多语言案例展示；未确认明确许可证前不直接搬运内容。
- [ApiMartAI/best-gpt-image-2-prompts](https://github.com/ApiMartAI/best-gpt-image-2-prompts)：参考展示方式；不直接复制图片和 Prompt。

## 9. 成本与算力转换

成本设计分为三层，实施时以当前项目真实配置和 OpenRouter 官方价格为准：

1. **供应商成本**：记录实际模型、输入/输出计费单位、原始单价、请求量和失败请求是否计费。
2. **任务成本**：加上重试、图片存储、OCR、审核模型和队列运行成本。
3. **站内价格**：按照当前站已有算力/积分转换规则计算用户侧消耗，保留原始成本和转换版本，不能只保存一个最终数字。

任何价格写入生产配置前都要记录来源 URL、抓取时间、币种、单位和生效版本。OpenRouter 价格变化不能静默覆盖历史案例成本。

## 10. 风险与控制

| 风险 | 控制措施 |
| --- | --- |
| 第三方来源授权不清 | 只同步明确许可或已确认授权的内容，保存许可证和来源证据 |
| 上游 GitHub 不可用 | 本地快照和自有资源托管，定期同步但不依赖实时访问 |
| AI 图片与标签不匹配 | OCR、视觉检查、人工审核和拒绝原因记录 |
| 批量生成成本失控 | 批次预算、并发限制、预估成本、取消和告警 |
| 重复或近似案例过多 | Prompt hash、资源 hash、相似度检测和精选规则 |
| 标签过细导致大量空结果 | 标签定义、组合目标和覆盖率报告，不盲目增加标签 |
| 迁移影响现有图库 | 向前兼容、备份、可回滚迁移和小批量灰度 |
| 生产验证不足 | 每阶段必须有可复现验证命令，最后做 Web/Admin 运行态检查 |

## 11. 每阶段交付格式

每个阶段完成后固定输出：


- 本阶段完成了什么。
- 修改或新增了哪些文件、迁移和接口。
- 执行了哪些验证，结果是什么。
- 哪些内容尚未验证，原因是什么。
- 当前风险和需要用户决策的事项。
- 是否建议进入下一阶段。
- 下一阶段具体要做什么。

完成汇报后暂停，不自动进入下一阶段。只有用户明确确认后才继续。

## 11.1 阶段 1 核对结果与实施边界

本小步骤已根据当前代码和迁移文件完成现状核对，结论如下。

### 当前数据链路

| 链路 | 当前实现 | 结论 |
| --- | --- | --- |
| 参考案例 | `galleryReference.ts` 在浏览器直接请求 GitHub Raw 的 `cases.json`，图片也直接拼接 GitHub Raw URL | 先保留前端展示兼容，后续增加服务端快照和自有资源地址 |
| 社区作品 | `gallery_items` 关联可选的 `works`，通过 `/api/gallery` 查询 `approved` 数据 | 继续作为社区发布和付费复用的兼容链路 |
| 社区标签 | `gallery_tags` 加 `gallery_items.tags JSONB`，列表通过 JSONB 包含关系筛选 | 暂时保留；新标签体系不能依赖单一 JSONB 字段 |
| 用户发布 | `/api/works/:id/publish` 写入 `gallery_items`，受 `gallery_audit_required` 配置影响 | 不改变现有发布入口，增加来源和审核事件时做兼容扩展 |
| 管理审核 | `/admin/gallery`、`PATCH /admin/gallery/:id`、`DELETE /admin/gallery/:id` | 先兼容当前审核接口，再逐步增加批量和详细审核能力 |
| 付费复用 | `/api/gallery/:id/clone` 查询公开作品，必要时扣算力并写入 `gallery_purchases` | 不能让参考案例同步破坏现有付费复用和财务幂等逻辑 |

### 已确认的接口边界

当前公共接口为：

- `GET /api/gallery/tags`：只返回社区标签。
- `GET /api/gallery`：只查询 `gallery_items` 中 `status='approved'` 的社区作品，支持 `tag`、分页。
- `GET /api/gallery/:id`：查询一个已审核社区作品。
- `POST /api/gallery/:id/clone`：复制 Prompt 和模型信息，并处理付费复用。

当前管理接口为：

- `GET /admin/gallery`：按状态分页列出作品。
- `PATCH /admin/gallery/:id`：修改审核状态或精选状态。
- `DELETE /admin/gallery/:id`：删除作品。

参考案例没有经过 API、权限、缓存和数据库，前端直接使用 `ReferenceGalleryItem` 类型。后续统一目录接口必须保持这些现有接口可用，不能让旧版 Web 或用户发布流程一次性切换失败。

### 统一目录的保守设计

本项目采用“公共目录 + 生产工作流分离”的结构：

1. `gallery_items` 继续作为对外展示和复用的公共目录实体，保留 `work_id`、`user_id`、付费字段和现有状态查询。
2. 在 `gallery_items` 上增量补充来源、授权、语言、内容指纹和规范化标签关联，不把生成中的中间状态全部塞进公共表。
3. 增加 `gallery_reference_sources` 保存外部仓库、源案例 ID、源 URL、许可证、署名、同步时间和原始内容 hash。
4. 增加规范化标签实体和案例关联表，覆盖 `category`、`style`、`scene`、`prompt_language`、`image_text_language`；现有 `tags JSONB` 作为兼容字段逐步迁移。
5. 增加 `gallery_case_specs` 保存 AI 生成的案例规格草稿，包含缺口任务、提示词版本、预期文字、候选模型和验收标准。
6. 增加 `gallery_generation_batches` 与 `gallery_generation_tasks` 保存批量生成、重试、取消、成本和输出资源。
7. 增加 `gallery_quality_checks` 与 `gallery_review_events` 保存 OCR、重复度、内容安全、人工审核和拒绝原因。

这样可以让一个已发布案例同时拥有“来源证据”和“生成记录”，但不会把尚未审核的规格或媒体错误地暴露到公共目录。

### 状态机决策

公共目录 `gallery_items.status` 暂时采用兼容现状的状态：

```text
pending -> approved
pending -> rejected
approved -> offline
offline -> approved
```

其中 `approved` 继续代表当前 Web 可见状态，避免立即修改 `ListPublic`、详情页、付费复用和作品过期保护逻辑。`draft`、`generating`、`pending_review` 等生产状态放入案例规格和生成任务表，待质量审核通过后才创建或更新公共目录记录。

所有状态变更都必须写入审核事件，不能只覆盖当前状态。删除操作优先改为下线或软删除，保留财务、来源和审计关联；是否允许物理删除需要在后台权限和迁移阶段单独确认。

### 统一案例响应草案

后续服务端统一目录接口的响应项以以下结构为目标，字段可选性优先兼容旧客户端：

```json
{
  "public_id": "gal_xxx",
  "source_type": "official|community|imported|generated",
  "title": "案例标题",
  "description": "用途说明",
  "prompt": "完整提示词",
  "prompt_language": "zh-CN",
  "image_text_languages": ["zh-CN"],
  "category": "Products & E-commerce",
  "styles": ["Photography"],
  "scenes": ["Commerce"],
  "model_code": "image-model-code",
  "media": {
    "type": "image",
    "url": "https://assets.example.com/gallery/gal_xxx.webp",
    "thumbnail_url": "https://assets.example.com/gallery/gal_xxx-thumb.webp"
  },
  "license": "MIT",
  "attribution": "Source repository or author",
  "source_url": "https://example.com/source",
  "status": "approved",
  "is_featured": false,
  "price": 0,
  "created_at": "2026-10-01T00:00:00Z"
}
```

旧版社区字段 `cover_url`、`media_url`、`thumbnail_url`、`tags`、`is_paid`、`like_count` 必须继续返回。参考案例从 GitHub 迁移到服务端后，前端才切换到这个统一结构；在此之前不删除现有 `ReferenceGalleryItem` 适配器。

### 第一批改造文件边界

进入代码实施后，第一批只允许触碰以下范围：

- 数据库：新增灵感广场来源、规范化标签和案例目录兼容迁移；不重写历史迁移，不删除现有字段。
- API：扩展 Gallery DTO、公共目录查询和管理详情；保留旧接口与旧字段。
- Worker：只建立同步任务的最小骨架，暂不接入大规模图片生成。
- Admin：先展示来源、许可证、标签和审核信息，再增加导入和批量操作。
- Web：先增加统一响应适配和明确的来源/空状态展示，暂不一次性重做视觉布局。
- 测试：补充迁移、幂等同步、旧接口兼容和状态转换测试。

以下内容明确不在下一小步骤中：批量 AI 生成、自动发布、全量标签补齐、价格配置写入生产和第三方内容批量搬运。

### 仍需在实施前确认的决策

- 对象存储采用当前项目已有资源存储能力，还是新增独立 bucket；在代码库核对资源上传路径后确定。
- 规范化标签是直接扩展 `gallery_tags`，还是新增按类型区分的 taxonomy 表；以现有后台标签维护能力和迁移成本为准。
- `gallery_items` 的来源字段是直接扩展，还是通过 `gallery_reference_sources` 一对一关联；优先选择可回滚且不影响现有查询的方案。

### 阶段 3A：本地来源快照与幂等同步骨架

本阶段先完成阶段 3 中最小且可回滚的服务端数据落点，不把参考案例强行写入现有社区作品表：

- 新增 `gallery_reference_sources`：保存仓库、manifest、许可证证据、署名、manifest hash、同步状态和最后同步时间。
- 新增 `gallery_reference_cases`：保存源案例 ID、标题、Prompt、分类、风格、场景、源图片 URL、内容 hash、活跃状态和资源授权状态。
- 新增 `services/api/cmd/gallery-sync`：从官方 manifest 下载、校验 taxonomy、计算 SHA-256、在事务中幂等 upsert。
- 使用 `source_id + source_item_id` 作为同步幂等键；同步前将该来源旧案例标记为 inactive，再把本次 manifest 中的案例恢复为 active。
- 内容 hash 变化时清空已有本地资源 URL，并将状态重置为 `pending_license`，防止上游内容变化后沿用旧审核结果。
- 许可证边界采用保守默认：仓库 MIT 许可证证据已保存，但案例图片的独立授权尚未逐项核验，因此同步结果默认为 `asset_license_status=unverified`、`status=pending_license`。
- 现有 `gallery_items`、社区投稿、付费复用和前端 GitHub 参考链路暂不改变。

执行方式：

```bash
pnpm gallery:sync -dry-run
pnpm gallery:sync
```

生产环境应通过 `GALLERY_REFERENCE_MANIFEST_URL`、`GALLERY_REFERENCE_REPOSITORY_URL`、`GALLERY_REFERENCE_LICENSE_URL` 等环境变量显式配置来源信息。命令默认拒绝空 manifest、重复源案例 ID、未知分类/风格/场景和必填字段缺失。

本次验证结果：迁移 `141_gallery_reference_catalog` 已在当前 Docker API 使用的 PostgreSQL 实例中应用；干运行校验 541 个案例；真实同步执行两次后，数据库保持 541 条案例、1 个来源、541 个唯一源案例 ID、541 条待授权核验记录，无重复插入。Go 单元测试和 `gofmt` 已通过。

阶段 3A 的边界：还没有把案例快照接入公共 API，没有下载或托管第三方图片，也没有自动把任何案例标记为公开发布。这些放在阶段 3B，继续保留人工授权核验和资源镜像的安全闸门。
- 站内算力转换沿用现有 `price_rule` 语义，还是为案例生成任务增加独立成本版本；必须在阶段 6 前锁定。

## 12. 当前执行状态

- [x] 阶段 1：创建开发计划文档。
- [x] 阶段 1：确认数据契约、标签规范和状态机。
- [x] 阶段 2：输出覆盖率和缺口报告。
- [x] 阶段 3A：本地来源快照与幂等同步骨架。
- [x] 阶段 3B：资源镜像与服务端参考案例 API。
- [x] 阶段 4：管理后台运营能力。
- [x] 阶段 5：AI 案例规格生成。
- [x] 阶段 6：批量媒体生成。
- [x] 阶段 7：质量审核与发布。
- [x] 阶段 8：前端体验优化。
- [x] 阶段 9：测试、上线与运营手册。

### 阶段 3B 实际交付

- 新增公开参考案例接口，只有已发布、有效且已镜像自有资源的案例可返回；未完成许可证审核的快照不会泄漏到公共 API。
- Admin 参考案例运营页支持来源快照导入、许可证状态、批量审核、精选、发布、下线、详情编辑和覆盖率展示。
- 本地验收：参考案例 541 条，资源 541/541 镜像成功，重复镜像不会重复上传；Admin `/admin/gallery/references` 返回 200，公共 API 未暴露待审核案例。

### 阶段 4 实际交付

- 保留现有社区图库审核链路，并补充参考案例的来源、许可证和覆盖率运营边界。
- 管理后台实际验证了登录、列表、覆盖率、许可证拦截、发布失败不改变状态和公共 API 隔离。
- 已知边界：阶段 4 只完成可运营的参考案例管理与审核入口，尚未把未经人工审核的参考案例自动发布到公共目录。

### 阶段 5 实际交付：AI 案例规格生成

- 新增迁移 `infra/migrations/142_gallery_case_specs.up.sql` 和回滚文件，建立 `gallery_gap_tasks`、`gallery_case_specs`、`gallery_case_spec_versions` 三张表。
- 新增后台 API：
  - `GET /admin/api/gallery/gap-tasks`
  - `PATCH /admin/api/gallery/gap-tasks/:id/status`
  - `GET /admin/api/gallery/case-specs`
  - `GET /admin/api/gallery/case-specs/:id`
  - `GET /admin/api/gallery/case-specs/:id/versions`
  - `POST /admin/api/gallery/case-specs/import`
  - `PATCH /admin/api/gallery/case-specs/:id`
- 新增 `scripts/generate-gallery-case-specs.js` 和 `pnpm gallery:specs`，从覆盖率报告生成 13 个缺口任务、20 个结构化 `draft` 规格，记录来源报告 hash、候选模型、验收标准、成本占位和风险边界；脚本不会调用图片模型、不会写入公共图库、不会自动发布。
- 新增后台页面 `apps/admin/src/app/admin/gallery/specs/page.tsx`，支持缺口任务状态、规格状态和任务筛选、Prompt/标签/候选模型/验收标准编辑、JSON 导入、版本历史、风险提示和 `draft/ready/archived` 状态管理；导航入口已加入 `AdminShell.tsx`。
- `ready` 发布前由后端强制字段校验和风险审核校验；自动风险识别会忽略 negative prompt 和明确否定语境，避免把“不要出现品牌”误判成真实品牌风险；真正含风险信号的规格必须人工标记 `approved` 后才可 `ready`。
- 导入按 `public_id` 幂等更新；内容、状态和风险信息完全不变时重复导入不会递增版本号，版本历史保留实际变更。

阶段 5 验证结果：

- `pnpm gallery:specs`：成功生成 13 个任务、20 个规格，全部 `draft`，`media_generation_called=false`。
- `pnpm --filter @starai/admin exec tsc --noEmit`：通过。
- `pnpm build:admin`：通过；新增 `/admin/gallery/specs` 已进入生产构建。
- `go test ./internal/service ./internal/handler`：通过，包含风险识别与否定语境回归测试。
- `git diff --check`、Node 语法检查：通过。
- 迁移 `142`：已在当前 Docker PostgreSQL 实例实际应用。
- 运行态接口：13 个缺口任务、20 个规格；全部为 `draft`；重复导入两次后仍为 20 条且版本不增加；缺少标题的 `ready` 更新返回 HTTP 400 且原记录保持 `draft`。
- 运行态浏览器：已实际打开 `http://localhost:3001/admin/gallery/specs`，页面显示 13 个缺口任务、20 个规格、0 个可生成规格，并可打开详情和版本历史。

阶段 5 保留的边界：尚未生成任何图片，尚未创建媒体生成任务，尚未把规格标记为 `ready`，也没有修改公共图库内容。`ready` 只代表下一阶段可以创建生成任务，不代表生成或发布成功。

### 阶段 6 实际交付：批量媒体生成

- 新增迁移 `infra/migrations/143_gallery_generation_batches.up.sql` 和回滚文件，建立独立的 `gallery_generation_batches`、`gallery_generation_tasks` 生产账本；不复用普通用户 `tasks`、钱包冻结或 `works` 发布链路。
- 新增后台 API：批次列表、批次详情、任务列表、创建、启动、取消和失败任务重试；创建时服务端重新读取 `ready` 规格，重新校验启用图片模型/线路，并使用 `ModelService.EstimateCost` 计算站内算力预估，记录 `cost_rule_version`、实际算力和上游成本。
- 新增 `gallery:generation` Asynq 队列和 Worker 专用处理器，复用现有图片路由、重试、线路健康、上游异步轮询和对象存储能力；成功结果只写入任务 `output` 和自有资源地址，最终状态为 `pending_review`，不会创建 `gallery_items`、不会写入公共 Web 图库。
- 新增 Admin 页面 `apps/admin/src/app/admin/gallery/generation/page.tsx`，支持批次状态、进度、成本、并发/重试、任务明细、图片预览和失败重试；导航入口已加入 `AdminShell.tsx`。
- 服务端保护已验证：当前数据库没有 `ready` 规格时，创建批次返回明确错误且不创建空批次；批次任务对同一规格有活动任务唯一约束，避免重复生成；预估成本是锁定记录，不会冻结普通用户余额。

阶段 6 验证结果：

- 迁移 `143` 已在当前 Docker PostgreSQL 实例实际应用；`gallery_generation_batches`、`gallery_generation_tasks` 已存在。
- `go test ./...`：API 和 Worker 均通过；Admin TypeScript 检查和生产构建均通过；运行态 `http://localhost:8080/health` 返回 worker `ok`，Admin `/admin/gallery/generation` 与 Web `/app/gallery` 均返回 200。
- 在本地闭环测试中临时将 1 条规格设为 `ready`、将 1 条图片线路切换到仓库内 mock 图片服务，通过后台真实创建并启动批次；Worker 实际生成并转存 1 张图片，批次为 `completed`，任务为 `pending_review`，实际算力为 `1.73301120`，没有产生公共图库记录。测试批次、对象存储文件、临时线路和规格状态已全部清理恢复。
- 修复了运行态发现的可空时间字段扫描问题：批次和任务详情的 `started_at`、`finished_at`、`cancelled_at` 现在正确处理 SQL `NULL`。

阶段 6 保留的边界：

- 当前 20 条阶段 5 规格仍为 `draft`，没有任何生产批次；必须先完成人工规格审核并将目标规格置为 `ready`。
- 当前数据库中的 `gpt-image-2` 与 `seedream-5-0-lite` OpenRouter 线路仍保留空 API Key/原有线路配置，真实上游生成需要管理员在模型线路后台填写并验证密钥；本轮闭环使用的是临时 mock 线路，未消耗 OpenRouter 额度。
- `completed` 批次表示所有生成任务已进入 `pending_review`，不表示案例已经公开；公开发布属于阶段 7 的质量审核与发布流程。

### 阶段 7 实际交付：质量审核与发布

- 新增迁移 `infra/migrations/144_gallery_quality_reviews.up.sql`、`infra/migrations/145_gallery_quality_semantic_check.up.sql` 和回滚文件，建立独立的 `gallery_quality_reviews` 审核记录，并为 `gallery_items` 增加 `generation_task_id` 唯一关联；阶段 6 的生成任务仍不会直接变成公共案例。
- 新增后台 API：
  - `GET /admin/api/gallery/quality-reviews`
  - `GET /admin/api/gallery/quality-reviews/:id`
  - `POST /admin/api/gallery/quality-reviews/:id/auto-check`
  - `POST /admin/api/gallery/quality-reviews/:id/approve`
  - `POST /admin/api/gallery/quality-reviews/:id/reject`
  - `POST /admin/api/gallery/quality-reviews/:id/publish`
- Worker 在生成任务进入 `pending_review` 后幂等创建审核记录；历史待审核任务也会由列表接口补齐审核记录。
- 自动质量检查包含：HTTP(S) 图片资源可访问性、响应 MIME、图片解码、尺寸和 50 MB 大小上限、任务/规格/候选模型关联、风险复核、内容 SHA-256、Prompt SHA-256、公共图库 URL 重复和审核记录重复检测。
- 质量证据还单独记录 `semantic_check`；当前部署没有视觉语义审核模型时标记为 `unavailable/manual_required`，明确要求人工核对图片与 Prompt/标签的一致性，不把人工经验伪装成自动评分。
- OCR 结果采用诚实状态：当前部署没有 OCR 引擎时，有预期文字的案例记录为 `unavailable` 并要求人工复核；没有预期文字时记录为 `not_required`，不会伪造 OCR 通过。
- 人工审核通过后只进入 `approved`；必须再次点击明确发布，事务内幂等创建或更新 `gallery_items`，并将任务标记为 `succeeded`。重复点击不会创建多个公共案例；拒绝必须填写原因；操作均写入 `admin_operation_logs`。
- 新增后台页面 `apps/admin/src/app/admin/gallery/quality-reviews/page.tsx`，支持按审核状态筛选、检查证据详情、图片预览、自动检查、人工通过/拒绝和明确发布；导航入口已加入 `AdminShell.tsx`。

阶段 7 验证结果：

- 迁移 `144`、`145` 已在当前 Docker PostgreSQL 实例实际应用；API、Worker、Admin 镜像已重新构建并重启。
- `go test ./...`：API 和 Worker 均通过；`pnpm --filter @starai/admin exec tsc --noEmit`、`pnpm build:admin`、`git diff --check` 均通过。Admin 构建仅保留仓库原有 Hook 警告和本页原生 `<img>` 性能警告。
- 运行态 `http://localhost:8080/health` 返回 `worker_status=ok`；Admin `/admin/gallery/quality-reviews` 返回 200；Web `/app/gallery` 返回 200；API 路由日志确认审核接口已注册。
- 使用隔离临时任务走真实后台页面闭环：首次使用已存在公共案例 URL 时被重复检测拦截；更换唯一图片后资源、解码、规格、模型、风险和重复检查通过；人工审核后保持“已审核待发布”；明确发布后公共 `/api/gallery` 可查询到 `approved` 案例，数据库关联任务状态为 `succeeded`。
- 验证结束后已删除临时审核记录、生成任务、生成批次和公共案例，并将临时规格恢复为 `draft`；当前 `gallery_quality_reviews`、`pending_review` 任务和由生成任务产生的 `gallery_items` 均为 0。

阶段 7 保留的边界：

- 当前没有 OCR 引擎；包含预期图片文字的案例会阻止自动通过并要求人工复核，后续可在独立阶段接入可配置 OCR/视觉模型适配器。
- 当前没有视觉语义审核模型；语义一致性以 `unavailable/manual_required` 进入审核证据，后续接入模型后可在同一字段替换为真实检查结果。
- 当前重复检测已覆盖 URL、内容 hash、Prompt hash；感知哈希和图像语义相似度仍属于后续增强项。

### 阶段 8 实际交付：前端灵感广场体验

- `GalleryPanel.tsx` 新增当前筛选摘要：搜索词、分类、风格、场景和语言都会以可移除芯片展示；支持单项移除和一键清除全部筛选，社区作品和参考案例均保持与外层工作台筛选状态同步。
- 空状态按原因区分：筛选交集为空时显示匹配提示并提供恢复入口；参考案例公共接口没有已发布案例时显示授权核验/发布中的明确提示；加载失败仍保留独立重试状态。
- 社区作品在搜索或语言筛选时展示真实可见结果数，空白搜索不会误判为有效筛选；分页、复制 Prompt、详情弹窗和已有复用接口保持不变。
- 详情弹窗在移动端限制图片/视频媒体高度，保证标题、Prompt、复制和生成按钮处于可滚动的同一交互上下文；桌面四列、移动单列和横向标签滚动均已实测。
- 新增中、英、日、韩、越本地化文案，避免把内部筛选值直接作为用户可见文案。

阶段 8 验证结果：

- `npm run lint`：通过。
- `npm run build`：通过；仅保留仓库原有图片优化警告和 `DigitalHumanWorkspace` 的 Hook 警告，`GalleryPanel` 无新增 lint/build 警告。
- Web 运行态：重启开发服务后实际打开 `http://localhost:3000/app/gallery`；参考案例空状态、社区作品 4 条、分类筛选、单项移除、空结果恢复、详情弹窗均已通过浏览器交互验证。
- 响应式实测：默认桌面尺寸和 `390x844` 移动尺寸均可访问；移动端筛选条可横向滚动，详情弹窗媒体与操作区未发生不可恢复的布局溢出。
- 运行态接口：Web `/app/gallery`、Admin `/admin/gallery/quality-reviews`、API `/api/gallery/references` 和 `/api/gallery` 均返回 HTTP 200；`/api/gallery/references` 当前返回 0 条是发布闸门下数据库暂无已发布参考案例的真实状态，不是前端筛选误报。

阶段 8 保留的边界：

- 标签旁的“可发布案例数量”仍需要 API 返回按标签聚合的发布计数；当前前端只能可靠展示当前结果数，未用不完整的分页数据伪造标签计数。
- “相近标签/推荐案例”、图片失败回退和离线快照仍需独立的数据接口与资源策略，未在本阶段虚构推荐或缓存结果。
- Prompt 语言与图片文字语言的独立筛选需要阶段 7/后续 OCR 结果字段在公共接口中落地；当前语言筛选仍基于 Prompt/标题文本，并已在前端保持现有语义。

### 阶段 9 实际交付：测试、上线与运营手册

- 修复迁移校验基线：登记历史 130、131、132 重复编号兼容例外，补齐 130_toolbox、131_chat、132_realtime 和 134–139 的真实 checksum；未登记的新重复编号仍会失败。
- 新增 `scripts/preprod-verify.sh` 和 `pnpm preprod:verify`，使用隔离 Compose 项目覆盖依赖健康、镜像构建、迁移幂等、隔离回滚恢复、API/Worker/Admin/Web 健康和 Gallery 接口 smoke；不会触碰当前开发数据库。
- 新增 [阶段 9 运营手册](inspiration-gallery-stage9-operations.md)，包含发布顺序、缺口到发布闭环、备份恢复、应用回滚、监控告警、成本预算和已知边界。
- API 与 Worker `go test ./...`、Admin/Web lint 和 production build 均通过；构建只保留仓库已有 warning，没有新增 TypeScript 或编译 error。
- `pnpm preprod:verify` 已在本机完整实测通过：151 对迁移首次执行成功、第二次全部幂等跳过、全量 `down` 成功、再次全量 `up` 恢复成功；隔离 API/Worker/Web/Admin、指标、未登录 401、管理员和用户登录、支付关闭保护、Gallery 公共和后台契约、mock 对话均通过，临时容器、网络和卷已自动清理。
- 浏览器验收发现运行中的 `next dev` 与同目录 `next build` 共享 `.next` 会造成 CSS 404；HTTP 页面本身仍可能返回 200，因此只做 curl 不足以验收。清理生成目录并重启 Web 开发服务后，浏览器实际显示完整样式、0 条参考案例的正确发布闸门空状态、4 条社区作品，控制台无 error。
- `scripts/integration-test.sh` 已改为显式错误报告、临时验证码配置恢复、无充值卡消耗，并覆盖四组 Gallery Admin 合约；真实 OpenRouter 额度和生产密钥仍不在本地 smoke 范围内。
- 新增 `scripts/gallery-e2e.sh` 并纳入 `pnpm preprod:verify`：在可销毁隔离环境中导入唯一缺口和 `ready` 规格，创建并启动批次，由 Worker 生成并持久化图片，再验证未审核禁止发布、自动质检、空拒绝原因拦截、人工通过、明确发布、重复发布幂等，以及公共 `/api/gallery` 精确出现一条案例。
- mock 图片接口改为按 Prompt 生成确定性的本地 PNG，不再依赖 `picsum.photos`；相同输入结果稳定，不同测试输入有不同内容 hash，既可离线回归，也能真实覆盖图片下载、解码、转存和重复检测。
- 增加 `LOCAL_STORAGE_INTERNAL_URL`：质量检查仅把匹配 `LOCAL_STORAGE_PUBLIC_URL` 的自有资源改写到容器内地址，数据库与前端仍保留公开 URL；纯函数单元测试覆盖正常改写、外部域名、相邻路径和未配置内部地址。
- 2026-10-02 最终隔离端到端验收通过，测试生成、审核并发布 1 条临时公共案例，随后完整销毁隔离容器、网络、数据库卷和临时上传目录；这是对阶段 9“至少一条从缺口到发布”验收标准的自动化回归，不会向当前开发数据库或正式 Gallery 发布内容。

阶段 9 保留的边界：

- 当前环境未自动发布任何新增参考案例；发布仍需管理员完成许可证、质量和明确发布操作。
- 生产环境应用回滚默认不反向回滚数据库迁移；数据库恢复必须依赖已验证的完整备份和维护窗口。
- 端到端的真实媒体生成需要管理员填入 OpenRouter API Key 并完成线路验证，本阶段不消耗真实上游额度。

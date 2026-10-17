# 「电商图片工厂」复刻开发计划

版本：v1.0（2026-10-16，已批准）
依据：`docs/ecom-factory-spec.md`（qqwapi.cn 登录实测 + 前端产物逆向的完整规格）。
决策：新建独立智能体（agent_mode `ecom_factory`）· 服务端工作流架构 · 提示词模板原样复刻 · 最小验证模型集（gpt-image-2 + glm 系）· Orbit-E 品牌 UI 重设计。

## 1. 功能拆解

| # | 功能 | 级别 |
|---|---|---|
| F1 | 商品资产上传（≤8）+ 风格参考（≤6）：点击/拖拽、预览、状态、移除 | P0 |
| F2 | 商品信息 4 字段（空值落兜底文案） | P0 |
| F3 | 生成参数：模型/平台/比例/清晰度/每类张数/语言 | P0 |
| F4 | 套装切换（5 预设）+ 类别多选（14 模块自由组合） | P0 |
| F5 | 生成计划：≤24 卡片，本地即时拼装完整提示词并可复制（免费） | P0 |
| F6 | 批量生成：并发 8 逐项提交-轮询，状态机 + 进度文案 | P0 |
| F7 | 单卡操作：重新生成（可改提示词）/ 精修（图生图）/ 下载 | P0 |
| F8 | 停止队列 / 继续未完成 | P1 |
| F9 | 批量模式面板（方式/相似度/强度/保持/改变/数量） | P1 |
| F10 | 反推提示词（免费+频控） | P1 |
| F11 | 快速生图（直输/反推提示词生成 1 张） | P1 |
| F12 | 打包下载 ZIP（后端组包） | P1 |
| F13 | 历史记录：全量恢复 + 进行中自动续跑 + 逐条删除 | P1 |
| F14 | 视觉风格 8 档 + 文案开关 + 补充要求 | P0 |
| F15 | 统计卡 / 余额与计费提示 | P2 |
| F16 | 写作品库；导出入口（视频工厂 → 查看作品库） | P1 |

明确不做：详情长图拼接、支付/会员体系、200 张超计划并发（对齐 24 cap）。

## 2. 系统架构

```
apps/web (Next.js)
  EcomFactoryWorkspace.tsx ── GET /api/agents/ecom_factory/catalog（模块/套装/模板同源下发）
        │  计划预览：前端用同源 catalog 本地即时拼提示词（复制免费）
        ├─ POST /api/assets/upload（已有）
        ├─ POST /api/agents/ecom_factory/projects（表单快照+计划）→ 预估冻结 → workflow_projects → asynq
        ├─ GET  /api/agent-projects/:id（轮询 items[].status/image_url/progress）
        ├─ POST /api/agent-projects/:id/items/:itemId/regenerate {prompt?}
        ├─ POST /api/agent-projects/:id/items/:itemId/refine {instruction}
        ├─ POST /api/agent-projects/:id/cancel（已有）
        ├─ POST /api/agents/ecom_factory/reverse {image_url}
        ├─ POST /api/agents/ecom_factory/quick {prompt,...}
        └─ GET  /api/agent-projects/:id/export（流式 ZIP）

services/api (Go)
  service/agent.go CreateProject 加 ecom_factory 分支（校验/默认值/计价冻结）
  agent_ecom_factory.go：catalog 下发、reverse、quick、export、items 级重入

services/worker (Go)
  workflow.go dispatch: agent_mode=='ecom_factory' → ecom_factory.go
  processEcomFactoryWorkflow：服务端重建计划（防篡改）→ 并发池 min(8,N)：
    buildPrompt（模板）→ resolveSize → imageMode → processImageTask（幂等 task_no）
    → 增量写 outputs.items → works 表 → 按成功张数结算
```

模板同源：模块目录 + 4 套模板 = 单一 embed JSON（数据驱动行模板 + 极简条件 DSL），catalog 接口下发同一份；Go/TS 各实现 ~50 行渲染引擎；对拍测试保证与参考站逐字一致。

## 3. 模块划分与关键文件

| 层 | 文件 | 内容 |
|---|---|---|
| DB | `infra/migrations/141_ecom_factory.up.sql` | workflow_definitions 新行 + toolbox 目录条目 |
| Worker | `services/worker/cmd/worker/ecom_factory.go`；`services/worker/internal/ecomfactory/{catalog.go, catalog.json, prompt.go, plan.go}` | 处理器 + 目录 + 模板 + 计划 |
| API | `services/api/internal/service/agent_ecom_factory.go`；handler 路由 | 校验/计价/catalog/reverse/quick/export/regenerate/refine |
| Web | `apps/web/src/components/workbench/EcomFactoryWorkspace.tsx` + 子组件 | AgentWorkspace 按 agent_mode 分流 |
| Admin | `apps/admin/src/app/admin/agents/page.tsx` | agent_mode 联合类型补 ecom_factory |

## 4. 数据流（主链路）

1. 表单 → 前端本地拼计划卡片（即时复制）→ 生成 N 张
2. POST projects：校验（普通：图∨名称∨卖点 ∧ 模块；batch：提示词；白名单；cap 24）→ 冻结（张数×单价，1K/2K 分档）→ 落库（inputs=表单快照）→ 入队
3. Worker：重建计划 → 并发生图 → 每项完成即写 outputs → 结束写 works、按成功结算、completed（失败项标 failed 不阻塞）
4. 单卡精修/重生成 → items 级任务 → 更新该项
5. 历史恢复：项目列表 → inputs 回填表单 + outputs 回填结果；运行中自动续轮询

## 5. 开发阶段与优先级

| 阶段 | 内容 | 预估 |
|---|---|---|
| M0 | 规格文档落盘 + catalog JSON + migration | 0.5 天 |
| M1 核心链路 | catalog 接口、CreateProject 校验计价、worker 处理器（普通模式）、Workspace 骨架 | 2-3 天 |
| M2 完整功能 | 批量面板、单卡重生成/精修、停止/继续、下载、ZIP | 2 天 |
| M3 体验对齐 | 反推+快速生图、历史恢复+自动续跑、作品库、统计卡 | 1.5 天 |
| M4 打磨 | Orbit-E UI、移动端、错误文案、i18n、模板对拍 5/5 | 1-2 天 |

## 6. 关键技术难点与风险

1. 模板同源防漂移：单一 embed JSON + 数据驱动行模板 + 对拍测试（与参考站剪贴板实测文本 diff）
2. items 级重入（已完成项目上单卡重跑/精修）：item-scoped 任务 + 幂等 task_no 防重复扣费
3. 部分失败计费：按实际成功张数 FreezeWithFinalize；重生成重新计费（对齐参考站）
4. cancel 协作：已提交生图等待自然完成，未开始项 stopped（ctx + 项目状态双通道）
5. 精修图生图：gpt-image-2 images/edits，自身图为唯一参考；需验证保真度
6. 反推滥用：频控 + 图片大小限制
7. 成本：24 张单次最高 ~$14 → 提交前预估展示 + 预算上限
8. 4K 档：gpt-image-2 仅 1K/2K 已验证，按模型能力隐藏/透传

## 7. 验收标准

- E2E：上新快发 5 张、详情页套装 5 张、批量 10 张、精修/重生成、反推、ZIP、历史恢复+续跑、停止/继续
- 提示词与参考站逐字一致（模板 diff）
- 计费：部分失败只结成功张数；重试不重复扣费
- UI：Orbit-E 品牌体系，功能入口一一对应

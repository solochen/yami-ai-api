# 角色替换智能体（角色替换台）

> 状态：实时换角已实现，真实上游 Key 的端到端验收待补。
> 对外定位：开放视频模型内核 + 自研实时工程链路 + 项目开源。
> 展示层约束：不出现上游品牌、产品名、功能代号与内部协议字段。见「品牌隔离」。

## 1. 产品流程

选角 → 排练 → 开播 → 热切换 → 抓取片段 → 收工结算。

- **选角**：角色卡（参考形象）保存在个人素材库，最多 30 张。
- **排练**：摄像头预览与开机检查，不建立会话、不扣费；预览可随时关闭并释放摄像头。
- **开播**：两阶段会话（先返回 `starting`，后台完成创建与就绪握手），可取消。
- **热切换**：直播中点击角色卡下发切换指令；成功无回执，失败异步回传。
- **抓取**：浏览器端录制合成画面，默认收录麦克风原声，片段自动进入「我的作品」。
- **收工**：挂断、查询上游用量、按实际秒数结算，展示本次总结。

页面不是聊天形态：横向角色架 + 监看台 + 信号面板 + 镜头带。快捷键 `1–9` 换角、`R` 抓取、`Esc` 结束会话；页头与画面右下角也都有「结束会话」按钮，点击打开确认弹窗。

## 2. 代码位置

| 层 | 文件 |
| --- | --- |
| 页面 | `apps/web/src/components/workbench/CharacterStudioWorkspace.tsx` |
| 前端接口 | `apps/web/src/lib/characterStudio.ts` |
| 实时通道适配 | `apps/web/src/lib/realtime/renderStage.ts` |
| 本地演练适配（仅开发） | `apps/web/src/lib/realtime/renderStageMock.ts` |
| 后端服务 | `services/api/internal/service/character_studio.go` |
| 后端路由 | `services/api/internal/handler/character_studio.go` |
| 迁移 | `infra/migrations/217_character_studio.up.sql` |
| 英文文案 | `apps/web/src/i18n/source-locales/supplements/en-US/characterStudio.ts` |
| 品牌隔离测试 | `apps/web/src/components/workbench/characterStudioBranding.test.mjs` |

## 3. 数据模型

- `character_studio_characters`：角色卡，含 `rights_confirmed_at`（上传时授权确认）。
- `character_studio_sessions`：会话元信息、上游 `live_id` / `conn_id` / `render_uid`、通道凭证、状态、切换次数、计费字段。
- `character_studio_takes`：片段索引，实际文件走素材库（`/api/assets/upload`）。

不保存原始视频流；会话凭证存 `rtc_payload`，仅服务端使用。

## 4. 接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/character-studio/config` | 售价、最长时长、清晰度、源码地址 |
| GET/POST | `/api/character-studio/characters` | 角色卡列表/新建 |
| PUT/DELETE | `/api/character-studio/characters/:id` | 改名/换图、删除 |
| POST | `/api/character-studio/sessions` | 创建会话，立即返回 `starting` |
| GET | `/api/character-studio/sessions/active` | 未关闭会话（刷新恢复） |
| GET | `/api/character-studio/sessions/:id` | 轮询状态；`live` 后返回通道凭证与 `output_channel` |
| POST | `/api/character-studio/sessions/:id/switch` | 热切换（不等待成功回执） |
| POST | `/api/character-studio/sessions/:id/heartbeat` | 心跳，10 秒/次 |
| POST | `/api/character-studio/sessions/:id/end` | 挂断 + 用量查询 + 结算 |
| POST | `/api/character-studio/sessions/:id/takes` | 片段登记 |
| GET | `/api/realtime/engine.js` | 浏览器运行时中性代理（公开、缓存 6 小时） |

## 5. 会话状态与可靠性

```
starting → live ⇄ reconnecting
    │         │
    ├→ failed └→ wrapping → ended
```

- **就绪握手**：建立 WebSocket 后立即发送初始化信令；`NOT_READY` 按 2 秒重发，整体 75 秒超时。
- **启动取消**：`end` 在 `starting` 阶段会取消后台创建、解冻额度、标记结束，不扣费。
- **断线重连**：控制连接断开后复用同一 `live_id` / `conn_id` 重连一次并重发初始化信令；失败则挂断结算。
- **服务端兜底**：启动 90 秒超时、心跳丢失 75 秒、单场最长时长（默认 10 分钟）任一触发自动结束。
- **刷新恢复**：`sessions/active` 返回未关闭会话，页面弹窗让用户选择恢复监看或结束结算。
- **片段上限**：单场最多 12 段（前端限制 6 段），每段最长 60 秒。

## 6. 计费

- 上游官方口径 1 积分/秒，积分单价复用 `vidu_text_credit_rate`（默认 0.03125 元）；分辨率不分档，540p/720p/1080p 同一秒单价。2026-11-13 实测：1080p 27 秒 = 27 积分，与 540p 一致（1 积分/秒）。
- 用户侧售价 `character_studio_sale_price_per_minute`，默认 2.49 算力/分钟。
- 开播前冻结 1 分钟额度；结束按上游 `billed_seconds` 结算；查询失败时按本地计时兜底。
- 费用计算：`billed_seconds / 60 × sale_price_per_minute`，向上保留 6 位小数。
- 交易类型 `video_usage`，备注「角色替换」。

配置键（后台「角色替换（实时渲染）」）：

- `character_studio_sale_price_per_minute`
- `character_studio_credit_per_second`
- `character_studio_max_minutes`
- `character_studio_default_resolution`
- `character_studio_engine_url`
- `character_studio_source_url`（预留，当前页面不展示）

API Key 与接口域名复用「实时数字人」的 `vidu_api_key` / `vidu_api_host`。

**默认角色**：`apps/web/public/character-studio/defaults/` 下 10 张白底静态图，前端「默认角色 / 我的角色」tab 展示；点击默认角色会把图片上传到当前用户素材库、创建角色卡并选中（已存在则直接选中），直播中同样走热切换。图片由 Vidu `viduq2` 生成（8 积分/张）；动物/卡通的换角效果受上游主体识别限制。

**默认角色的公网问题**：静态图本身由 web 站点提供，不依赖对象存储；真实开播时上游拉取的是点击后复制到用户素材库的**副本 URL**。本地 MinIO 的 `MINIO_PUBLIC_URL=/media` 是相对地址，上游访问不到，所以本地只能验证展示/选择/演练，真实开播需公网存储或隧道；生产 COS 环境首次点击会把副本传到 COS，返回 `https://img...` 绝对地址，不需要预先手动上传 10 张图。若服务端仍返回 `/media/...`，检查 `MINIO_PUBLIC_URL` / `storage_public_url`，而不是默认角色代码。

**片段文件位置**：视频字节不落在项目目录或数据库里，而是对象存储。数据库只保存 `assets.bucket` / `assets.object_key`、`character_studio_takes.url` 和 `works.metadata.video_url`。本地开发使用 Docker 里的 MinIO（bucket `starai-works`，key `assets/<user_id>/<asset_id>/<filename>`，公开 URL 由 `/media/...` 代理）；生产按 `MINIO_*` 配置写入 COS/R2/MinIO，`MINIO_PUBLIC_URL` 是 `img` 这类公网 CDN 域名，对象 key 不变。作品过期清理会同时删除 works、assets 记录和对象存储文件。

**片段与作品**：抓取片段会先上传到 assets，再登记 `character_studio_takes`；登记成功后由 API 同步创建 `works` 记录（`metadata.source=character_studio`），因此会出现在「我的作品」。迁移 219 会为历史 take 补建作品记录。片段仍受作品保留天数限制，需要长期保存请下载。

**实时通道接入**：页面启动时会预热浏览器运行时；开播后先释放排练摄像头，再以 authInfo 字符串入会（对象参数仅作兼容回退）。接入失败会用全新实例重试一次，发布视频失败会短暂等待后重试；仍然失败时不自动结束会话，而是显示「连接没有建立」并提供「重新接入 / 结束会话」。RTC 通道断开同样进入该状态，避免把可重试的网络抖动直接变成扣费会话结束。

> 本地联调注意：dev server 的 HMR 刷新会触发页面的 pagehide 结束逻辑。测试实时通道时先刷新到稳定版本，测试过程中不要改前端文件；需要稳定复测时使用生产构建。

**参考图可达性**：角色卡图片支持两种来源——本地上传（进入素材库）或**公网链接**（直接保存绝对 URL，编辑器内实时预览）。上传后如果是 `/media/...` 相对地址，会先按 `vidu_public_base_url` 补成绝对地址再提交；补全后仍不可达时开播会被上游拒绝。

- 本地开发：`MINIO_PUBLIC_URL=/media`，上传返回相对地址，上游无法访问，应改用「公网链接」或配置可公网访问的存储域名。
- 生产 COS/CDN：`MINIO_PUBLIC_URL` 必须是绝对公网域名（如 `https://img.yourdomain.com`），对象可被公开 GET；不要使用会过期的签名 URL，也不要开启会拦截非浏览器请求的防盗链。

## 7. 运行时白标

- 浏览器请求本站 `/api/realtime/engine.js`，服务端代理并缓存第三方运行时，页面 DOM 不出现上游 CDN 域名。
- `renderStage.ts` 是唯一绑定第三方运行时全局对象的位置；页面、接口、文案文件不引用。
- 边界：网络面板、脚本内容与媒体连接仍可能被技术人员识别；彻底匿名需要自建渲染，不在本期。
- 若上游条款不允许代理，可将 `character_studio_engine_url` 配置为直连地址，展示层仍无品牌。

## 8. 品牌隔离

**页面禁止出现**：上游品牌/产品名、`S2` / `S2-Editing` / `S2-Avatar` 等代号、`subject_replacement`、`live_id`、`render_uid`、`client_secret`、`conn_init`、`switch_prompt`、`s_editing`、供应商 RTC 名称。

**统一用词**：实时渲染服务 / 实时通道 / 渲染节点 / 角色替换 / 合成画面 / 身份锚定 / 时域稳定。

新页面、接口字段与英文文案由 `characterStudioBranding.test.mjs` 扫描；后端用户可见错误由 `character_studio_test.go` 扫描。

## 9. 本地演练（开发）

- 访问 `/app/agents/character_studio?rehearse=1`，仅在 `NODE_ENV !== "production"` 生效。
- 不建立真实会话、不连接实时通道、不产生费用；用画布把摄像头画面做风格化处理，模拟合成画面。
- 用于验证完整交互、切换、倒计时、录制、收工总结与响应式布局。
- 生产构建不会暴露入口；真实通道仍需上游 Key。

## 10. 验收清单

- [x] 迁移与 checksum
- [x] Go 单元测试（协议报文、分辨率、费用、错误脱敏、字段名）
- [x] 前端类型检查、lint、品牌隔离测试
- [x] 开发环境演练全流程与 1440 / 1024 / 390 截图
- [x] 后端启动失败路径：冻结 → 上游拒绝 → 解冻，余额不变
- [x] 未关闭会话恢复/结束
- [ ] 真实 Key 端到端：首帧时延、热切换生效、挂断结算、连续开播、断线重连
- [ ] 上游允许代理运行时的条款确认
- [ ] 服务重启时进行中会话的冻结额度对账（建议后续补运营兜底）

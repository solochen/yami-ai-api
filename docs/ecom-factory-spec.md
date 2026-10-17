# 电商图片工厂（Ecom Factory）功能规格书

版本：v1.0（2026-10-16）
来源：qqwapi.cn「电商图片工厂」（toolKey `ecom-product-image`）登录态实测 + 前端产物逆向，规格为**逐字提取**，作为复刻实现与验收的唯一依据。
计费原则：**反推与规划免费；生成图片按所选图片模型实际扣费**。

---

## 1. 页面结构（三栏工作台）

| 区域 | 内容 |
|---|---|
| 顶栏 | 面包屑（平台/工具箱）、深浅色切换、公告、专属客服、返回工具箱 |
| 工具头 | 名称「电商图片工厂」+ 副标题 `Ecommerce Image Lab` + 能力标签「商品图 · 详情页 · 种草图 · 风格复刻 · 反推提示词」+ 主按钮 `生成 N 张`（生成中 → `停止队列`；存在失败/未完成项 → `继续未完成`） |
| 左栏 | 商品资产上传（0/8）、风格参考上传（0/6）、商品信息 4 字段 |
| 中栏 | 生成参数（模型/平台/比例/清晰度/每类张数/语言）→ 套装条 → 类别多选片（batch 套装时切换为批量面板）→ 视觉风格/文案开关/补充要求 → 生成计划（提示词卡片）→ 出图结果 |
| 右栏 | 提示词快速生图（输入或反推）、导出与流转（打包下载/查看作品库）、历史记录、统计卡（参考图/计划/完成） |

batch 套装激活时文案变化：左栏「商品资产」→「主参考图」（副文案「主体、构图、原始画面」）、「风格参考」→「辅助参考图」（「风格、背景、材质、局部元素」）。

## 2. 输入项

### 2.1 商品资产（role=product，≤8 张）
主体图、细节图、包装图；点击/拖拽上传；卡片含缩略图、文件名、状态（本地/上传中/已上传/失败）+ 移除按钮。

### 2.2 风格参考（role=reference，≤6 张）
构图、光影、氛围、爆款样式；交互同上。

### 2.3 商品信息（全部可选）
| 字段 | 占位符 | 空值兜底（落入提示词） |
|---|---|---|
| 商品名称 productName | 例：便携咖啡机 | `上传参考图中的商品` |
| 核心卖点 sellingPoints（多行） | 材质、功能、痛点、优惠、规格 | `请从商品外观和使用场景中提炼核心卖点` |
| 目标人群 audience | 例：办公室白领、露营用户 | `目标电商用户` |
| 使用场景 scenario | 例：清晨厨房、户外露营、直播间 | `真实购买与使用场景` |

### 2.4 生成参数
- **图片模型**：模型目录接口下发（别名 + effectiveCapabilities：supportedRatios / supportedResolutions / maxReferenceImages），能力动态约束比例/清晰度选项；无能力信息时回退 `1:1/3:4/4:3/9:16/16:9` + `1K/2K`
- **平台 platform**（7）：淘宝/天猫、京东、拼多多、抖音电商、小红书、亚马逊、独立站（仅写入提示词）
- **比例 ratio**（5）：16:9、4:3、1:1、3:4、9:16；未选时用模块默认比例；模型不支持时回退 1:1 或第一档
- **清晰度 resolution**：1K、2K、4K → 尺寸映射见 §6
- **每类张数 countPerModule**：1–3（batch 模式替换为批量张数 batchCount：输入 1–200）
- **语言 language**（4）：简体中文、英文、中英双语、无文字

### 2.5 套装（单选；切换即重置类别选择）
| id | label | 描述 | moduleIds |
|---|---|---|---|
| launch | 上新快发 | 主图、白底、场景、详情和种草一次配齐。 | main-square, white-bg, scene-hero, detail-cover, seed-cover |
| detail | 详情页套装 | 详情首屏、材质、卖点、对比和场景承接转化。 | detail-cover, texture-close, selling-grid, compare, scene-hero |
| social | 种草内容 | 面向小红书、抖音和直播间的内容图。 | seed-cover, lifestyle, live-sticker, scene-hero |
| clone | 风格复刻 | 参考爆款样图生成同调性的商品系列图。 | style-clone, color-series, main-square, shop-banner |
| batch | 参考图批量生成 | 多参考图加自定义指令，批量生成改款、融合和营销图。 | reference-batch |

### 2.6 类别模块目录（14，多选自由组合）
| id | label | shortLabel | channel（用途） | defaultRatio | goal（商业目标） | prompt（画面任务） | accent |
|---|---|---|---|---|---|---|---|
| main-square | 平台主图 | 主图 | 淘宝/天猫/京东/拼多多 | 1:1 | 清晰展示商品主体，适合搜索列表第一张图。 | 生成电商平台首图。商品主体居中，轮廓完整，材质真实，画面干净，留出适度呼吸空间。构图要适合移动端搜索列表缩略图，第一眼能识别品类和卖点。 | #0ea5e9 |
| white-bg | 白底质检图 | 白底 | 平台审核/货架图 | 1:1 | 保留商品外观，用纯净白底提升审核通过率。 | 生成标准电商白底图。纯白或近白背景，商品边缘干净，阴影轻微自然，不添加装饰物，不改变商品结构、颜色、品牌标识和材质。 | #64748b |
| scene-hero | 场景卖点图 | 场景 | 详情首屏/广告图 | 4:3 | 用真实使用场景承接核心卖点。 | 生成商业场景卖点图。围绕目标用户的真实使用场景设计环境，商品保持主角位置，加入合理道具和光影，画面有生活感但不过度杂乱。 | #10b981 |
| detail-cover | 详情页首屏 | 详情 | 详情页 | 3:4 | 首屏完成商品、利益点和质感表达。 | 生成电商详情页首屏图。画面纵向排版，商品主体和关键卖点层次清楚，有高级商业海报质感，适合放在详情页顶部承接转化。 | #f59e0b |
| texture-close | 材质细节图 | 细节 | 详情模块 | 4:3 | 突出材质、工艺、纹理、接口或细节优势。 | 生成商品材质细节图。使用近景或微距视角，突出纹理、工艺、接口、结构或手感，保留真实比例和材质，背景简洁，光线能展示细节层次。 | #8b5cf6 |
| selling-grid | 卖点模块图 | 卖点 | 详情模块 | 3:4 | 把多个卖点整理成电商详情模块。 | 生成卖点模块图。使用清晰的信息层级表达 2 到 4 个核心卖点，商品图、图标式视觉元素和留白排版协调，适合详情页连续阅读。 | #ef4444 |
| compare | 对比说明图 | 对比 | 详情模块/广告 | 4:3 | 表达使用前后、旧款新款或竞品差异。 | 生成对比说明图。采用左右或上下对比构图，突出使用前后、升级前后或不同方案的差异。画面要直观克制，避免虚假夸张。 | #14b8a6 |
| seed-cover | 种草封面 | 种草 | 小红书/抖音封面 | 3:4 | 让商品像真实用户内容一样有点击欲。 | 生成社媒种草封面。画面真实自然，有轻内容感和分享感，商品融入生活场景，构图适合小红书或短视频封面，避免硬广堆砌。 | #ec4899 |
| lifestyle | 生活方式图 | 生活 | 种草/品牌内容 | 4:3 | 塑造品牌调性和目标人群的生活画面。 | 生成生活方式广告图。以目标用户的日常空间、动作或情绪为核心，商品自然出现，整体像高质量品牌内容，真实、干净、有温度。 | #22c55e |
| live-sticker | 直播贴片 | 贴片 | 直播间/短视频挂件 | 9:16 | 为直播间生成可叠加的商品利益点画面。 | 生成直播间商品贴片图。竖屏构图，商品和利益点醒目，留出主播区域和按钮区域，视觉强但不遮挡主体，适合直播间或短视频叠加。 | #f97316 |
| style-clone | 风格复刻图 | 复刻 | 爆款复刻/系列图 | 1:1 | 参考样图的构图、光影、色调和电商氛围。 | 生成风格复刻图。参考上传样图的构图、光影、色彩、布景和商业质感，但主体必须替换为当前商品，不复制原图文字、水印、品牌或人物身份。 | #6366f1 |
| color-series | 系列配色图 | 系列 | 店铺系列/多 SKU | 1:1 | 让同一商品形成统一视觉系列。 | 生成系列配色图。保持商品主体、机位和排版一致，调整背景色、道具和光影形成同风格系列，适合店铺 SKU 展示和批量上新。 | #06b6d4 |
| shop-banner | 店铺横幅 | 横幅 | 店铺首页/活动页 | 16:9 | 生成活动、上新或品牌专区横幅。 | 生成电商店铺横幅。横向构图，商品主体清晰，左右留出运营文案空间，画面适合店铺首页、活动页或广告 banner。 | #84cc16 |
| reference-batch | 参考图批量生成 | 批量 | 多参考图/批量改款 | 1:1 | 根据上传参考图和用户指令，批量生成稳定可控的电商图片。 | 生成参考图驱动的批量电商图片。以用户上传图片作为主体、构图、风格或局部元素参考，并严格遵循用户提示词中指定的保持项、改变项和相似度要求。 | #0f766e |

### 2.7 批量面板（batch 套装专属）
- 生成方式 batchStrategy（4，单选）：保持主体改局部、参考风格重绘、多图融合生成、按提示词自由扩展
- 参考相似度 batchSimilarity（4）：30%、50%、70%、90%
- 变化强度 batchVariation（3）：低变化、中变化、高变化
- 用户提示词 batchPrompt：必填
- 必须保持 batchKeepKeys（多选，默认 主体/构图/光影）：subject 主体、composition 构图、lighting 光影、background 背景、material 材质、color 颜色
- 允许改变 batchChangeKeys（多选，默认 局部图案/风格）：local-pattern 局部图案、background 背景、props 道具、style 风格、copy 文案、scene 场景
- 快速数量（档位）：5 / 10 / 20 / 50 / 100 / 200（输入上限 200；实际一次执行计划 cap 24）

### 2.8 风格与约束
- 视觉风格 visualStyle（8）：高端商业摄影、干净白底货架、真实生活方式、小红书种草、直播间强转化、极简科技感、温暖家居感、自定义（自定义 → customStyle 文本，生效值取其文本）
- 允许生成少量运营文案 copyEnabled（checkbox；与 language 联动，见模板第 12 行）
- 补充要求 extraPrompt（自由文本，非空时追加模板末行）

## 3. 处理流程

### 3.1 计划生成（免费、即时）
- 普通：所选模块 × countPerModule（clamp 1–3）→ 计划项 ≤24；label 在 count>1 时为 `{label} {n}`；ratio = 所选比例 ∥ 模块默认比例（需模型支持，否则回退）
- batch：N 个「参考批量 001…N」（padStart(3,'0')），计划 cap 24
- 每张计划卡片**即时拼装完整提示词并可复制**（本地渲染，零成本）

### 3.2 生成（收费）
1. 校验：模型已选；普通模式需（资产>0 ∨ productName ∨ sellingPoints）∧ 已选模块；batch 模式需 batchPrompt 非空
2. 上传本地图片 → 对象存储 URL；参考图集合 = product + reference 合集，cap `min(maxReferenceImages(默认8), 8)`
3. 并发 worker 池 `min(8, N)`，逐项：提交生图 → 返回 workId → 轮询（前 2 次 1.8s、之后 5s；404×5 → 失败；超时 → 提示稍后查看）→ 状态落卡
4. imageMode：0 参考图 → `text-to-image`；1 → `image-to-image`；>1 → `multi-image`
5. 状态机：`queued(排队中) → generating(提交生成/已提交 workId/生成中 n) → success(完成) | failed(失败)`；停止 → 未开始项 `stopped(已停止)`
6. 期间持续写历史；全部结束后统一刷新图片直链；成功项入作品库

### 3.3 单卡操作
- **重新生成**：打开编辑器预填当前提示词（可改）→ 以新提示词重跑该卡（重新计费）
- **精修**：编辑器输入调整要求（默认 `整体质感、清晰度与细节优化`）→ prompt = 精修前缀 + 要求；以该卡自身图为唯一参考图，image-to-image（重新计费）
- **下载**：`{productName|电商图片}-{label}.jpg`（文件名清理 `[\\/:*?"<>|]→_`、空白→`-`、cap 48 字符）

### 3.4 反推提示词（免费）
上传图片 → base64 → 视觉模型 + 固定反推指令 → 结果回填可编辑文本框。

### 3.5 快速生图
右栏文本框直输提示词（或反推所得）→ 生成 1 张（label=快速生图；参考图 ≤3；moduleId 取第一选中模块 ∥ main-square）。

### 3.6 导出与流转
- **打包下载**：先刷新直链 → ZIP 命名 `{productName|电商图片工厂}-电商套图.zip`，每张 `{label}-{itemId}.jpg`
- **查看作品库**（参考站为「送入AI视频工厂」纯路由跳转；本仓库无视频工厂，替换为作品库入口）

### 3.7 历史与断点恢复
- 历史卡：`{productName|套装label} · {套装label} · {n}/{m} 张完成` + `{状态} · {平台} · {清晰度}`；逐条删除
- 点击 → 拉取 payload → **全量恢复**表单/图片/计划/结果；generating 且有 workId 的项自动恢复轮询
- 进入页面时：当前为空 且 存在「进行中」历史 → 自动恢复
- 「继续未完成」：重跑全部非 success 项

## 4. 提示词模板（逐字，四套）

变量：`{productName}{sellingPoints}{audience}{scenario}{platform}{visualStyle}{language}{extraPrompt}`、模块 `{channel}{label}{prompt}{goal}`、批次 `{n}`。空行过滤，`\n` 连接。

### 4.1 普通模式（Gl）
```
你是资深电商视觉设计师，请生成一张可直接用于{channel}的{label}。
商品：{productName || 上传参考图中的商品}
核心卖点：{sellingPoints || 请从商品外观和使用场景中提炼核心卖点}
目标人群：{audience || 目标电商用户}
使用场景：{scenario || 真实购买与使用场景}
目标平台：{platform}
视觉风格：{visualStyle生效值 || 高端商业摄影}
画面任务：{module.prompt}
商业目标：{module.goal}
版本方向：第 {n} 张，构图和道具可以有轻微差异，但商品身份必须一致。
{有资产}严格参考已上传图片中的商品身份、外形、比例、材质、颜色和关键结构；参考图只用于构图、光影、风格和场景方向，不复制水印、人物身份或第三方品牌。
{无资产}无参考图时，根据商品信息生成可信的商业摄影画面，商品结构要合理，不要出现不可能的材质和形态。
{文案允许：language≠无文字 且 copyEnabled}可以加入少量{language}短文案，文案必须清晰、无错别字、少而有力；不确定的文字区域请留白。
{文案禁止}画面中不要新增文字、数字、二维码、水印和无关 LOGO；若参考图已有商品标识，只能在商品本体上自然保留。
输出要求：真实商业摄影质感，主体清楚，边缘干净，光影自然，材质可信，构图适合移动端电商浏览。
禁止：低清晰度、变形、错位手指、混乱文字、错误商标、过度磨皮、重复主体、拼接痕迹、侵权品牌元素。
{extraPrompt 非空}补充要求：{extraPrompt}
```

### 4.2 批量模式（Xl）
```
你是资深电商视觉设计师，请根据用户上传的一张或多张参考图，生成新的电商图片。
参考图使用规则：第一张上传图作为主要视觉参考，其余图片作为风格、构图、背景、材质或局部元素参考；不得直接复制参考图中的水印、角标、第三方品牌、人物身份或受保护图案。
{可选}商品/项目：{productName}
{可选}商业卖点：{sellingPoints}
{可选}目标人群：{audience}
{可选}使用场景：{scenario}
目标平台：{platform}
视觉风格：{visualStyle生效值 || 高端商业摄影}
生成方式：{batchStrategy}
目标相似度：约 {batchSimilarity}。保留商业属性和核心方向，但必须通过新的构图、细节、元素组织或局部设计形成新图，避免直接复制。
变化强度：{batchVariation}
必须尽量保持：{batchKeepKeys labels 顿号连接；空 → 按用户提示词判断}
允许重点改变：{batchChangeKeys labels}。如果用户要求只改变局部元素，例如印花、标签、杯身图案、包装图案、背景道具或局部装饰，则必须把变化限制在该局部区域，其他未指定区域尽量保持不变。
优先级规则：用户指令优先于本工具的通用框架。若用户指令与生成方式、相似度、变化强度、保持项、改变项或补充要求存在冲突，请以用户指令为准；只有在用户没有明确说明时，才使用本工具的默认框架约束。
用户指令：{batchPrompt}
批量版本：第 {n} 张。每张都要在细节、元素组合、局部设计、构图节奏或风格表达上形成可见差异，避免与同批结果重复。
{文案允许}如确实需要文案，仅使用少量{language}短文案，文字必须清晰、准确、少而有力。
{文案禁止}画面中不要新增文字、数字、二维码、水印和无关 LOGO；参考图里的水印、角标、平台标识和第三方品牌不要复制。
输出要求：真实商业摄影或高质量电商视觉质感，主体清楚，边缘干净，光影自然，材质可信，构图适合移动端电商浏览。
禁止：低清晰度、畸形、拼接痕迹、错位结构、混乱文字、错误商标、二维码、水印、满屏无关装饰、侵犯第三方品牌或直接照搬参考图。
{extraPrompt 非空}补充要求：{extraPrompt}
```

### 4.3 精修（On 前缀）
```
以参考图为基准进行精修，严格保持商品的外形、比例、材质、颜色、品牌标识与整体构图一致，仅按以下要求调整，其余部分保持原样：{用户要求 || 整体质感、清晰度与细节优化}
```

### 4.4 反推指令（An）
```
请把这张图片反推成一段可直接用于电商商品图 AI 生成的中文提示词，覆盖：商品主体与品类、材质质感、构图与机位、光影与氛围、背景与场景、色调与风格。只输出一段可直接使用的提示词，不要解释、不要分点、不要加引号。
```

## 5. 常量与上限

| 常量 | 值 |
|---|---|
| 商品图上限 | 8 |
| 风格参考上限 | 6 |
| 计划上限 | 24 |
| 每类张数 | 1–3 |
| 并发 worker | 8 |
| 历史条数 | 200 |
| 批量数量输入上限 | 200（快速档 5/10/20/50/100/200） |
| 快速生图参考图 | ≤3 |
| 生图参考图 cap | min(model.maxReferenceImages ∥ 8, 8) |

## 6. 尺寸映射（resolution × ratio → size）

| ratio | 1K | 2K |
|---|---|---|
| 1:1 | 1024x1024 | 2048x2048 |
| 16:9 | 1536x864 | 2048x1152 |
| 9:16 | 864x1536 | 1152x2048 |
| 4:3 | 1536x1152 | 2048x1536 |
| 3:4 | 1152x1536 | 1536x2048 |

其他 resolution 档（如 4K）回退 1K 表；已是 `WxH` 格式则透传。

## 7. 状态与文案

- 项状态：queued 排队 / generating 生成中 / success 完成 / failed 失败 / stopped 已停止
- 进度文案：排队中 → 提交生成 → 已提交 {workId} → 生成中 {n} → 完成 | 失败 | 已停止；恢复场景：恢复轮询 / 恢复中 {n} / 仍在生成 / 历史结果
- 批次状态：running 进行中 / completed 已完成 / failed 有失败 / stopped 已停止
- 历史 summary：`{套装label} · {成功数}/{总数} 张完成`（running 追加 `· 进行中`，stopped `· 已停止`，failed `· 有失败`）
- 空态文案：`上传商品图或填写商品信息后，选择套装并开始生成。`
- 提交校验失败文案：`请先补充商品图、商品信息或选择生成模块` / `请先填写批量生成提示词` / `图片模型尚未加载`

## 8. 历史 payload 数据模型（字段名对齐参考站）

```json
{
  "assets": [{ "id": "product-xxx-1", "role": "product|reference", "name": "a.jpg", "url": "/storage/...", "status": "ready" }],
  "productName": "", "sellingPoints": "", "audience": "", "scenario": "",
  "platform": "淘宝/天猫", "language": "简体中文",
  "visualStyle": "高端商业摄影", "customStyle": "",
  "selectedSuite": "launch", "selectedModuleIds": ["main-square", "..."],
  "ratio": "1:1", "resolution": "2K", "countPerModule": 1,
  "copyEnabled": true, "extraPrompt": "", "modelId": "…",
  "batchPrompt": "", "batchCount": 20, "batchStrategy": "保持主体改局部",
  "batchSimilarity": "50%", "batchVariation": "中变化",
  "batchKeepKeys": ["subject", "composition", "lighting"],
  "batchChangeKeys": ["local-pattern", "style"],
  "batchId": "batch-…", "batchStatus": "completed",
  "results": [{ "id": "result-…", "moduleId": "detail-cover", "label": "详情页首屏", "ratio": "1:1", "prompt": "…", "status": "success", "imageUrl": "/storage/…", "workId": "…", "error": "", "progress": "完成" }],
  "updatedAt": 1792116715276
}
```

## 9. 参考站 API 契约（对照用；本仓库映射见开发计划）

| 能力 | 参考站 | 本仓库映射 |
|---|---|---|
| 上传 | POST /api/upload/asset（FormData file+folder） | POST /api/assets/upload（已有） |
| 反推 | POST /api/image/analyze {image:dataURL, prompt} | POST /api/agents/ecom_factory/reverse（新，glm-4-6v） |
| 生成 | POST /api/image/generate {toolId,model,prompt,ratio,resolution,size,n,imageMode,metadata,referenceImages[]} → {workId,…} | workflow project + worker processImageTask |
| 轮询 | GET /api/image/status?workId= | GET /api/agent-projects/:id |
| 直链刷新 | POST /api/image/display-urls | outputs 落库 URL（无需） |
| 下载 | GET /api/image/file/{workId} | 对象存储 URL 直下 |
| 模型目录 | GET /api/model-aliases/public?capability=image | GET /api/agents（runtime_config）+ models |
| 历史 | /api/ai-tools/history CRUD | workflow_projects 列表 + inputs 快照 |
| ZIP | 前端组包 | GET /api/agent-projects/:id/export（Go 组包） |

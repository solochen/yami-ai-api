# 灵感广场覆盖率与缺口报告

> 快照时间：2026-10-02T01:20:33.212Z
>
> 本报告由 `scripts/analyze-gallery-coverage.js` 生成。重复运行脚本可以刷新数据，不应手工修改统计数字。

## 1. 数据源与范围

| 数据集 | 地址 | 读取结果 |
| --- | --- | --- |
| 参考案例 | `https://raw.githubusercontent.com/freestylefly/awesome-gpt-image-2/main/data/cases.json` | 541 个 |
| 社区案例 | `http://localhost:8080/api/gallery` | 4 个 |

当前采用的覆盖目标：一级分类、风格、场景每项至少 **8** 个案例；重点标签组合每项至少 **3** 个案例。目标是运营优先级，不代表所有标签组合都必须生成。

## 2. 摘要

- 参考案例：**541** 个，13 个分类，19 个风格，10 个场景。
- Prompt 文本语言：英文及拉丁语系 249、中文 234、日文 56、韩文 2。
- 参考案例分类中未达 8 个的标签：**0** 个；风格：**8** 个；场景：**0** 个。
- 社区案例：**4**，当前仍使用旧版自由标签，尚未形成可与参考案例对齐的分类/风格/场景统计。
- 本次没有根据缺口自动生成或发布任何图片。

## 3. 参考案例单维覆盖

### 分类

| 分类 | 当前案例数 | 目标（8） | 缺口 |
| --- | --- | --- | --- |
| Architecture & Spaces | 12 | 8 | 0 |
| Brand & Logos | 27 | 8 | 0 |
| Characters & People | 31 | 8 | 0 |
| Charts & Infographics | 53 | 8 | 0 |
| Documents & Publishing | 11 | 8 | 0 |
| History & Classical Themes | 16 | 8 | 0 |
| Illustration & Art | 59 | 8 | 0 |
| Other Use Cases | 28 | 8 | 0 |
| Photography & Realism | 78 | 8 | 0 |
| Posters & Typography | 90 | 8 | 0 |
| Products & E-commerce | 42 | 8 | 0 |
| Scenes & Storytelling | 21 | 8 | 0 |
| UI & Interfaces | 73 | 8 | 0 |

### 风格

| 风格 | 当前案例数 | 目标（8） | 缺口 |
| --- | --- | --- | --- |
| 3D | 40 | 8 | 0 |
| Architecture | 4 | 8 | 4 |
| Brand | 86 | 8 | 0 |
| Character | 113 | 8 | 0 |
| Characters | 2 | 8 | 6 |
| Charts | 3 | 8 | 5 |
| Classical | 14 | 8 | 0 |
| Documents | 4 | 8 | 4 |
| History | 7 | 8 | 1 |
| Illustration | 103 | 8 | 0 |
| Infographic | 74 | 8 | 0 |
| Other Use Cases | 9 | 8 | 0 |
| Photography | 3 | 8 | 5 |
| Poster | 199 | 8 | 0 |
| Product | 56 | 8 | 0 |
| Products | 5 | 8 | 3 |
| Realistic | 217 | 8 | 0 |
| Scenes | 2 | 8 | 6 |
| UI | 250 | 8 | 0 |

### 场景

| 场景 | 当前案例数 | 目标（8） | 缺口 |
| --- | --- | --- | --- |
| Commerce | 382 | 8 | 0 |
| Creative | 52 | 8 | 0 |
| Education | 42 | 8 | 0 |
| Fashion | 122 | 8 | 0 |
| Food | 42 | 8 | 0 |
| History | 12 | 8 | 0 |
| Social | 87 | 8 | 0 |
| Story | 65 | 8 | 0 |
| Tech | 404 | 8 | 0 |
| Travel | 48 | 8 | 0 |

## 4. Prompt 语言覆盖

| 展示名 | 检测值 | 案例数 | 说明 |
| --- | --- | --- | --- |
| 英文及拉丁语系 | latin | 249 | 仅代表 Prompt 文本 |
| 中文 | zh | 234 | 仅代表 Prompt 文本 |
| 日文 | ja | 56 | 仅代表 Prompt 文本 |
| 韩文 | ko | 2 | 仅代表 Prompt 文本 |

检测规则与当前 Web 的 `detectGalleryLanguage` 保持一致：日文、韩文、中文、拉丁字符依次检测。一个 Prompt 如果混有多种文字，当前实现只计为一个主检测结果。

**重要限制：** 本报告没有把图片中的实际文字语言当作已知事实。要补齐“图片文字语言”维度，必须在后续同步/审核阶段接入 OCR，并将 OCR 结果和置信度写入案例数据。

## 5. 重点组合缺口

目标组合数为每项 3 个。零案例或低于目标不等于必须生成：应先确认组合是否有用户价值、是否存在标签定义冲突，以及是否可以通过相近案例满足需求。

### 分类 + 场景

- 可能组合：130
- 已有案例的组合：112（86.2%）
- 零案例组合：18
- 未达到目标组合：46

最需要优先关注的组合（按案例数从少到多，最多展示 20 个）：

| 左侧标签 | 右侧标签 | 案例数 | 距离目标 |
| --- | --- | --- | --- |
| Architecture & Spaces | Education | 0 | 3 |
| Architecture & Spaces | History | 0 | 3 |
| Brand & Logos | History | 0 | 3 |
| Brand & Logos | Travel | 0 | 3 |
| Characters & People | History | 0 | 3 |
| Characters & People | Travel | 0 | 3 |
| Documents & Publishing | Education | 0 | 3 |
| Documents & Publishing | Fashion | 0 | 3 |
| Documents & Publishing | Food | 0 | 3 |
| Documents & Publishing | History | 0 | 3 |
| History & Classical Themes | Education | 0 | 3 |
| History & Classical Themes | Food | 0 | 3 |
| History & Classical Themes | Travel | 0 | 3 |
| Illustration & Art | History | 0 | 3 |
| Photography & Realism | History | 0 | 3 |
| Posters & Typography | History | 0 | 3 |
| Products & E-commerce | History | 0 | 3 |
| Scenes & Storytelling | Creative | 0 | 3 |
| Architecture & Spaces | Creative | 1 | 2 |
| Architecture & Spaces | Fashion | 1 | 2 |


### 分类 + 风格

- 可能组合：247
- 已有案例的组合：120（48.6%）
- 零案例组合：127
- 未达到目标组合：160

最需要优先关注的组合（按案例数从少到多，最多展示 20 个）：

| 左侧标签 | 右侧标签 | 案例数 | 距离目标 |
| --- | --- | --- | --- |
| Architecture & Spaces | Character | 0 | 3 |
| Architecture & Spaces | Characters | 0 | 3 |
| Architecture & Spaces | Charts | 0 | 3 |
| Architecture & Spaces | Classical | 0 | 3 |
| Architecture & Spaces | Documents | 0 | 3 |
| Architecture & Spaces | History | 0 | 3 |
| Architecture & Spaces | Other Use Cases | 0 | 3 |
| Architecture & Spaces | Photography | 0 | 3 |
| Architecture & Spaces | Products | 0 | 3 |
| Architecture & Spaces | Scenes | 0 | 3 |
| Brand & Logos | Architecture | 0 | 3 |
| Brand & Logos | Characters | 0 | 3 |
| Brand & Logos | Charts | 0 | 3 |
| Brand & Logos | Classical | 0 | 3 |
| Brand & Logos | Documents | 0 | 3 |
| Brand & Logos | History | 0 | 3 |
| Brand & Logos | Other Use Cases | 0 | 3 |
| Brand & Logos | Photography | 0 | 3 |
| Brand & Logos | Products | 0 | 3 |
| Brand & Logos | Scenes | 0 | 3 |


### 场景 + Prompt 语言

- 可能组合：50
- 已有案例的组合：32（64.0%）
- 零案例组合：18
- 未达到目标组合：24

最需要优先关注的组合（按案例数从少到多，最多展示 20 个）：

| 左侧标签 | 右侧标签 | 案例数 | 距离目标 |
| --- | --- | --- | --- |
| Commerce | other | 0 | 3 |
| Creative | ko | 0 | 3 |
| Creative | other | 0 | 3 |
| Education | ko | 0 | 3 |
| Education | other | 0 | 3 |
| Fashion | ko | 0 | 3 |
| Fashion | other | 0 | 3 |
| Food | ko | 0 | 3 |
| Food | other | 0 | 3 |
| History | ja | 0 | 3 |
| History | ko | 0 | 3 |
| History | other | 0 | 3 |
| Social | other | 0 | 3 |
| Story | ko | 0 | 3 |
| Story | other | 0 | 3 |
| Tech | other | 0 | 3 |
| Travel | ko | 0 | 3 |
| Travel | other | 0 | 3 |
| Commerce | ko | 1 | 2 |
| Creative | ja | 1 | 2 |


## 6. 社区案例现状

### 自由标签

| 标签 | 案例数 |
| --- | --- |
| concept | 2 |
| illustration | 2 |
| anime | 1 |
| ecommerce | 1 |
| photography | 1 |

### Prompt/标题文本语言

| 展示名 | 检测值 | 案例数 |
| --- | --- | --- |
| 中文 | zh | 4 |

社区案例目前有 4 个公开种子案例，且标签 `illustration`、`photography`、`concept`、`ecommerce`、`anime` 与参考案例的分类/风格/场景不是同一套枚举。下一步不应直接把这些字符串拼在一起计算总覆盖率。

## 7. 数据质量信号

| 检查项 | 结果 |
| --- | --- |
| 参考案例缺标题 | 0 |
| 参考案例缺 Prompt | 0 |
| 参考案例缺分类 | 0 |
| 参考案例缺风格 | 0 |
| 参考案例缺场景 | 0 |
| 参考案例重复标题 + Prompt 组 | 10 |
| 重复组涉及案例数 | 30 |

- 图片文字语言尚未通过 OCR 统计；当前语言覆盖仅依据 Prompt/标题文本检测。
- 参考案例与社区标签体系语义不同，本报告不把两者直接相加作为统一覆盖率。

## 8. 缺口判定与下一批建议

建议按以下顺序使用本报告，而不是直接按零值批量生成：

1. 先处理单维度缺口：优先补齐案例数为 0 或明显低于目标的分类、风格、场景。
2. 再从“分类 + 场景”中挑选与产品实际使用相关的组合，避免生成无意义的笛卡尔积。
3. 对“场景 + Prompt 语言”缺口，先确认用户需要的是 Prompt 语言还是图片中文字语言；后者要等 OCR 数据建立后再排期。
4. 参考案例先做来源、许可证、图片可托管性和内容质量核验，再进入本地快照；社区案例则单独走投稿审核链路。
5. 每个缺口任务先生成案例规格，人工确认后才进入图片生成和审核。

## 9. 下一步实施边界

本阶段只完成统计和报告，不修改案例数据，不生成媒体。

下一阶段建议进入“本地案例库与来源同步”的最小设计：先把经过许可证确认的参考案例保存为服务端可用快照，并建立来源 ID、内容 hash 和幂等同步规则。同步完成后再用同一套分析脚本针对本地快照做覆盖率统计。

# 阶段 5 案例规格草稿

> 本文件由 scripts/generate-gallery-case-specs.js 生成。只包含可审核规格，不生成图片、不写入公共图库、不自动发布。

- 来源报告：docs/inspiration-gallery-coverage-report.json
- 来源快照：2026-10-02T01:20:33.212Z
- 来源 hash：90c725ff923baf3e8260e90ffb96f0ce36d3e6d25bc1ed0c70b4521ad6952696
- 缺口任务：13
- 案例规格：20

## 运营边界

- 所有规格初始状态为 `draft`。
- 规格只有在字段校验通过、风险审核完成后才能被标记为 `ready`。
- `ready` 只代表允许进入下一阶段媒体生成，不代表已经生成或公开。
- 本批不包含真实品牌、名人、版权角色和真实机构作为主体。

## 首批任务

| 任务 | 类型 | 目标 | 规格数 | 优先级 |
| --- | --- | ---: | ---: | --- |
| Architecture 风格补齐 | style | 2 | 2 | high |
| Characters 风格补齐 | style | 2 | 2 | high |
| Charts 风格补齐 | style | 2 | 2 | high |
| Documents 风格补齐 | style | 2 | 2 | high |
| History 风格补齐 | style | 1 | 1 | high |
| Photography 风格补齐 | style | 2 | 2 | high |
| Products 风格补齐 | style | 2 | 2 | high |
| Scenes 风格补齐 | style | 2 | 2 | high |
| Architecture & Spaces + Education 组合 | category_scene | 1 | 1 | medium |
| Documents & Publishing + Food 组合 | category_scene | 1 | 1 | medium |
| History & Classical Themes + Travel 组合 | category_scene | 1 | 1 | medium |
| Scenes & Storytelling + Creative 组合 | category_scene | 1 | 1 | medium |
| Creative + 韩文 Prompt 组合 | scene_language | 1 | 1 | medium |

## 下一步审核

1. 运营在后台检查标题、Prompt、标签和候选模型。
2. 对有风险标记的规格补充风险审核结论。
3. 只把确认无误且可生成的规格标记为 `ready`。
4. 阶段 6 再根据 `ready` 规格创建批量媒体生成任务。

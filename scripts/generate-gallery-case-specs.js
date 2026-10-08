#!/usr/bin/env node

/**
 * Generate stage-5 case-spec drafts from a coverage report.
 * This command only writes reviewable JSON/Markdown specifications. It never
 * calls an image provider and never changes public gallery data.
 */

const fs = require("node:fs");
const crypto = require("node:crypto");

const reportPath = process.env.GALLERY_COVERAGE_JSON || "docs/inspiration-gallery-coverage-report.json";
const outPath = process.env.GALLERY_CASE_SPECS_OUT || "docs/gallery-case-specs-stage5.json";
const markdownPath = process.env.GALLERY_CASE_SPECS_MD || "docs/gallery-case-specs-stage5.md";
const tick = String.fromCharCode(96);

const modelCandidates = [
  { code: "gpt-image-2", role: "primary", rationale: "优先用于文字结构、信息图和复杂构图的候选验证。" },
  { code: "seedream-5-0-lite", role: "fallback", rationale: "作为成本更低的备选图片模型，便于后续小批量对比。" },
];

const plans = [
  { id: "style-architecture", dimension_type: "style", style: "Architecture", title: "Architecture 风格补齐", rationale: "覆盖率报告显示 Architecture 风格仅 4 个案例，优先补充无品牌、无真实人物的可持续建筑主题。", specs: [
    ["可持续城市能源中心剖面", "Architecture & Spaces", "Tech", "en", "以建筑剖面和清晰结构表达能源、储能与公共空间关系。"],
    ["滨水公共图书馆空间导览", "Architecture & Spaces", "Education", "en", "用建筑摄影与导览图结合的方式展示空间层级和动线。"],
  ] },
  { id: "style-characters", dimension_type: "style", style: "Characters", title: "Characters 风格补齐", rationale: "Characters 风格只有 2 个案例，使用原创人物和通用场景，避开现实名人、版权角色和品牌服饰。", specs: [
    ["原创植物学研究员角色设定", "Characters & People", "Creative", "en", "展示原创成年角色的正面、侧面和工作状态，强调服装材质与工具。"],
    ["原创城市骑行者角色卡", "Characters & People", "Travel", "zh-CN", "用原创成年角色表达城市骑行装备、姿态和安全配件。"],
  ] },
  { id: "style-charts", dimension_type: "style", style: "Charts", title: "Charts 风格补齐", rationale: "Charts 风格只有 3 个案例，优先补教育和公共知识场景，验收重点是信息层级与可读性。", specs: [
    ["城市用水循环科普信息图", "Charts & Infographics", "Education", "zh-CN", "以流程图、比例图和局部放大解释城市用水循环，不放置真实机构标识。"],
    ["家庭预算分配可视化卡片", "Charts & Infographics", "Commerce", "zh-CN", "用抽象示例数据展示预算分类、比例和趋势，数据必须明确为虚构示例。"],
  ] },
  { id: "style-documents", dimension_type: "style", style: "Documents", title: "Documents 风格补齐", rationale: "Documents 风格只有 4 个案例，补充可复用的课程讲义和操作手册版式，不使用真实机构名称。", specs: [
    ["植物养护课程讲义版式", "Documents & Publishing", "Education", "zh-CN", "制作一页模块化课程讲义，包含标题、步骤、插图占位和注意事项。"],
    ["家庭烘焙步骤手册版式", "Documents & Publishing", "Food", "zh-CN", "制作一页易读的烘焙操作手册，使用虚构配方和通用器具。"],
  ] },
  { id: "style-history", dimension_type: "style", style: "History", title: "History 风格补齐", rationale: "History 风格距离目标还差 1 个，使用公共历史主题的抽象叙事，不复刻受版权保护的艺术作品。", specs: [
    ["古代商路地图叙事海报", "History & Classical Themes", "Travel", "zh-CN", "以原创地图构图讲述古代商路的货物、地形和旅程节点，避免真实机构徽记。"],
  ] },
  { id: "style-photography", dimension_type: "style", style: "Photography", title: "Photography 风格补齐", rationale: "Photography 风格只有 3 个案例，先补无品牌静物与通用商业摄影，避免真实人物授权问题。", specs: [
    ["无品牌陶器静物商业摄影", "Photography & Realism", "Commerce", "zh-CN", "拍摄无品牌手作陶器组合，强调材质、光影、留白和电商构图。"],
    ["城市雨后公共空间摄影", "Photography & Realism", "Travel", "en", "表现无可识别人物的雨后公共空间，重点检验真实光线和材质。"],
  ] },
  { id: "style-products", dimension_type: "style", style: "Products", title: "Products 风格补齐", rationale: "Products 风格只有 5 个案例，补充无品牌产品展示和结构图，服务电商与设计评审场景。", specs: [
    ["模块化收纳盒产品爆炸图", "Products & E-commerce", "Commerce", "zh-CN", "展示原创模块化收纳盒的组件、连接方式和使用状态，不出现品牌或商标。"],
    ["便携净水壶产品展示板", "Products & E-commerce", "Tech", "en", "展示原创净水壶的材质、滤芯结构与使用场景，使用中性产品命名。"],
  ] },
  { id: "style-scenes", dimension_type: "style", style: "Scenes", title: "Scenes 风格补齐", rationale: "Scenes 风格只有 2 个案例，补充具有明确环境叙事的原创场景，避免版权世界观。", specs: [
    ["森林观测站叙事场景", "Scenes & Storytelling", "Story", "en", "以原创森林观测站为场景，表达时间、天气和人物行动线索，不出现现实机构。"],
    ["夜间创意工作室场景", "Scenes & Storytelling", "Creative", "zh-CN", "表现原创创意工作室的桌面、草图、灯光与空间关系，不使用真实品牌设备。"],
  ] },
  { id: "pair-category-scene", dimension_type: "category_scene", category: "Architecture & Spaces", scene: "Education", title: "Architecture & Spaces + Education 组合", rationale: "覆盖率报告显示该组合为 0，且对教学空间、建筑导览和科普展示有明确产品价值。", specs: [
    ["未来学校建筑剖面教学图", "Architecture & Spaces", "Education", "zh-CN", "用原创未来学校建筑剖面展示教室、实验室、图书区和公共动线。"],
  ] },
  { id: "pair-category-scene-doc-food", dimension_type: "category_scene", category: "Documents & Publishing", scene: "Food", title: "Documents & Publishing + Food 组合", rationale: "该组合为 0，适合补充高复用的教程型内容，并能同时验证文本排版与步骤信息。", specs: [
    ["家常汤品图文教程页", "Documents & Publishing", "Food", "zh-CN", "制作一页通用家常汤品教程，包含食材、步骤、时间和安全提示，文字保持简洁。"],
  ] },
  { id: "pair-category-scene-history", dimension_type: "category_scene", category: "History & Classical Themes", scene: "Travel", title: "History & Classical Themes + Travel 组合", rationale: "该组合为 0，使用原创公共历史主题和抽象旅行路线，避免真实人物、机构和受保护文物复刻。", specs: [
    ["古代商路旅行路线图", "History & Classical Themes", "Travel", "zh-CN", "以原创路线图表现古代商路地理、季节和物资节点，使用抽象地名和图例。"],
  ] },
  { id: "pair-category-scene-story", dimension_type: "category_scene", category: "Scenes & Storytelling", scene: "Creative", title: "Scenes & Storytelling + Creative 组合", rationale: "该组合为 0，补充创意工作流中的环境叙事案例，适合验证场景连续性和镜头构图。", specs: [
    ["创意工作室的一天", "Scenes & Storytelling", "Creative", "zh-CN", "通过同一原创工作室的晨间、制作和收工线索表达创作过程，不出现真实品牌。"],
  ] },
  { id: "language-korean", dimension_type: "scene_language", scene: "Creative", prompt_language: "ko", title: "Creative + 韩文 Prompt 组合", rationale: "覆盖率报告显示 Creative + ko 为 0；先建设一个独立的韩文 Prompt 草稿，图片文字语言保持 none，避免把两种语言混为一谈。", specs: [
    ["창작 스튜디오 무드보드", "Scenes & Storytelling", "Creative", "ko", "창작 스튜디오의 자연광, 재료, 작업 테이블과 차분한 색감을 표현하는 무드보드."],
  ] },
];

function stableHash(value) {
  return crypto.createHash("sha256").update(JSON.stringify(value)).digest("hex");
}

function makeTask(plan, report, reportHash) {
  const count = plan.style ? (report.reference.styles.find((item) => item.value === plan.style)?.count || 0) : 0;
  return {
    public_id: "gap_stage5_" + plan.id,
    source_report_hash: reportHash,
    source_snapshot_at: report.snapshotAt,
    dimension_type: plan.dimension_type,
    category: plan.category || "",
    style: plan.style || "",
    scene: plan.scene || "",
    prompt_language: plan.prompt_language || "",
    target_count: 8,
    current_count: count,
    gap_count: Math.max(1, plan.specs.length),
    priority: plan.dimension_type === "style" ? "high" : "medium",
    title: plan.title,
    rationale: plan.rationale,
    risk_flags: [],
    source_context: { report_path: reportPath, selection_policy: "high_value_gap_first", media_generation_allowed: false },
  };
}

function makeSpec(plan, row, index, reportHash) {
  const [title, category, scene, language, purpose] = row;
  const styles = plan.style ? [plan.style] : category === "Architecture & Spaces" ? ["Architecture"] : category === "Documents & Publishing" ? ["Documents"] : category === "History & Classical Themes" ? ["History"] : category === "Photography & Realism" ? ["Photography"] : category === "Products & E-commerce" ? ["Products"] : category === "Scenes & Storytelling" ? ["Scenes"] : ["Illustration"];
  const prompt = language === "ko" ? "원본 브랜드와 실존 인물을 사용하지 않고 " + purpose : "Create an original, production-ready image concept for " + title + ". " + purpose + " Use neutral, non-branded subjects, coherent composition, accurate materials, clear hierarchy, and no copyrighted characters or real-person likeness.";
  return {
    public_id: "spec_stage5_" + plan.id + "_" + String(index + 1).padStart(2, "0"),
    gap_task_public_id: "gap_stage5_" + plan.id,
    title, purpose, prompt,
    negative_prompt: "brand logos, trademarks, real public figures, copyrighted characters, unreadable text, random watermark, distorted anatomy, duplicate objects",
    prompt_language: language, image_text_languages: [], expected_text: [],
    category, styles, scenes: [scene], aspect_ratio: category.includes("Documents") || category.includes("Charts") ? "4:5" : "3:4",
    output_media_type: "image", candidate_models: modelCandidates,
    acceptance_criteria: [
      "分类、风格和场景与缺口任务一致，主体可在缩略图中被识别。",
      "无品牌、名人、版权角色和真实机构标识；如触发风险必须先完成人工风险审核。",
      "主体结构、材质、光影和构图稳定，未出现明显重复、畸变或不可解释元素。",
      "图片不要求出现文字；若模型生成随机文字，审核时应判定为需返修。",
      "生成结果保留模型、参数、成本和资源地址，进入 pending_review，不得自动公开。",
    ],
    selection_reason: "由覆盖率报告的高价值缺口筛选；先覆盖单维度缺口，再覆盖有实际使用价值的分类与场景组合。",
    risk_flags: [], risk_notes: "", risk_review_status: "not_required", status: "draft",
    generator_type: "codex", generator_model_code: "", generator_prompt_version: "stage5-v1",
    generation_input: { source_report_hash: reportHash, plan_id: plan.id, media_generation_allowed: false },
    generation_output: {}, estimated_cost: 0, actual_cost: 0, input_tokens: 0, output_tokens: 0,
  };
}

const report = JSON.parse(fs.readFileSync(reportPath, "utf8"));
const reportHash = stableHash(report);
const tasks = plans.map((plan) => makeTask(plan, report, reportHash));
const specs = plans.flatMap((plan) => plan.specs.map((row, index) => makeSpec(plan, row, index, reportHash)));
const output = {
  schema_version: "gallery-case-specs/v1",
  generated_at: new Date().toISOString(),
  generator: { type: "codex", prompt_version: "stage5-v1", media_generation_called: false },
  source_report: { path: reportPath, snapshot_at: report.snapshotAt, sha256: reportHash },
  policy: { status: "draft", public_publish_allowed: false, media_generation_allowed: false },
  tasks, specs,
};
fs.writeFileSync(outPath, JSON.stringify(output, null, 2) + "\n");

const rows = [
  "# 阶段 5 案例规格草稿", "",
  "> 本文件由 scripts/generate-gallery-case-specs.js 生成。只包含可审核规格，不生成图片、不写入公共图库、不自动发布。", "",
  "- 来源报告：" + reportPath,
  "- 来源快照：" + report.snapshotAt,
  "- 来源 hash：" + reportHash,
  "- 缺口任务：" + tasks.length,
  "- 案例规格：" + specs.length, "",
  "## 运营边界", "",
  "- 所有规格初始状态为 " + tick + "draft" + tick + "。",
  "- 规格只有在字段校验通过、风险审核完成后才能被标记为 " + tick + "ready" + tick + "。",
  "- " + tick + "ready" + tick + " 只代表允许进入下一阶段媒体生成，不代表已经生成或公开。",
  "- 本批不包含真实品牌、名人、版权角色和真实机构作为主体。", "",
  "## 首批任务", "",
  "| 任务 | 类型 | 目标 | 规格数 | 优先级 |",
  "| --- | --- | ---: | ---: | --- |",
  ...tasks.map((task) => "| " + task.title + " | " + task.dimension_type + " | " + task.gap_count + " | " + specs.filter((item) => item.gap_task_public_id === task.public_id).length + " | " + task.priority + " |"),
  "", "## 下一步审核", "",
  "1. 运营在后台检查标题、Prompt、标签和候选模型。",
  "2. 对有风险标记的规格补充风险审核结论。",
  "3. 只把确认无误且可生成的规格标记为 " + tick + "ready" + tick + "。",
  "4. 阶段 6 再根据 " + tick + "ready" + tick + " 规格创建批量媒体生成任务。",
];
fs.writeFileSync(markdownPath, rows.join("\n") + "\n");
console.log(JSON.stringify({ report: reportPath, output: outPath, markdown: markdownPath, tasks: tasks.length, specs: specs.length, reportHash }, null, 2));

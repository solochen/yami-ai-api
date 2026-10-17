// 电商图片工厂：模块目录 + 提示词模板（与 services/worker/cmd/worker/ecom_factory_catalog.json 同源）。
// 双端各有 golden 对拍测试（docs/ecom-factory-spec.md §4），修改任一端必须同步另一端并通过测试。

export type EcomModule = {
  id: string; label: string; shortLabel: string; channel: string;
  defaultRatio: string; goal: string; prompt: string; accent: string;
};
export type EcomSuite = { id: string; label: string; description: string; moduleIds: string[] };
export type EcomBatchKey = { key: string; label: string };

export const ECOM_LIMITS = {
  maxProductImages: 8, maxReferenceImages: 6, maxPlanItems: 24,
  maxCountPerModule: 3, maxBatchCount: 200, quickMaxReferences: 3, defaultMaxReferences: 8,
};

export const ECOM_PLATFORMS = ["淘宝/天猫", "京东", "拼多多", "抖音电商", "小红书", "亚马逊", "独立站"] as const;
export const ECOM_LANGUAGES = ["简体中文", "英文", "中英双语", "无文字"] as const;
export const ECOM_VISUAL_STYLES = ["高端商业摄影", "干净白底货架", "真实生活方式", "小红书种草", "直播间强转化", "极简科技感", "温暖家居感", "自定义"] as const;
export const ECOM_RATIOS = ["1:1", "3:4", "4:3", "9:16", "16:9"] as const;
export const ECOM_RESOLUTIONS = ["1K", "2K"] as const;

export const ECOM_SIZE_MAP: Record<string, Record<string, string>> = {
  "1K": { "1:1": "1024x1024", "16:9": "1536x864", "9:16": "864x1536", "4:3": "1536x1152", "3:4": "1152x1536" },
  "2K": { "1:1": "2048x2048", "16:9": "2048x1152", "9:16": "1152x2048", "4:3": "2048x1536", "3:4": "1536x2048" },
};

export const ECOM_MODULES: EcomModule[] = [
  { id: "main-square", label: "平台主图", shortLabel: "主图", channel: "淘宝/天猫/京东/拼多多", defaultRatio: "1:1", goal: "清晰展示商品主体，适合搜索列表第一张图。", prompt: "生成电商平台首图。商品主体居中，轮廓完整，材质真实，画面干净，留出适度呼吸空间。构图要适合移动端搜索列表缩略图，第一眼能识别品类和卖点。", accent: "#0ea5e9" },
  { id: "white-bg", label: "白底质检图", shortLabel: "白底", channel: "平台审核/货架图", defaultRatio: "1:1", goal: "保留商品外观，用纯净白底提升审核通过率。", prompt: "生成标准电商白底图。纯白或近白背景，商品边缘干净，阴影轻微自然，不添加装饰物，不改变商品结构、颜色、品牌标识和材质。", accent: "#64748b" },
  { id: "scene-hero", label: "场景卖点图", shortLabel: "场景", channel: "详情首屏/广告图", defaultRatio: "4:3", goal: "用真实使用场景承接核心卖点。", prompt: "生成商业场景卖点图。围绕目标用户的真实使用场景设计环境，商品保持主角位置，加入合理道具和光影，画面有生活感但不过度杂乱。", accent: "#10b981" },
  { id: "detail-cover", label: "详情页首屏", shortLabel: "详情", channel: "详情页", defaultRatio: "3:4", goal: "首屏完成商品、利益点和质感表达。", prompt: "生成电商详情页首屏图。画面纵向排版，商品主体和关键卖点层次清楚，有高级商业海报质感，适合放在详情页顶部承接转化。", accent: "#f59e0b" },
  { id: "texture-close", label: "材质细节图", shortLabel: "细节", channel: "详情模块", defaultRatio: "4:3", goal: "突出材质、工艺、纹理、接口或细节优势。", prompt: "生成商品材质细节图。使用近景或微距视角，突出纹理、工艺、接口、结构或手感，保留真实比例和材质，背景简洁，光线能展示细节层次。", accent: "#8b5cf6" },
  { id: "selling-grid", label: "卖点模块图", shortLabel: "卖点", channel: "详情模块", defaultRatio: "3:4", goal: "把多个卖点整理成电商详情模块。", prompt: "生成卖点模块图。使用清晰的信息层级表达 2 到 4 个核心卖点，商品图、图标式视觉元素和留白排版协调，适合详情页连续阅读。", accent: "#ef4444" },
  { id: "compare", label: "对比说明图", shortLabel: "对比", channel: "详情模块/广告", defaultRatio: "4:3", goal: "表达使用前后、旧款新款或竞品差异。", prompt: "生成对比说明图。采用左右或上下对比构图，突出使用前后、升级前后或不同方案的差异。画面要直观克制，避免虚假夸张。", accent: "#14b8a6" },
  { id: "seed-cover", label: "种草封面", shortLabel: "种草", channel: "小红书/抖音封面", defaultRatio: "3:4", goal: "让商品像真实用户内容一样有点击欲。", prompt: "生成社媒种草封面。画面真实自然，有轻内容感和分享感，商品融入生活场景，构图适合小红书或短视频封面，避免硬广堆砌。", accent: "#ec4899" },
  { id: "lifestyle", label: "生活方式图", shortLabel: "生活", channel: "种草/品牌内容", defaultRatio: "4:3", goal: "塑造品牌调性和目标人群的生活画面。", prompt: "生成生活方式广告图。以目标用户的日常空间、动作或情绪为核心，商品自然出现，整体像高质量品牌内容，真实、干净、有温度。", accent: "#22c55e" },
  { id: "live-sticker", label: "直播贴片", shortLabel: "贴片", channel: "直播间/短视频挂件", defaultRatio: "9:16", goal: "为直播间生成可叠加的商品利益点画面。", prompt: "生成直播间商品贴片图。竖屏构图，商品和利益点醒目，留出主播区域和按钮区域，视觉强但不遮挡主体，适合直播间或短视频叠加。", accent: "#f97316" },
  { id: "style-clone", label: "风格复刻图", shortLabel: "复刻", channel: "爆款复刻/系列图", defaultRatio: "1:1", goal: "参考样图的构图、光影、色调和电商氛围。", prompt: "生成风格复刻图。参考上传样图的构图、光影、色彩、布景和商业质感，但主体必须替换为当前商品，不复制原图文字、水印、品牌或人物身份。", accent: "#6366f1" },
  { id: "color-series", label: "系列配色图", shortLabel: "系列", channel: "店铺系列/多 SKU", defaultRatio: "1:1", goal: "让同一商品形成统一视觉系列。", prompt: "生成系列配色图。保持商品主体、机位和排版一致，调整背景色、道具和光影形成同风格系列，适合店铺 SKU 展示和批量上新。", accent: "#06b6d4" },
  { id: "shop-banner", label: "店铺横幅", shortLabel: "横幅", channel: "店铺首页/活动页", defaultRatio: "16:9", goal: "生成活动、上新或品牌专区横幅。", prompt: "生成电商店铺横幅。横向构图，商品主体清晰，左右留出运营文案空间，画面适合店铺首页、活动页或广告 banner。", accent: "#84cc16" },
  { id: "reference-batch", label: "参考图批量生成", shortLabel: "批量", channel: "多参考图/批量改款", defaultRatio: "1:1", goal: "根据上传参考图和用户指令，批量生成稳定可控的电商图片。", prompt: "生成参考图驱动的批量电商图片。以用户上传图片作为主体、构图、风格或局部元素参考，并严格遵循用户提示词中指定的保持项、改变项和相似度要求。", accent: "#0f766e" },
];

export const ECOM_SUITES: EcomSuite[] = [
  { id: "launch", label: "上新快发", description: "主图、白底、场景、详情和种草一次配齐。", moduleIds: ["main-square", "white-bg", "scene-hero", "detail-cover", "seed-cover"] },
  { id: "detail", label: "详情页套装", description: "详情首屏、材质、卖点、对比和场景承接转化。", moduleIds: ["detail-cover", "texture-close", "selling-grid", "compare", "scene-hero"] },
  { id: "social", label: "种草内容", description: "面向小红书、抖音和直播间的内容图。", moduleIds: ["seed-cover", "lifestyle", "live-sticker", "scene-hero"] },
  { id: "clone", label: "风格复刻", description: "参考爆款样图生成同调性的商品系列图。", moduleIds: ["style-clone", "color-series", "main-square", "shop-banner"] },
  { id: "batch", label: "参考图批量生成", description: "多参考图加自定义指令，批量生成改款、融合和营销图。", moduleIds: ["reference-batch"] },
];

export const ECOM_BATCH = {
  strategies: ["保持主体改局部", "参考风格重绘", "多图融合生成", "按提示词自由扩展"] as const,
  similarities: ["30%", "50%", "70%", "90%"] as const,
  variations: ["低变化", "中变化", "高变化"] as const,
  countOptions: [5, 10, 20, 50, 100, 200],
  defaultKeepKeys: ["subject", "composition", "lighting"],
  defaultChangeKeys: ["local-pattern", "style"],
  keepKeys: [
    { key: "subject", label: "主体" }, { key: "composition", label: "构图" }, { key: "lighting", label: "光影" },
    { key: "background", label: "背景" }, { key: "material", label: "材质" }, { key: "color", label: "颜色" },
  ] as EcomBatchKey[],
  changeKeys: [
    { key: "local-pattern", label: "局部图案" }, { key: "background", label: "背景" }, { key: "props", label: "道具" },
    { key: "style", label: "风格" }, { key: "copy", label: "文案" }, { key: "scene", label: "场景" },
  ] as EcomBatchKey[],
};

export const ECOM_REFINE = {
  prefix: "以参考图为基准进行精修，严格保持商品的外形、比例、材质、颜色、品牌标识与整体构图一致，仅按以下要求调整，其余部分保持原样：",
  defaultInstruction: "整体质感、清晰度与细节优化",
};

export const ECOM_REVERSE_PROMPT = "请把这张图片反推成一段可直接用于电商商品图 AI 生成的中文提示词，覆盖：商品主体与品类、材质质感、构图与机位、光影与氛围、背景与场景、色调与风格。只输出一段可直接使用的提示词，不要解释、不要分点、不要加引号。";

type TemplateLine = { when?: string; text: string };

const NORMAL_LINES: TemplateLine[] = [
  { text: "你是资深电商视觉设计师，请生成一张可直接用于{{module.channel}}的{{module.label}}。" },
  { text: "商品：{{productName||上传参考图中的商品}}" },
  { text: "核心卖点：{{sellingPoints||请从商品外观和使用场景中提炼核心卖点}}" },
  { text: "目标人群：{{audience||目标电商用户}}" },
  { text: "使用场景：{{scenario||真实购买与使用场景}}" },
  { text: "目标平台：{{platform}}" },
  { text: "视觉风格：{{visualStyle||高端商业摄影}}" },
  { text: "画面任务：{{module.prompt}}" },
  { text: "商业目标：{{module.goal}}" },
  { text: "版本方向：第 {{variantIndex}} 张，构图和道具可以有轻微差异，但商品身份必须一致。" },
  { when: "hasAssets", text: "严格参考已上传图片中的商品身份、外形、比例、材质、颜色和关键结构；参考图只用于构图、光影、风格和场景方向，不复制水印、人物身份或第三方品牌。" },
  { when: "noAssets", text: "无参考图时，根据商品信息生成可信的商业摄影画面，商品结构要合理，不要出现不可能的材质和形态。" },
  { when: "copyAllowed", text: "可以加入少量{{language}}短文案，文案必须清晰、无错别字、少而有力；不确定的文字区域请留白。" },
  { when: "copyForbidden", text: "画面中不要新增文字、数字、二维码、水印和无关 LOGO；若参考图已有商品标识，只能在商品本体上自然保留。" },
  { text: "输出要求：真实商业摄影质感，主体清楚，边缘干净，光影自然，材质可信，构图适合移动端电商浏览。" },
  { text: "禁止：低清晰度、变形、错位手指、混乱文字、错误商标、过度磨皮、重复主体、拼接痕迹、侵权品牌元素。" },
  { when: "hasExtra", text: "补充要求：{{extraPrompt}}" },
];

const BATCH_LINES: TemplateLine[] = [
  { text: "你是资深电商视觉设计师，请根据用户上传的一张或多张参考图，生成新的电商图片。" },
  { text: "参考图使用规则：第一张上传图作为主要视觉参考，其余图片作为风格、构图、背景、材质或局部元素参考；不得直接复制参考图中的水印、角标、第三方品牌、人物身份或受保护图案。" },
  { when: "hasProductName", text: "商品/项目：{{productName}}" },
  { when: "hasSellingPoints", text: "商业卖点：{{sellingPoints}}" },
  { when: "hasAudience", text: "目标人群：{{audience}}" },
  { when: "hasScenario", text: "使用场景：{{scenario}}" },
  { text: "目标平台：{{platform}}" },
  { text: "视觉风格：{{visualStyle||高端商业摄影}}" },
  { text: "生成方式：{{batch.strategy}}" },
  { text: "目标相似度：约 {{batch.similarity}}。保留商业属性和核心方向，但必须通过新的构图、细节、元素组织或局部设计形成新图，避免直接复制。" },
  { text: "变化强度：{{batch.variation}}" },
  { text: "必须尽量保持：{{batch.keepLabels||按用户提示词判断}}" },
  { text: "允许重点改变：{{batch.changeLabels||按用户提示词判断}}。如果用户要求只改变局部元素，例如印花、标签、杯身图案、包装图案、背景道具或局部装饰，则必须把变化限制在该局部区域，其他未指定区域尽量保持不变。" },
  { text: "优先级规则：用户指令优先于本工具的通用框架。若用户指令与生成方式、相似度、变化强度、保持项、改变项或补充要求存在冲突，请以用户指令为准；只有在用户没有明确说明时，才使用本工具的默认框架约束。" },
  { text: "用户指令：{{batch.prompt}}" },
  { text: "批量版本：第 {{variantIndex}} 张。每张都要在细节、元素组合、局部设计、构图节奏或风格表达上形成可见差异，避免与同批结果重复。" },
  { when: "copyAllowed", text: "如确实需要文案，仅使用少量{{language}}短文案，文字必须清晰、准确、少而有力。" },
  { when: "copyForbidden", text: "画面中不要新增文字、数字、二维码、水印和无关 LOGO；参考图里的水印、角标、平台标识和第三方品牌不要复制。" },
  { text: "输出要求：真实商业摄影或高质量电商视觉质感，主体清楚，边缘干净，光影自然，材质可信，构图适合移动端电商浏览。" },
  { text: "禁止：低清晰度、畸形、拼接痕迹、错位结构、混乱文字、错误商标、二维码、水印、满屏无关装饰、侵犯第三方品牌或直接照搬参考图。" },
  { when: "hasExtra", text: "补充要求：{{extraPrompt}}" },
];

function lookup(vars: Record<string, unknown>, path: string): string {
  let current: unknown = vars;
  for (const part of path.split(".")) {
    if (current === null || typeof current !== "object") return "";
    current = (current as Record<string, unknown>)[part];
  }
  if (typeof current === "string") return current;
  if (typeof current === "number") return String(current);
  return "";
}

function interpolate(text: string, vars: Record<string, unknown>): string {
  return text.replace(/\{\{([^}]+)\}\}/g, (_, expr: string) => {
    const [path, fallback] = expr.split("||").map(s => s.trim());
    const value = lookup(vars, path);
    return value || fallback || "";
  });
}

function renderLines(lines: TemplateLine[], vars: Record<string, unknown>): string {
  const out: string[] = [];
  for (const line of lines) {
    if (line.when && !vars[line.when]) continue;
    const text = interpolate(line.text, vars);
    if (text.trim() !== "") out.push(text);
  }
  return out.join("\n");
}

export type EcomFormState = {
  productName: string; sellingPoints: string; audience: string; scenario: string;
  platform: string; language: string; visualStyle: string; customStyle: string;
  extraPrompt: string; copyEnabled: boolean; hasAssets: boolean;
  batchPrompt: string; batchStrategy: string; batchSimilarity: string; batchVariation: string;
  batchKeepKeys: string[]; batchChangeKeys: string[];
};

function promptVars(form: EcomFormState): Record<string, unknown> {
  const visualStyle = form.visualStyle === "自定义" ? form.customStyle.trim() : form.visualStyle;
  const copyAllowed = form.language !== "无文字" && form.copyEnabled;
  return {
    productName: form.productName.trim(), sellingPoints: form.sellingPoints.trim(),
    audience: form.audience.trim(), scenario: form.scenario.trim(),
    platform: form.platform, language: form.language,
    visualStyle: visualStyle.trim(), extraPrompt: form.extraPrompt.trim(),
    copyAllowed, copyForbidden: !copyAllowed,
    hasAssets: form.hasAssets, noAssets: !form.hasAssets,
    hasExtra: form.extraPrompt.trim() !== "",
    hasProductName: form.productName.trim() !== "",
    hasSellingPoints: form.sellingPoints.trim() !== "",
    hasAudience: form.audience.trim() !== "",
    hasScenario: form.scenario.trim() !== "",
  };
}

export function renderNormalPrompt(form: EcomFormState, module: EcomModule, variantIndex: number): string {
  return renderLines(NORMAL_LINES, {
    ...promptVars(form),
    module: { channel: module.channel, label: module.label, prompt: module.prompt, goal: module.goal },
    variantIndex,
  });
}

function keyLabels(keys: string[], catalog: EcomBatchKey[]): string {
  const labels = keys.map(k => catalog.find(e => e.key === k)?.label).filter(Boolean);
  return labels.join("、");
}

export function renderBatchPrompt(form: EcomFormState, variantIndex: number): string {
  return renderLines(BATCH_LINES, {
    ...promptVars(form),
    batch: {
      strategy: form.batchStrategy, similarity: form.batchSimilarity,
      variation: form.batchVariation, prompt: form.batchPrompt.trim(),
      keepLabels: keyLabels(form.batchKeepKeys, ECOM_BATCH.keepKeys),
      changeLabels: keyLabels(form.batchChangeKeys, ECOM_BATCH.changeKeys),
    },
    variantIndex,
  });
}

export function buildRefinePrompt(instruction: string): string {
  const text = instruction.trim() || ECOM_REFINE.defaultInstruction;
  return ECOM_REFINE.prefix + text;
}

export function resolveEcomSize(ratio: string, resolution: string): string {
  if (/^\d+x\d+$/i.test(ratio)) return ratio;
  const tier = ECOM_SIZE_MAP[resolution] ? resolution : "1K";
  return ECOM_SIZE_MAP[tier][ratio] || ECOM_SIZE_MAP["1K"][ratio] || "1024x1024";
}

// ---------- 计划构建（与 worker ecomBuildState 同构） ----------

export type EcomPlanItem = {
  id: string; moduleId: string; label: string; ratio: string; prompt: string;
};

export function moduleById(id: string): EcomModule | undefined {
  return ECOM_MODULES.find(m => m.id === id);
}

export function suiteById(id: string): EcomSuite | undefined {
  return ECOM_SUITES.find(s => s.id === id);
}

export function resolveModules(selectedModuleIds: string[], selectedSuite: string): EcomModule[] {
  const ids = selectedModuleIds.length ? selectedModuleIds : (suiteById(selectedSuite) || ECOM_SUITES[0]).moduleIds;
  const seen = new Set<string>();
  const modules: EcomModule[] = [];
  for (const id of ids) {
    if (seen.has(id)) continue;
    const m = moduleById(id);
    if (m) { modules.push(m); seen.add(id); }
  }
  return modules;
}

export function buildPlan(
  form: EcomFormState, mode: string, opts: {
    selectedModuleIds: string[]; selectedSuite: string; ratio: string; countPerModule: number; batchCount: number;
  },
): EcomPlanItem[] {
  const items: EcomPlanItem[] = [];
  if (mode === "batch") {
    const count = Math.min(Math.max(opts.batchCount || 5, 1), ECOM_LIMITS.maxPlanItems);
    const batchModule = ECOM_MODULES[ECOM_MODULES.length - 1];
    for (let i = 1; i <= count; i++) {
      items.push({
        id: `batch-${String(i).padStart(3, "0")}`, moduleId: batchModule.id,
        label: `参考批量 ${String(i).padStart(3, "0")}`,
        ratio: opts.ratio || batchModule.defaultRatio,
        prompt: renderBatchPrompt(form, i),
      });
    }
    return items;
  }
  const modules = resolveModules(opts.selectedModuleIds, opts.selectedSuite);
  const count = Math.min(Math.max(opts.countPerModule || 1, 1), ECOM_LIMITS.maxCountPerModule);
  for (const mod of modules) {
    for (let v = 1; v <= count; v++) {
      items.push({
        id: `${mod.id}-${v}`, moduleId: mod.id,
        label: count > 1 ? `${mod.label} ${v}` : mod.label,
        ratio: opts.ratio || mod.defaultRatio,
        prompt: renderNormalPrompt(form, mod, v),
      });
    }
  }
  return items.slice(0, ECOM_LIMITS.maxPlanItems);
}

export const ECOM_STATUS_LABELS: Record<string, string> = {
  queued: "排队", generating: "生成中", success: "完成", failed: "失败", stopped: "已停止",
};
export const ECOM_BATCH_STATUS_LABELS: Record<string, string> = {
  running: "进行中", completed: "已完成", failed: "有失败", stopped: "已停止",
};

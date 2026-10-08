#!/usr/bin/env node

/**
 * Build a reproducible coverage report for the inspiration gallery.
 * Node.js 20+ is required for the built-in fetch implementation.
 */

const fs = require("node:fs");
const path = require("node:path");

const DEFAULT_REFERENCE_URL = "https://raw.githubusercontent.com/freestylefly/awesome-gpt-image-2/main/data/cases.json";
const DEFAULT_TIMEOUT_MS = 30_000;
const PROMPT_LANGUAGES = ["zh", "latin", "ja", "ko", "other"];
const LANGUAGE_LABELS = {
  zh: "中文",
  latin: "英文及拉丁语系",
  ja: "日文",
  ko: "韩文",
  other: "其他",
};

function parseArgs(argv) {
  const args = {
    referenceUrl: process.env.GALLERY_REFERENCE_URL || DEFAULT_REFERENCE_URL,
    communityUrl: process.env.GALLERY_COMMUNITY_URL || "",
    out: process.env.GALLERY_COVERAGE_REPORT || "docs/inspiration-gallery-coverage-report.md",
    jsonOut: process.env.GALLERY_COVERAGE_JSON || "docs/inspiration-gallery-coverage-report.json",
    pairTarget: 3,
  };
  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index];
    if (value === "--reference-url") args.referenceUrl = argv[++index];
    else if (value === "--community-url") args.communityUrl = argv[++index];
    else if (value === "--out") args.out = argv[++index];
    else if (value === "--json-out") args.jsonOut = argv[++index];
    else if (value === "--pair-target") args.pairTarget = Math.max(1, Number(argv[++index]) || 3);
    else if (value === "--help" || value === "-h") {
      console.log(`Usage: node scripts/analyze-gallery-coverage.js [options]

Options:
  --reference-url <url>  Reference manifest URL (default: awesome-gpt-image-2)
  --community-url <url>  Public gallery API URL, fetched page by page
  --out <path>           Write Markdown report
  --json-out <path>      Write machine-readable summary JSON
  --pair-target <n>      Minimum target for important tag pairs (default: 3)
`);
      process.exit(0);
    }
  }
  return args;
}

async function fetchJSON(url, label) {
  const response = await fetch(url, { signal: AbortSignal.timeout(DEFAULT_TIMEOUT_MS), headers: { accept: "application/json" } });
  if (!response.ok) throw new Error(`${label} returned HTTP ${response.status}`);
  return response.json();
}

function unwrapAPIResponse(value) {
  if (value && typeof value === "object" && value.data && typeof value.data === "object") return value.data;
  return value;
}

async function fetchCommunityItems(baseUrl) {
  if (!baseUrl) return { status: "not_requested", items: [], total: 0 };
  const items = [];
  let page = 1;
  let reportedTotal = null;
  while (page <= 100) {
    const url = new URL(baseUrl);
    url.searchParams.set("page", String(page));
    url.searchParams.set("page_size", "60");
    const payload = unwrapAPIResponse(await fetchJSON(url, "community gallery API"));
    const pageItems = Array.isArray(payload?.items) ? payload.items : [];
    reportedTotal = Number.isFinite(Number(payload?.total)) ? Number(payload.total) : reportedTotal;
    items.push(...pageItems);
    if (pageItems.length === 0 || pageItems.length < 60 || (reportedTotal !== null && items.length >= reportedTotal)) break;
    page += 1;
  }
  return { status: "ok", items, total: reportedTotal ?? items.length };
}

function detectPromptLanguage(value) {
  const text = String(value || "");
  if (/[\u3040-\u30ff]/u.test(text)) return "ja";
  if (/[\uac00-\ud7af]/u.test(text)) return "ko";
  if (/[\u3400-\u9fff]/u.test(text)) return "zh";
  if (/[A-Za-zÀ-ÖØ-öø-ÿ]/u.test(text)) return "latin";
  return "other";
}

function asArray(value) {
  return Array.isArray(value) ? value.filter((item) => typeof item === "string" && item.trim()) : [];
}

function normalizeReferenceCase(item) {
  return {
    id: String(item?.id ?? ""),
    title: String(item?.title || ""),
    prompt: String(item?.prompt || ""),
    category: String(item?.category || "未标注"),
    styles: asArray(item?.styles),
    scenes: asArray(item?.scenes),
    promptLanguage: detectPromptLanguage(item?.prompt),
  };
}

function normalizeCommunityItem(item) {
  return {
    id: String(item?.public_id || ""),
    title: String(item?.title || ""),
    prompt: String(item?.prompt || ""),
    tags: asArray(item?.tags),
    promptLanguage: detectPromptLanguage(item?.prompt || item?.title),
  };
}

function countValues(items, readValues) {
  const counts = new Map();
  for (const item of items) {
    for (const value of readValues(item)) counts.set(value, (counts.get(value) || 0) + 1);
  }
  return [...counts.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
}

function countSingleValues(items, readValue, values) {
  const counts = new Map(values.map((value) => [value, 0]));
  for (const item of items) {
    const value = readValue(item);
    counts.set(value, (counts.get(value) || 0) + 1);
  }
  return [...counts.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
}

function countPairs(items, readLeft, readRight) {
  const counts = new Map();
  for (const item of items) {
    const leftValues = readLeft(item);
    const rightValues = readRight(item);
    for (const left of leftValues) {
      for (const right of rightValues) {
        const key = `${left}\u0000${right}`;
        counts.set(key, (counts.get(key) || 0) + 1);
      }
    }
  }
  return counts;
}

function allPairs(leftValues, rightValues, counts) {
  const pairs = [];
  for (const left of leftValues) {
    for (const right of rightValues) {
      pairs.push({ left, right, count: counts.get(`${left}\u0000${right}`) || 0 });
    }
  }
  return pairs;
}

function dimensionReport(items, values, readValue, target) {
  return values.map((value) => {
    const count = items.reduce((total, item) => total + (readValue(item) === value ? 1 : 0), 0);
    return { value, count, target, gap: Math.max(0, target - count) };
  });
}

function multiDimensionReport(items, values, readValues, target) {
  const counts = new Map(countValues(items, readValues));
  return values.map((value) => {
    const count = counts.get(value) || 0;
    return { value, count, target, gap: Math.max(0, target - count) };
  });
}

function pairReport(items, leftValues, rightValues, readLeft, readRight, target) {
  const pairs = allPairs(leftValues, rightValues, countPairs(items, readLeft, readRight));
  const zero = pairs.filter((pair) => pair.count === 0);
  const belowTarget = pairs.filter((pair) => pair.count < target);
  const urgent = pairs
    .filter((pair) => pair.count < target)
    .sort((a, b) => a.count - b.count || a.left.localeCompare(b.left) || a.right.localeCompare(b.right))
    .slice(0, 20);
  return {
    total: pairs.length,
    present: pairs.length - zero.length,
    zero: zero.length,
    belowTarget: belowTarget.length,
    pairs,
    urgent,
  };
}

function qualityReport(items) {
  const missing = {
    title: items.filter((item) => !item.title).length,
    prompt: items.filter((item) => !item.prompt).length,
    category: items.filter((item) => !item.category || item.category === "未标注").length,
    styles: items.filter((item) => item.styles.length === 0).length,
    scenes: items.filter((item) => item.scenes.length === 0).length,
  };
  const seen = new Map();
  for (const item of items) {
    const key = `${item.title.trim().toLocaleLowerCase()}\u0000${item.prompt.trim().toLocaleLowerCase()}`;
    if (key !== "\u0000") seen.set(key, (seen.get(key) || 0) + 1);
  }
  const duplicateGroups = [...seen.values()].filter((count) => count > 1);
  return { missing, duplicateGroups: duplicateGroups.length, duplicateItems: duplicateGroups.reduce((sum, count) => sum + count, 0) };
}

function formatNumber(value) {
  return new Intl.NumberFormat("en-US").format(value);
}

function markdownTable(headers, rows) {
  const lines = [`| ${headers.join(" | ")} |`, `| ${headers.map(() => "---").join(" | ")} |`];
  for (const row of rows) lines.push(`| ${row.join(" | ")} |`);
  return lines.join("\n");
}

function percentage(part, total) {
  if (!total) return "0.0%";
  return `${((part / total) * 100).toFixed(1)}%`;
}

function pairMarkdown(title, report, target) {
  const rows = report.urgent.map((pair) => [pair.left, pair.right, formatNumber(pair.count), formatNumber(Math.max(0, target - pair.count))]);
  return `### ${title}

- 可能组合：${formatNumber(report.total)}
- 已有案例的组合：${formatNumber(report.present)}（${percentage(report.present, report.total)}）
- 零案例组合：${formatNumber(report.zero)}
- 未达到目标组合：${formatNumber(report.belowTarget)}

最需要优先关注的组合（按案例数从少到多，最多展示 20 个）：

${markdownTable(["左侧标签", "右侧标签", "案例数", "距离目标"], rows)}
`;
}

function buildMarkdown(result) {
  const { snapshotAt, sources, reference, community, targets } = result;
  const categoryRows = reference.categories.map((item) => [item.value, formatNumber(item.count), formatNumber(item.target), formatNumber(item.gap)]);
  const styleRows = reference.styles.map((item) => [item.value, formatNumber(item.count), formatNumber(item.target), formatNumber(item.gap)]);
  const sceneRows = reference.scenes.map((item) => [item.value, formatNumber(item.count), formatNumber(item.target), formatNumber(item.gap)]);
  const languageRows = reference.promptLanguages.map((item) => [LANGUAGE_LABELS[item.value] || item.value, item.value, formatNumber(item.count), "仅代表 Prompt 文本"]);
  const communityTagRows = community.tags.map((item) => [item.value, formatNumber(item.count)]);
  const communityLanguageRows = community.promptLanguages.map((item) => [LANGUAGE_LABELS[item.value] || item.value, item.value, formatNumber(item.count)]);
  const communityResultLabel = community.status === "ok" ? `${formatNumber(community.total)} 个` : community.status === "not_requested" ? "未请求" : "不可用";
  const warnings = [
    ...(reference.manifestTotal !== reference.total ? [`来源 manifest 声明 ${reference.manifestTotal} 个案例，但实际读取到 ${reference.total} 个。`] : []),
    ...(community.status === "unavailable" ? [`社区 API 不可用：${community.error}`] : []),
    "图片文字语言尚未通过 OCR 统计；当前语言覆盖仅依据 Prompt/标题文本检测。",
    "参考案例与社区标签体系语义不同，本报告不把两者直接相加作为统一覆盖率。",
  ];
  return `# 灵感广场覆盖率与缺口报告

> 快照时间：${snapshotAt}
>
> 本报告由 \`scripts/analyze-gallery-coverage.js\` 生成。重复运行脚本可以刷新数据，不应手工修改统计数字。

## 1. 数据源与范围

${markdownTable(["数据集", "地址", "读取结果"], [
  ["参考案例", `\`${sources.reference}\``, `${formatNumber(reference.total)} 个`],
  ["社区案例", sources.community ? `\`${sources.community}\`` : "未请求", communityResultLabel],
])}

当前采用的覆盖目标：一级分类、风格、场景每项至少 **${targets.dimension}** 个案例；重点标签组合每项至少 **${targets.pair}** 个案例。目标是运营优先级，不代表所有标签组合都必须生成。

## 2. 摘要

- 参考案例：**${formatNumber(reference.total)}** 个，${formatNumber(reference.categories.length)} 个分类，${formatNumber(reference.styles.length)} 个风格，${formatNumber(reference.scenes.length)} 个场景。
- Prompt 文本语言：${reference.promptLanguages.map((item) => `${LANGUAGE_LABELS[item.value] || item.value} ${formatNumber(item.count)}`).join("、")}。
- 参考案例分类中未达 ${targets.dimension} 个的标签：**${reference.categories.filter((item) => item.gap > 0).length}** 个；风格：**${reference.styles.filter((item) => item.gap > 0).length}** 个；场景：**${reference.scenes.filter((item) => item.gap > 0).length}** 个。
- 社区案例：**${community.status === "ok" ? formatNumber(community.total) : "未完成统计"}**，当前仍使用旧版自由标签，尚未形成可与参考案例对齐的分类/风格/场景统计。
- 本次没有根据缺口自动生成或发布任何图片。

## 3. 参考案例单维覆盖

### 分类

${markdownTable(["分类", "当前案例数", `目标（${targets.dimension}）`, "缺口"], categoryRows)}

### 风格

${markdownTable(["风格", "当前案例数", `目标（${targets.dimension}）`, "缺口"], styleRows)}

### 场景

${markdownTable(["场景", "当前案例数", `目标（${targets.dimension}）`, "缺口"], sceneRows)}

## 4. Prompt 语言覆盖

${markdownTable(["展示名", "检测值", "案例数", "说明"], languageRows)}

检测规则与当前 Web 的 \`detectGalleryLanguage\` 保持一致：日文、韩文、中文、拉丁字符依次检测。一个 Prompt 如果混有多种文字，当前实现只计为一个主检测结果。

**重要限制：** 本报告没有把图片中的实际文字语言当作已知事实。要补齐“图片文字语言”维度，必须在后续同步/审核阶段接入 OCR，并将 OCR 结果和置信度写入案例数据。

## 5. 重点组合缺口

目标组合数为每项 ${targets.pair} 个。零案例或低于目标不等于必须生成：应先确认组合是否有用户价值、是否存在标签定义冲突，以及是否可以通过相近案例满足需求。

${pairMarkdown("分类 + 场景", reference.pairs.categoryScene, targets.pair)}

${pairMarkdown("分类 + 风格", reference.pairs.categoryStyle, targets.pair)}

${pairMarkdown("场景 + Prompt 语言", reference.pairs.sceneLanguage, targets.pair)}

## 6. 社区案例现状

### 自由标签

${communityTagRows.length ? markdownTable(["标签", "案例数"], communityTagRows) : "暂无社区标签数据。"}

### Prompt/标题文本语言

${communityLanguageRows.length ? markdownTable(["展示名", "检测值", "案例数"], communityLanguageRows) : "暂无社区案例数据。"}

${community.status === "ok" ? `社区案例目前有 ${formatNumber(community.total)} 个公开种子案例，` : "本次没有完成社区案例 API 统计，"}且标签 \`illustration\`、\`photography\`、\`concept\`、\`ecommerce\`、\`anime\` 与参考案例的分类/风格/场景不是同一套枚举。下一步不应直接把这些字符串拼在一起计算总覆盖率。

## 7. 数据质量信号

${markdownTable(["检查项", "结果"], [
  ["参考案例缺标题", formatNumber(reference.quality.missing.title)],
  ["参考案例缺 Prompt", formatNumber(reference.quality.missing.prompt)],
  ["参考案例缺分类", formatNumber(reference.quality.missing.category)],
  ["参考案例缺风格", formatNumber(reference.quality.missing.styles)],
  ["参考案例缺场景", formatNumber(reference.quality.missing.scenes)],
  ["参考案例重复标题 + Prompt 组", formatNumber(reference.quality.duplicateGroups)],
  ["重复组涉及案例数", formatNumber(reference.quality.duplicateItems)],
])}

${warnings.map((warning) => `- ${warning}`).join("\n")}

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
`;
}

function buildResult(manifest, communityResult, args) {
  const referenceItems = (Array.isArray(manifest?.cases) ? manifest.cases : []).map(normalizeReferenceCase);
  const categories = asArray(manifest?.categories);
  const styles = asArray(manifest?.styles);
  const scenes = asArray(manifest?.scenes);
  const dimensionTarget = 8;
  const pairTarget = args.pairTarget;
  const communityItems = communityResult.items.map(normalizeCommunityItem);
  const communityTags = countValues(communityItems, (item) => item.tags).map(([value, count]) => ({ value, count }));
  const communityPromptLanguages = countSingleValues(communityItems, (item) => item.promptLanguage, PROMPT_LANGUAGES).filter((item) => item[1] > 0).map(([value, count]) => ({ value, count }));
  return {
    snapshotAt: new Date().toISOString(),
    sources: { reference: args.referenceUrl, community: args.communityUrl },
    targets: { dimension: dimensionTarget, pair: pairTarget },
    reference: {
      manifestTotal: Number(manifest?.totalCases) || referenceItems.length,
      total: referenceItems.length,
      categories: multiDimensionReport(referenceItems, categories, (item) => [item.category], dimensionTarget),
      styles: multiDimensionReport(referenceItems, styles, (item) => item.styles, dimensionTarget),
      scenes: multiDimensionReport(referenceItems, scenes, (item) => item.scenes, dimensionTarget),
      promptLanguages: countSingleValues(referenceItems, (item) => item.promptLanguage, PROMPT_LANGUAGES).filter((item) => item[1] > 0).map(([value, count]) => ({ value, count })),
      pairs: {
        categoryScene: pairReport(referenceItems, categories, scenes, (item) => [item.category], (item) => item.scenes, pairTarget),
        categoryStyle: pairReport(referenceItems, categories, styles, (item) => [item.category], (item) => item.styles, pairTarget),
        sceneLanguage: pairReport(referenceItems, scenes, PROMPT_LANGUAGES, (item) => item.scenes, (item) => [item.promptLanguage], pairTarget),
      },
      quality: qualityReport(referenceItems),
    },
    community: {
      status: communityResult.status,
      error: communityResult.error || "",
      total: communityResult.total,
      tags: communityTags,
      promptLanguages: communityPromptLanguages,
    },
  };
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (!args.out && !args.jsonOut) throw new Error("At least one of --out or --json-out is required");
  const manifest = await fetchJSON(args.referenceUrl, "reference manifest");
  let communityResult;
  try {
    communityResult = await fetchCommunityItems(args.communityUrl);
  } catch (error) {
    communityResult = { status: "unavailable", items: [], total: 0, error: error.message };
  }
  const result = buildResult(manifest, communityResult, args);
  if (args.out) {
    fs.mkdirSync(path.dirname(path.resolve(args.out)), { recursive: true });
    fs.writeFileSync(args.out, buildMarkdown(result), "utf8");
  }
  if (args.jsonOut) {
    fs.mkdirSync(path.dirname(path.resolve(args.jsonOut)), { recursive: true });
    fs.writeFileSync(args.jsonOut, `${JSON.stringify(result, null, 2)}\n`, "utf8");
  }
  console.log(JSON.stringify({
    report: args.out || null,
    json: args.jsonOut || null,
    referenceCases: result.reference.total,
    communityCases: result.community.total,
    communityStatus: result.community.status,
    categoryZeroPairs: result.reference.pairs.categoryScene.zero,
    categoryStyleZeroPairs: result.reference.pairs.categoryStyle.zero,
    sceneLanguageZeroPairs: result.reference.pairs.sceneLanguage.zero,
  }, null, 2));
}

main().catch((error) => {
  console.error(`[gallery coverage] ${error.message}`);
  process.exitCode = 1;
});

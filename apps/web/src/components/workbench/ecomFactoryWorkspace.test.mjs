import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import ts from "typescript";

// 与 Go 端 ecom_factory_test.go 共用同一份参考站（qqwapi.cn）登录态实测剪贴板文本，
// 双端 golden 一致 ⇒ 前后端提示词逐字同源。
const source = readFileSync(new URL("../../lib/ecomFactory.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
const module = { exports: {} };
vm.runInNewContext(compiled, { module, exports: module.exports });
const lib = module.exports;

const goldenForm = {
  productName: "LA-2", sellingPoints: "", audience: "", scenario: "",
  platform: "抖音电商", language: "简体中文", visualStyle: "高端商业摄影", customStyle: "",
  extraPrompt: "", copyEnabled: true, hasAssets: true,
  batchPrompt: "", batchStrategy: "保持主体改局部", batchSimilarity: "50%", batchVariation: "中变化",
  batchKeepKeys: ["subject", "composition", "lighting"], batchChangeKeys: ["local-pattern", "style"],
};

const goldenDetailCover = `你是资深电商视觉设计师，请生成一张可直接用于详情页的详情页首屏。
商品：LA-2
核心卖点：请从商品外观和使用场景中提炼核心卖点
目标人群：目标电商用户
使用场景：真实购买与使用场景
目标平台：抖音电商
视觉风格：高端商业摄影
画面任务：生成电商详情页首屏图。画面纵向排版，商品主体和关键卖点层次清楚，有高级商业海报质感，适合放在详情页顶部承接转化。
商业目标：首屏完成商品、利益点和质感表达。
版本方向：第 1 张，构图和道具可以有轻微差异，但商品身份必须一致。
严格参考已上传图片中的商品身份、外形、比例、材质、颜色和关键结构；参考图只用于构图、光影、风格和场景方向，不复制水印、人物身份或第三方品牌。
可以加入少量简体中文短文案，文案必须清晰、无错别字、少而有力；不确定的文字区域请留白。
输出要求：真实商业摄影质感，主体清楚，边缘干净，光影自然，材质可信，构图适合移动端电商浏览。
禁止：低清晰度、变形、错位手指、混乱文字、错误商标、过度磨皮、重复主体、拼接痕迹、侵权品牌元素。`;

const goldenTexture = `你是资深电商视觉设计师，请生成一张可直接用于详情模块的材质细节图。
商品：LA-2
核心卖点：请从商品外观和使用场景中提炼核心卖点
目标人群：目标电商用户
使用场景：真实购买与使用场景
目标平台：抖音电商
视觉风格：高端商业摄影
画面任务：生成商品材质细节图。使用近景或微距视角，突出纹理、工艺、接口、结构或手感，保留真实比例和材质，背景简洁，光线能展示细节层次。
商业目标：突出材质、工艺、纹理、接口或细节优势。
版本方向：第 1 张，构图和道具可以有轻微差异，但商品身份必须一致。
严格参考已上传图片中的商品身份、外形、比例、材质、颜色和关键结构；参考图只用于构图、光影、风格和场景方向，不复制水印、人物身份或第三方品牌。
可以加入少量简体中文短文案，文案必须清晰、无错别字、少而有力；不确定的文字区域请留白。
输出要求：真实商业摄影质感，主体清楚，边缘干净，光影自然，材质可信，构图适合移动端电商浏览。
禁止：低清晰度、变形、错位手指、混乱文字、错误商标、过度磨皮、重复主体、拼接痕迹、侵权品牌元素。`;

test("normal prompt matches reference site golden text verbatim", () => {
  const detailCover = lib.ECOM_MODULES.find(m => m.id === "detail-cover");
  const texture = lib.ECOM_MODULES.find(m => m.id === "texture-close");
  assert.equal(lib.renderNormalPrompt(goldenForm, detailCover, 1), goldenDetailCover);
  assert.equal(lib.renderNormalPrompt(goldenForm, texture, 1), goldenTexture);
});

test("normal prompt branches: no assets / copy off / extra", () => {
  const module = lib.ECOM_MODULES.find(m => m.id === "main-square");
  const noAssets = lib.renderNormalPrompt({ ...goldenForm, hasAssets: false }, module, 2);
  assert.ok(noAssets.includes("无参考图时，根据商品信息生成可信的商业摄影画面"));
  assert.ok(!noAssets.includes("严格参考已上传图片"));
  assert.ok(noAssets.includes("版本方向：第 2 张"));

  const copyOff = lib.renderNormalPrompt({ ...goldenForm, copyEnabled: false }, module, 1);
  assert.ok(copyOff.includes("画面中不要新增文字、数字、二维码、水印和无关 LOGO"));
  assert.ok(!copyOff.includes("可以加入少量"));

  const noTextLang = lib.renderNormalPrompt({ ...goldenForm, language: "无文字" }, module, 1);
  assert.ok(!noTextLang.includes("可以加入少量"));

  const extra = lib.renderNormalPrompt({ ...goldenForm, extraPrompt: "背景保持浅色" }, module, 1);
  assert.ok(extra.includes("补充要求：背景保持浅色"));
});

test("batch prompt renders framework and user priority lines", () => {
  const form = { ...goldenForm, batchPrompt: "把杯身图案换成夏日水果插画" };
  const got = lib.renderBatchPrompt(form, 3);
  for (const want of [
    "参考图使用规则：第一张上传图作为主要视觉参考",
    "生成方式：保持主体改局部",
    "必须尽量保持：主体、构图、光影",
    "允许重点改变：局部图案、风格。",
    "优先级规则：用户指令优先于本工具的通用框架。",
    "用户指令：把杯身图案换成夏日水果插画",
    "批量版本：第 3 张。",
  ]) {
    assert.ok(got.includes(want), `missing: ${want}`);
  }
  assert.ok(!got.includes("目标人群："));
});

test("refine prompt uses fixed prefix and default instruction", () => {
  assert.equal(
    lib.buildRefinePrompt("把背景换成浅灰色"),
    lib.ECOM_REFINE.prefix + "把背景换成浅灰色",
  );
  assert.equal(lib.buildRefinePrompt(""), lib.ECOM_REFINE.prefix + lib.ECOM_REFINE.defaultInstruction);
});

test("plan builder matches reference-site item composition", () => {
  const plan = lib.buildPlan(goldenForm, "suite", {
    selectedModuleIds: [], selectedSuite: "detail", ratio: "1:1", countPerModule: 1, batchCount: 20,
  });
  assert.equal(plan.length, 5);
  assert.equal(plan[0].id, "detail-cover-1");
  assert.equal(plan[0].prompt, goldenDetailCover);

  const batch = lib.buildPlan(goldenForm, "batch", {
    selectedModuleIds: [], selectedSuite: "batch", ratio: "1:1", countPerModule: 1, batchCount: 200,
  });
  assert.equal(batch.length, 24);
  assert.equal(batch[0].label, "参考批量 001");

  const multi = lib.buildPlan(goldenForm, "suite", {
    selectedModuleIds: ["main-square"], selectedSuite: "launch", ratio: "1:1", countPerModule: 3, batchCount: 20,
  });
  assert.equal(multi.length, 3);
  assert.equal(multi[1].label, "平台主图 2");
});

test("size map resolves reference-site pixel sizes", () => {
  assert.equal(lib.resolveEcomSize("1:1", "1K"), "1024x1024");
  assert.equal(lib.resolveEcomSize("3:4", "2K"), "1536x2048");
  assert.equal(lib.resolveEcomSize("1:1", "4K"), "1024x1024");
  assert.equal(lib.resolveEcomSize("2048x2048", "1K"), "2048x2048");
});

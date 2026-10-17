package main

import (
	_ "embed"
	"strings"
	"testing"
)

// 参考站（qqwapi.cn 电商图片工厂）登录态实测剪贴板提示词，逐字对拍。
// form 快照来自历史记录 LA-2 · 详情页套装 · 抖音电商 · 2K（5 张商品图、无风格参考、
// 商品名称 LA-2、其余信息为空、简体中文、允许运营文案、高端商业摄影）。
var ecomGoldenForm = ecomForm{
	ProductName: "LA-2",
	Platform:    "抖音电商",
	Language:    "简体中文",
	VisualStyle: "高端商业摄影",
	Resolution:  "2K",
	Suite:       "detail",
	CopyEnabled: true,
	HasAssets:   true,
}

const ecomGoldenDetailCover = `你是资深电商视觉设计师，请生成一张可直接用于详情页的详情页首屏。
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
禁止：低清晰度、变形、错位手指、混乱文字、错误商标、过度磨皮、重复主体、拼接痕迹、侵权品牌元素。`

const ecomGoldenTexture = `你是资深电商视觉设计师，请生成一张可直接用于详情模块的材质细节图。
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
禁止：低清晰度、变形、错位手指、混乱文字、错误商标、过度磨皮、重复主体、拼接痕迹、侵权品牌元素。`

func TestRenderEcomNormalPromptGolden(t *testing.T) {
	catalog, err := loadEcomCatalog()
	if err != nil {
		t.Fatalf("加载目录失败: %v", err)
	}
	cases := []struct {
		moduleID string
		golden   string
	}{
		{"detail-cover", ecomGoldenDetailCover},
		{"texture-close", ecomGoldenTexture},
	}
	for _, tc := range cases {
		module, ok := catalog.ModuleByID(tc.moduleID)
		if !ok {
			t.Fatalf("模块缺失: %s", tc.moduleID)
		}
		got := renderEcomNormalPrompt(catalog, ecomGoldenForm, module, 1)
		if got != tc.golden {
			t.Errorf("模块 %s 提示词与参考站实测不一致\ngot:\n%s\nwant:\n%s", tc.moduleID, got, tc.golden)
		}
	}
}

func TestRenderEcomNormalPromptBranches(t *testing.T) {
	catalog, _ := loadEcomCatalog()
	module, _ := catalog.ModuleByID("main-square")
	noAssets := ecomGoldenForm
	noAssets.HasAssets = false
	got := renderEcomNormalPrompt(catalog, noAssets, module, 2)
	if strings.Contains(got, "严格参考已上传图片") {
		t.Error("无资产时不应包含参考图行")
	}
	if !strings.Contains(got, "无参考图时，根据商品信息生成可信的商业摄影画面") {
		t.Error("无资产时应包含无参考图行")
	}
	if !strings.Contains(got, "版本方向：第 2 张") {
		t.Error("应渲染版本序号 2")
	}

	noCopy := ecomGoldenForm
	noCopy.CopyEnabled = false
	got = renderEcomNormalPrompt(catalog, noCopy, module, 1)
	if !strings.Contains(got, "画面中不要新增文字、数字、二维码、水印和无关 LOGO") {
		t.Error("关闭文案时应包含禁止文字行")
	}
	if strings.Contains(got, "可以加入少量") {
		t.Error("关闭文案时不应包含允许文案行")
	}

	noText := ecomGoldenForm
	noText.Language = "无文字"
	got = renderEcomNormalPrompt(catalog, noText, module, 1)
	if strings.Contains(got, "可以加入少量") {
		t.Error("语言=无文字时不应包含允许文案行")
	}

	withExtra := ecomGoldenForm
	withExtra.ExtraPrompt = "背景保持浅色"
	got = renderEcomNormalPrompt(catalog, withExtra, module, 1)
	if !strings.Contains(got, "补充要求：背景保持浅色") {
		t.Error("补充要求应追加末行")
	}

	emptyName := ecomGoldenForm
	emptyName.ProductName = ""
	got = renderEcomNormalPrompt(catalog, emptyName, module, 1)
	if !strings.Contains(got, "商品：上传参考图中的商品") {
		t.Error("空商品名应使用兜底文案")
	}
}

func TestRenderEcomBatchPrompt(t *testing.T) {
	catalog, _ := loadEcomCatalog()
	form := ecomGoldenForm
	form.BatchPrompt = "把杯身图案换成夏日水果插画"
	form.BatchStrategy = "保持主体改局部"
	form.BatchSimilarity = "50%"
	form.BatchVariation = "中变化"
	form.BatchKeepKeys = []string{"subject", "composition", "lighting"}
	form.BatchChangeKeys = []string{"local-pattern", "style"}
	got := renderEcomBatchPrompt(catalog, form, 3)
	required := []string{
		"你是资深电商视觉设计师，请根据用户上传的一张或多张参考图，生成新的电商图片。",
		"参考图使用规则：第一张上传图作为主要视觉参考",
		"商品/项目：LA-2",
		"生成方式：保持主体改局部",
		"目标相似度：约 50%。",
		"变化强度：中变化",
		"必须尽量保持：主体、构图、光影",
		"允许重点改变：局部图案、风格。",
		"优先级规则：用户指令优先于本工具的通用框架。",
		"用户指令：把杯身图案换成夏日水果插画",
		"批量版本：第 3 张。",
		"禁止：低清晰度、畸形、拼接痕迹、错位结构、混乱文字、错误商标、二维码、水印、满屏无关装饰、侵犯第三方品牌或直接照搬参考图。",
	}
	for _, want := range required {
		if !strings.Contains(got, want) {
			t.Errorf("批量提示词缺少行: %s\ngot:\n%s", want, got)
		}
	}
	if strings.Contains(got, "目标人群：") {
		t.Error("人群为空时不应出现目标人群行")
	}
}

func TestEcomRefinePrompt(t *testing.T) {
	catalog, _ := loadEcomCatalog()
	instruction := "把背景换成浅灰色，商品角度微调"
	want := catalog.Templates.Refine.Prefix + instruction
	if want != "以参考图为基准进行精修，严格保持商品的外形、比例、材质、颜色、品牌标识与整体构图一致，仅按以下要求调整，其余部分保持原样：把背景换成浅灰色，商品角度微调" {
		t.Errorf("精修模板拼接异常: %s", want)
	}
}

func TestEcomResolveSize(t *testing.T) {
	catalog, _ := loadEcomCatalog()
	cases := map[[2]string]string{
		{"1:1", "1K"}:   "1024x1024",
		{"16:9", "1K"}:  "1536x864",
		{"3:4", "2K"}:   "1536x2048",
		{"1:1", "4K"}:   "1024x1024", // 缺档回退 1K
		{"9:16", "2K"}:  "1152x2048",
		{"2048x2048", "1K"}: "2048x2048", // WxH 透传
	}
	for input, want := range cases {
		if got := catalog.resolveEcomSize(input[0], input[1]); got != want {
			t.Errorf("resolveEcomSize(%s, %s) = %s, want %s", input[0], input[1], got, want)
		}
	}
}

func TestEcomCatalogIntegrity(t *testing.T) {
	catalog, err := loadEcomCatalog()
	if err != nil {
		t.Fatalf("加载目录失败: %v", err)
	}
	if len(catalog.Modules) != 14 {
		t.Errorf("模块数 = %d, want 14", len(catalog.Modules))
	}
	if len(catalog.Suites) != 5 {
		t.Errorf("套装数 = %d, want 5", len(catalog.Suites))
	}
	moduleIDs := map[string]bool{}
	for _, m := range catalog.Modules {
		moduleIDs[m.ID] = true
		if m.Label == "" || m.Channel == "" || m.Goal == "" || m.Prompt == "" || m.DefaultRatio == "" {
			t.Errorf("模块 %s 字段不完整", m.ID)
		}
	}
	for _, s := range catalog.Suites {
		for _, id := range s.ModuleIds {
			if !moduleIDs[id] {
				t.Errorf("套装 %s 引用未知模块 %s", s.ID, id)
			}
		}
	}
}

func TestEcomBuildState(t *testing.T) {
	catalog, _ := loadEcomCatalog()
	inputs := map[string]interface{}{
		"mode": "suite", "selectedSuite": "detail", "productName": "LA-2",
		"platform": "抖音电商", "resolution": "2K", "ratio": "1:1", "countPerModule": 1,
		"language": "简体中文", "visualStyle": "高端商业摄影", "copyEnabled": true,
		"assets": []interface{}{map[string]interface{}{"url": "http://x/1.jpg", "role": "product"}},
	}
	state, errMsg := ecomBuildState(catalog, inputs)
	if errMsg != "" || state == nil {
		t.Fatalf("构建失败: %s", errMsg)
	}
	if len(state.Items) != 5 {
		t.Fatalf("详情页套装应 5 项, got %d", len(state.Items))
	}
	if state.Items[0].ID != "detail-cover-1" || state.Items[0].Prompt != ecomGoldenDetailCover {
		t.Errorf("首项提示词与参考站不一致:\n%s", state.Items[0].Prompt)
	}
}

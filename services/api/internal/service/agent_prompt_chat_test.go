package service

import (
	"strings"
	"testing"
)

// 每个内置提示词智能体都必须是可直接使用的完整成品：
// 有明确的角色、工作步骤、固定的输出章节，以及"需要你补充"这一统一收尾章节。
var expectedPromptAgentCodes = []string{
	"video_script_writer",
	"ecom_sales_copy",
	"spoken_copy",
	"wechat_article",
	"moments_copy",
	"event_plan",
	"franchise_copy",
	"quotation_copy",
	"prompt_engineer",
	"visual_asset_extractor",
}

func TestPromptPacksCoverBuiltinAgents(t *testing.T) {
	codes := PromptAgentCodes()
	if len(codes) != len(expectedPromptAgentCodes) {
		t.Fatalf("提示词包数量 = %d，期望 %d", len(codes), len(expectedPromptAgentCodes))
	}
	for _, want := range expectedPromptAgentCodes {
		if _, ok := PromptPackByCode(want); !ok {
			t.Fatalf("缺少提示词包: %s", want)
		}
	}
}

func TestPromptPacksAreComplete(t *testing.T) {
	for _, code := range PromptAgentCodes() {
		pack, ok := PromptPackByCode(code)
		if !ok {
			t.Fatalf("提示词包读取失败: %s", code)
		}
		if pack.Code != code {
			t.Fatalf("%s: Code 字段不一致 = %s", code, pack.Code)
		}
		if pack.DefaultChatModelCode == "" {
			t.Fatalf("%s: 缺少默认对话模型", code)
		}
		prompt := pack.SystemPrompt
		if len([]rune(prompt)) < 200 {
			t.Fatalf("%s: 系统提示词过短（%d 字）", code, len([]rune(prompt)))
		}
		if strings.Count(prompt, "\n## ") < 5 {
			t.Fatalf("%s: 输出章节不足 5 个", code)
		}
		if !strings.Contains(prompt, "## 需要你补充") {
			t.Fatalf("%s: 缺少统一的「需要你补充」章节", code)
		}
		if !strings.Contains(prompt, "## 工作步骤") {
			t.Fatalf("%s: 缺少「工作步骤」章节", code)
		}
		// 反编造与合规底线：由通用输出纪律兜底，组合后必须存在。
		effective := composePromptAgentSystemPrompt(prompt)
		if !strings.Contains(effective, "严禁编造") {
			t.Fatalf("%s: 组合后的提示词缺少反编造约束", code)
		}
		if !strings.Contains(effective, "## 通用输出纪律") {
			t.Fatalf("%s: 组合后的提示词缺少通用输出纪律", code)
		}
	}
}

func TestComposePromptAgentSystemPromptAppendsRulesOnce(t *testing.T) {
	composed := composePromptAgentSystemPrompt("你是测试智能体。")
	if !strings.Contains(composed, "## 通用输出纪律") {
		t.Fatal("未追加通用输出纪律")
	}
	if strings.Count(composed, "## 通用输出纪律") != 1 {
		t.Fatal("通用输出纪律被重复追加")
	}
	again := composePromptAgentSystemPrompt(composed)
	if again != composed {
		t.Fatal("已包含通用输出纪律时不应再追加")
	}
	if got := composePromptAgentSystemPrompt("   "); got != "" {
		t.Fatalf("空提示词应返回空字符串，得到 %q", got)
	}
}

func TestValidatePromptAgentInput(t *testing.T) {
	if err := validatePromptAgentInput("   ", 4000); err == nil {
		t.Fatal("空消息应报错")
	}
	if err := validatePromptAgentInput("你好", 4000); err != nil {
		t.Fatalf("正常消息不应报错: %v", err)
	}
	long := strings.Repeat("字", 11)
	if err := validatePromptAgentInput(long, 10); err == nil {
		t.Fatal("超长消息应报错")
	} else if !strings.Contains(err.Error(), "10") {
		t.Fatalf("超长提示应包含上限值: %v", err)
	}
}

func TestIntValueClamps(t *testing.T) {
	cases := []struct {
		in       interface{}
		fallback int
		min      int
		max      int
		want     int
	}{
		{float64(1200), 100, 100, 2000, 1200},
		{float64(5), 100, 100, 2000, 100},
		{float64(99999), 100, 100, 2000, 2000},
		{"oops", 300, 100, 2000, 300},
		{nil, 40, 2, 80, 40},
	}
	for _, c := range cases {
		if got := intValue(c.in, c.fallback, c.min, c.max); got != c.want {
			t.Fatalf("intValue(%v) = %d，期望 %d", c.in, got, c.want)
		}
	}
}

func TestFloatValueOrDefault(t *testing.T) {
	if got := floatValueOrDefault(float64(0.6), 0.7); got != 0.6 {
		t.Fatalf("float64 解析失败: %v", got)
	}
	if got := floatValueOrDefault("0.9", 0.7); got != 0.7 {
		t.Fatalf("非数字应回落默认值: %v", got)
	}
	if got := floatValueOrDefault(nil, 0.5); got != 0.5 {
		t.Fatalf("空值应回落默认值: %v", got)
	}
}
